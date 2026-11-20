package engine

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"requestflow/model"
)

// Event 执行过程中产生的事件，UI 订阅它实时显示日志与步骤状态。
type Event struct {
	Type    string // step-start | step-done | step-error | log | loop-iter | done
	StepID  string
	StepName string
	Message string
	Time    time.Time
}

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
type tcpConnPool struct {
	mu    sync.Mutex
	conns map[string]*tcpConnEntry
}

type tcpConnEntry struct {
	conn    net.Conn
	timeout time.Duration
}

func newTCPConnPool() *tcpConnPool {
	return &tcpConnPool{conns: map[string]*tcpConnEntry{}}
}

// get 取连接；不存在返回 nil。
func (p *tcpConnPool) get(key string) *tcpConnEntry {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.conns[key]
}

// put 存入连接；若同名连接已存在则先关闭旧连接。
func (p *tcpConnPool) put(key string, entry *tcpConnEntry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if old := p.conns[key]; old != nil && old.conn != entry.conn {
		_ = old.conn.Close()
	}
	p.conns[key] = entry
}

// remove 关闭并移除连接。
func (p *tcpConnPool) remove(key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if entry := p.conns[key]; entry != nil {
		_ = entry.conn.Close()
		delete(p.conns, key)
		return true
	}
	return false
}

// closeAll 关闭所有长连接。
func (p *tcpConnPool) closeAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for key, entry := range p.conns {
		_ = entry.conn.Close()
		delete(p.conns, key)
	}
}

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
func (e *Engine) Context() *Context { return e.ctx }

// Trace 暴露结构化执行记录（供报告使用）。
func (e *Engine) Trace() *TraceLog { return e.trace }

// Stop 请求在当前步骤后停止。
func (e *Engine) Stop() { atomic.StoreInt32(&e.stopFlag, 1) }

// stopped 安全读取停止标记。
func (e *Engine) stopped() bool { return atomic.LoadInt32(&e.stopFlag) == 1 }

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
func (e *Engine) runStepScript(code string, step model.Step, last *model.Response, phase string) error {
	if strings.TrimSpace(code) == "" {
		return nil
	}
	wrap := step
	wrap.Code = code
	var err error
	if strings.EqualFold(step.ScriptLang, "python") || strings.EqualFold(step.ScriptLang, "py") {
		err = runPythonScript(e.ctx, wrap, last)
	} else {
		err = runScript(e.ctx, wrap, last)
	}
	if err != nil {
		return fmt.Errorf("%s 脚本失败: %w", phase, err)
	}
	return nil
}

func (e *Engine) runRequest(step model.Step) error {
	// Pre-request Script：在渲染请求参数之前执行，脚本对变量的改写能影响本次请求。
	if err := e.runStepScript(step.PreScript, step, e.lastResp, "Pre-request"); err != nil {
		return err
	}
	method := e.ctx.Render(step.Method)
	rawURL := e.ctx.Render(step.URL)
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		base := e.ctx.Render(e.scenario.BaseURL)
		if base != "" {
			rawURL = strings.TrimRight(base, "/") + "/" + strings.TrimLeft(rawURL, "/")
		}
	}
	headers := e.ctx.RenderMap(step.Headers)
	query := e.ctx.RenderMap(step.Query)
	body := e.ctx.Render(step.Body)
	// 构造渲染后的完整请求 step：携带 Body 类型 / 表单 / GraphQL / 认证全部字段。
	reqStep := model.Step{
		BodyType:         e.ctx.Render(step.BodyType),
		Body:             body,
		GraphQLQuery:     e.ctx.Render(step.GraphQLQuery),
		GraphQLVariables: e.ctx.Render(step.GraphQLVariables),
		AuthType:         e.ctx.Render(step.AuthType),
		AuthToken:        e.ctx.Render(step.AuthToken),
		AuthUser:         e.ctx.Render(step.AuthUser),
		AuthPass:         e.ctx.Render(step.AuthPass),
		AuthKeyName:      e.ctx.Render(step.AuthKeyName),
		AuthKeyValue:     e.ctx.Render(step.AuthKeyValue),
		AuthKeyIn:        e.ctx.Render(step.AuthKeyIn),
		AuthTokenURL:     e.ctx.Render(step.AuthTokenURL),
		AuthClientID:     e.ctx.Render(step.AuthClientID),
		AuthClientSecret: e.ctx.Render(step.AuthClientSecret),
		AuthScope:        e.ctx.Render(step.AuthScope),
		AuthGrantType:    e.ctx.Render(step.AuthGrantType),
	}
	for _, f := range step.FormData {
		reqStep.FormData = append(reqStep.FormData, model.FormField{
			Key:   e.ctx.Render(f.Key),
			Type:  f.Type,
			Value: e.ctx.Render(f.Value),
		})
	}

	e.emit("log", step.ID, step.Name, fmt.Sprintf("%s %s", method, rawURL))
	resp, err := e.http.DoRequest(&reqStep, method, rawURL, headers, query)
	if err != nil {
		return err
	}
	e.lastResp = resp
	e.emit("log", step.ID, step.Name, fmt.Sprintf("响应 %d, %d 字节", resp.StatusCode, len(resp.Body)))

	if err := applyExtracts(e.ctx, resp, step.Extract); err != nil {
		return err
	}
	details, detErr := applyAssertsDetailed(e.ctx, resp, step.Assert)
	e.trace.addAssertions(step.Name, step.ID, details)
	if detErr != nil {
		return detErr
	}
	e.ctx.SetStepResponse(step.Name, responseView(resp))
	// Tests（PostScript）：在响应提取与断言之后执行，last 指向本次响应。
	if err := e.runStepScript(step.PostScript, step, resp, "Tests"); err != nil {
		return err
	}
	return nil
}

// runBranch 条件分支：when 成立执行 then 子步骤，否则执行 else 子步骤。
// 支持结构化 Conditions（按 Logic 组合）与原始 When 表达式（优先）。
func (e *Engine) runBranch(step model.Step, depth int) error {
	br := step.Branch
	if br == nil {
		return fmt.Errorf("branch 步骤缺少 branch 配置")
	}
	hit := e.evalBranch(br)
	if hit {
		e.emit("log", step.ID, step.Name, "条件成立 → 执行 then 分支")
	} else {
		e.emit("log", step.ID, step.Name, "条件不成立 → 执行 else 分支")
	}
	children := br.Else
	if hit {
		children = br.Then
	}
	for _, child := range children {
		if !child.Enabled {
			continue
		}
		if err := e.runStep(child, depth+1); err != nil {
			return err
		}
	}
	return nil
}

// evalBranch 求值分支条件：When 表达式优先，否则按 Logic 组合 Conditions。
func (e *Engine) evalBranch(br *model.Branch) bool {
	if strings.TrimSpace(br.When) != "" {
		return e.evalBreak(br.When) // 复用表达式求值（支持比较/函数/变量）
	}
	if len(br.Conditions) == 0 {
		return false
	}
	logic := strings.ToLower(br.Logic)
	if logic != "or" {
		logic = "and"
	}
	for _, cond := range br.Conditions {
		ok := e.evalCondition(cond)
		if logic == "and" && !ok {
			return false
		}
		if logic == "or" && ok {
			return true
		}
	}
	return logic == "and"
}

// evalCondition 求值单个结构化条件。
func (e *Engine) evalCondition(cond model.Condition) bool {
	val := e.ctx.Get(cond.Var)
	op := strings.ToLower(strings.TrimSpace(cond.Op))
	// 中文操作符别名（UI 下拉友好），统一归一化为英文。
	aliases := map[string]string{
		"大于": "gt", "大于等于": "gte", "不小于": "gte",
		"小于": "lt", "小于等于": "lte", "不大于": "lte",
		"等于": "eq", "不等于": "ne", "包含": "contains", "不包含": "notcontains",
		"存在": "exists", "非空": "exists", "为空": "empty", "为真": "true",
	}
	if a, ok := aliases[op]; ok {
		op = a
	}
	switch op {
	case "exists":
		return valueLen(val) > 0
	case "empty", "isempty":
		return valueLen(val) == 0
	case "true", "istrue":
		if b, ok := val.(bool); ok {
			return b
		}
		f, _ := toFloat(val)
		return f != 0 || strings.EqualFold(toStr(val), "true")
	}

	// 数值比较（两边都能转数字时），否则字符串比较
	lf, lok := toFloat(val)
	rf, rok := toFloat(cond.Value)
	ls, rs := toStr(val), cond.Value
	switch op {
	case "eq", "==":
		if lok && rok {
			return lf == rf
		}
		return ls == rs
	case "ne", "!=":
		if lok && rok {
			return lf != rf
		}
		return ls != rs
	case "gt", ">":
		if lok && rok {
			return lf > rf
		}
		return ls > rs
	case "gte", ">=":
		if lok && rok {
			return lf >= rf
		}
		return ls >= rs
	case "lt", "<":
		if lok && rok {
			return lf < rf
		}
		return ls < rs
	case "lte", "<=":
		if lok && rok {
			return lf <= rf
		}
		return ls <= rs
	case "contains":
		return strings.Contains(ls, rs)
	case "notcontains":
		return !strings.Contains(ls, rs)
	default:
		// 未知操作符：退化为相等比较
		return ls == rs
	}
}

// runLoop 实现循环翻页。三种模式：count / foreach / until。
func (e *Engine) runLoop(step model.Step, depth int) error {
	if step.Loop == nil {
		return fmt.Errorf("loop 步骤缺少 loop 配置")
	}
	lp := step.Loop
	max := lp.Max
	if max <= 0 {
		max = 100
	}
	mode := strings.ToLower(lp.Mode)
	if mode == "" {
		mode = "count"
	}

	// foreach 预先取数组长度
	var items []interface{}
	if mode == "foreach" {
		var ok bool
		items, ok = toSlice(e.ctx.Get(lp.ListVar))
		if !ok {
			return fmt.Errorf("foreach: 变量 %s 不是数组", lp.ListVar)
		}
		if max > len(items) {
			max = len(items)
		}
	}

	page := lp.PageStart
	for i := 0; i < max; i++ {
		if e.stopped() {
			break
		}
		// 设置循环变量
		if lp.PageVar != "" {
			e.ctx.Set(lp.PageVar, page)
		}
		switch mode {
		case "foreach":
			item := items[i]
			if lp.ItemVar != "" {
				e.ctx.Set(lp.ItemVar, item)
			}
			if lp.IndexVar != "" {
				e.ctx.Set(lp.IndexVar, i)
			}
		}

		e.emit("loop-iter", step.ID, step.Name, fmt.Sprintf("第 %d 轮 (page=%d)", i+1, page))

		for _, child := range step.Children {
			if !child.Enabled {
				continue
			}
			if err := e.runStep(child, depth+1); err != nil {
				return err
			}
		}

		// until 模式：条件成立即退出（如 len(rows) == 0 表示翻到空页）
		if mode == "until" && lp.BreakWhen != "" {
			if e.evalBreak(lp.BreakWhen) {
				e.emit("log", step.ID, step.Name, "满足终止条件，退出循环")
				break
			}
		}
		// count 模式：到达指定次数退出
		if mode == "count" && lp.Count > 0 && i+1 >= lp.Count {
			break
		}
		page += lp.PageStep
		if lp.PageStep == 0 {
			page = lp.PageStart + i + 1
		}
	}
	return nil
}

// evalBreak 求值 until 的终止条件，支持 "{{len(rows)}} == 0" 与内置函数。
func (e *Engine) evalBreak(expr string) bool {
	rendered := e.ctx.Render(expr)
	// 已完全渲染为布尔字面量
	switch strings.TrimSpace(rendered) {
	case "true":
		return true
	case "false":
		return false
	}
	// 直接交给表达式求值器
	v := e.ctx.Eval(expr)
	b, _ := toFloat(v)
	if b != 0 {
		return true
	}
	if s, ok := v.(bool); ok {
		return s
	}
	return false
}
