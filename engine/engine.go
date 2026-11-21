package engine

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"requestflow/model"
)

// Event 执行过程中产生的事件，UI 订阅它实时显示日志与步骤状态。
type Event struct {
	Type     string // step-start | step-done | step-error | log | loop-iter | done
	StepID   string
	StepName string
	Message  string
	Time     time.Time
}

// Result 单次执行的汇总结果。

// Result 单次执行的汇总结果。
type Result struct {
	Scenario string
	OK       bool
	Err      error
	Vars     map[string]interface{}
	Started  time.Time
	Ended    time.Time
	// 报告用结构化记录（TraceLog 为空时为空切片）
	Steps      []StepTrace    `json:"steps,omitempty"`
	Assertions []AssertDetail `json:"assertions,omitempty"`
}

// Engine 场景执行引擎。

// Engine 场景执行引擎。
type Engine struct {
	scenario *model.Scenario
	ctx      *Context
	http     *HTTPClient
	onEvent  func(Event)
	lastResp *model.Response
	stopFlag int32 // 原子访问，支持并行子任务安全读取

	// emitMu 串行化事件回调。并行汇合的多个子任务共享同一个 onEvent，
	// 加锁保证 UI/日志订阅方不会收到并发调用。
	emitMu *sync.Mutex
	// localCtx 标记该 Engine 是否使用外部注入的上下文（并行子任务为 true），
	// 用于避免子任务污染父场景的变量命名空间。
	localCtx bool

	// trace 收集步骤/断言记录；并行子任务共享同一个 trace。
	trace *TraceLog

	// tcpPool 长连接池：按连接名共享 TCP socket，实现跨步骤长连接。
	// 父引擎与并行子引擎共享同一个池；由根 Run 结束时统一关闭。
	tcpPool *tcpConnPool
}

// tcpConnPool 线程安全的长连接池。

func NewEngine(sc *model.Scenario, onEvent func(Event)) *Engine {
	timeout := 30 * time.Second
	// nil 场景防御：界面在打开/新建的过渡态可能传入 nil，避免解引用 panic。
	if sc == nil {
		sc = &model.Scenario{Name: "空场景"}
	}
	// 变量合并：场景全局变量打底，当前激活环境的同名变量覆盖（环境变量分层）。
	// 不修改场景原始数据，只构造本次运行用的初始变量集。
	merged := mergeEnvVariables(sc)
	return &Engine{
		scenario: sc,
		ctx:      NewContext(merged),
		http:     NewHTTPClient(timeout, true),
		onEvent:  onEvent,
		emitMu:   &sync.Mutex{},
		trace:    &TraceLog{},
		tcpPool:  newTCPConnPool(),
	}
}

// mergeEnvVariables 合并全局变量与当前激活环境的变量，环境变量优先。

// mergeEnvVariables 合并全局变量与当前激活环境的变量，环境变量优先。
func mergeEnvVariables(sc *model.Scenario) map[string]string {
	out := make(map[string]string, len(sc.Variables))
	for k, v := range sc.Variables {
		out[k] = v
	}
	if env := sc.EnvVars(sc.ActiveEnv); env != nil {
		for k, v := range env {
			out[k] = v
		}
	}
	return out
}

func (e *Engine) emit(typ, stepID, stepName, msg string) {
	if e.onEvent != nil {
		e.emitMu.Lock()
		e.onEvent(Event{Type: typ, StepID: stepID, StepName: stepName, Message: msg, Time: time.Now()})
		e.emitMu.Unlock()
	}
}

// Context 暴露运行时上下文，便于运行后检查变量。

// Context 暴露运行时上下文，便于运行后检查变量。
func (e *Engine) Context() *Context { return e.ctx }

// Trace 暴露结构化执行记录（供报告使用）。

// Trace 暴露结构化执行记录（供报告使用）。
func (e *Engine) Trace() *TraceLog { return e.trace }

// Stop 请求在当前步骤后停止。

// Stop 请求在当前步骤后停止。
func (e *Engine) Stop() { atomic.StoreInt32(&e.stopFlag, 1) }

// stopped 安全读取停止标记。

// stopped 安全读取停止标记。
func (e *Engine) stopped() bool { return atomic.LoadInt32(&e.stopFlag) == 1 }

// Run 按顺序执行整个场景。

// Run 按顺序执行整个场景。
func (e *Engine) Run() *Result {
	if e.tcpPool == nil {
		e.tcpPool = newTCPConnPool()
	}
	// 场景结束（含失败/提前停止）统一关闭所有长连接，避免 socket 泄漏。
	defer e.tcpPool.closeAll()
	res := &Result{Scenario: e.scenario.Name, Started: time.Now()}
	for _, step := range e.scenario.Steps {
		if e.stopped() {
			e.emit("log", "", "", "已请求停止")
			break
		}
		if !step.Enabled {
			continue
		}
		if err := e.runStep(step, 0); err != nil {
			res.Err = err
			res.OK = false
			e.emit("done", "", "", fmt.Sprintf("执行失败: %v", err))
			break
		}
	}
	res.Ended = time.Now()
	res.Vars = e.ctx.Snapshot()
	if e.trace != nil {
		res.Steps, res.Assertions = e.trace.Snapshot()
	}
	if res.Err == nil {
		res.OK = true
		e.emit("done", "", "", "执行完成")
	}
	return res
}

// runStep 执行单步，depth 为循环嵌套层级。

// runStep 执行单步，depth 为循环嵌套层级。
func (e *Engine) runStep(step model.Step, depth int) (retErr error) {
	started := time.Now()
	defer func() {
		if e.trace != nil {
			trace := StepTrace{
				Name: step.Name, Type: step.Type,
				Status: "pass", Started: started, Ended: time.Now(),
			}
			trace.DurationMS = trace.Ended.Sub(started).Milliseconds()
			if retErr != nil {
				trace.Status = "fail"
				trace.Err = retErr.Error()
			}
			e.trace.addStep(trace)
		}
	}()
	e.emit("step-start", step.ID, step.Name, "开始")
	var err error
	switch step.Type {
	case model.StepRequest:
		err = e.runRequest(step)
	case model.StepTCP:
		err = e.runTCPStep(step)
	case model.StepLoop:
		err = e.runLoop(step, depth)
	case model.StepBranch:
		err = e.runBranch(step, depth)
	case model.StepSwitch:
		err = e.runSwitch(step, depth)
	case model.StepParallel:
		err = e.runParallel(step, depth)
	case model.StepMerge:
		err = e.runMerge(step)
	case model.StepProcess:
		err = runProcess(e.ctx, step)
		e.emit("log", step.ID, step.Name, fmt.Sprintf("内置处理: %s", step.Op))
	case model.StepScript:
		if strings.EqualFold(step.ScriptLang, "python") || strings.EqualFold(step.ScriptLang, "py") {
			err = runPythonScript(e.ctx, step, e.lastResp)
		} else {
			err = runScript(e.ctx, step, e.lastResp)
		}
		e.emit("log", step.ID, step.Name, "自定义脚本已执行")
	case model.StepExport:
		var path string
		path, err = runExport(e.ctx, e.ctx.Render(step.From), e.ctx.Render(step.Format), e.ctx.Render(step.Path))
		if err == nil {
			e.emit("log", step.ID, step.Name, "已导出: "+path)
		}
	case model.StepAssert:
		details, detErr := applyAssertsDetailed(e.ctx, e.lastResp, step.Assert)
		e.trace.addAssertions(step.Name, step.ID, details)
		err = detErr
	default:
		err = fmt.Errorf("未知步骤类型: %s", step.Type)
	}

	if err != nil {
		e.emit("step-error", step.ID, step.Name, err.Error())
		if !step.ContinueOnFail {
			return err
		}
		e.emit("log", step.ID, step.Name, "忽略错误并继续")
		return nil
	}
	e.emit("step-done", step.ID, step.Name, "完成")
	return nil
}

// runStepScript 按脚本语言执行一段请求级脚本（Pre-request / Tests）。
// 语言取自 step.ScriptLang，空值按 js 处理，与独立 script 步骤保持一致。
