package engine

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"requestflow/model"
)

// runSwitch 多分支：自上而下依次求值每个 case，命中第一个即执行其 Steps 并跳出；
// 全部未命中则执行 Default。case 的求值复用 branch 的条件逻辑。
func (e *Engine) runSwitch(step model.Step, depth int) error {
	sw := step.Switch
	if sw == nil {
		return fmt.Errorf("switch 步骤缺少 switch 配置")
	}
	for i := range sw.Cases {
		c := sw.Cases[i]
		br := &model.Branch{
			Logic:      c.Logic,
			Conditions: c.Conditions,
			When:       c.When,
		}
		if !e.evalBranch(br) {
			continue
		}
		label := c.Name
		if label == "" {
			label = fmt.Sprintf("case %d", i+1)
		}
		e.emit("log", step.ID, step.Name, "命中 "+label)
		for _, child := range c.Steps {
			if !child.Enabled {
				continue
			}
			if err := e.runStep(child, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	e.emit("log", step.ID, step.Name, "未命中任何 case → 执行 default")
	for _, child := range sw.Default {
		if !child.Enabled {
			continue
		}
		if err := e.runStep(child, depth+1); err != nil {
			return err
		}
	}
	return nil
}

// taskProduct 并行子任务的产物与执行结果。
type taskProduct struct {
	name     string
	value    interface{}
	err      error
	index    int
	duration int64 // 耗时（毫秒）
}

// taskStatusVar 默认的任务状态登记表变量名。
const taskStatusVar = "task"

// registerTaskStatus 把各并行任务的执行状态写入上下文，供 merge 门控表达式引用。
// 结构：task.<任务名>.status = success|fail；.output；.error；.duration(ms)。
// 同时写入 task.success/failed/total 汇总计数与 task.allSuccess 布尔。
func (e *Engine) registerTaskStatus(p *model.Parallel, rs []taskProduct) {
	name := p.StatusVar
	if name == "" {
		name = taskStatusVar
	}
	table := make(map[string]interface{}, len(rs)+4)
	success, failed := 0, 0
	for _, r := range rs {
		status, errText := "success", ""
		if r.err != nil {
			status, errText = "fail", r.err.Error()
			failed++
		} else {
			success++
		}
		table[r.name] = map[string]interface{}{
			"status":   status,
			"output":   r.value,
			"error":    errText,
			"duration": r.duration,
		}
	}
	table["success"] = success
	table["failed"] = failed
	table["total"] = len(rs)
	table["allSuccess"] = failed == 0
	e.ctx.Set(name, table)
}

// runParallel 并行结果汇合：并发执行每个子任务（各自独立的变量沙箱，
// 共享 HTTP 客户端与事件回调），全部完成后按 Mode 把产物汇合到 Var 变量。
func (e *Engine) runParallel(step model.Step, depth int) error {
	p := step.Parallel
	if p == nil {
		return fmt.Errorf("parallel 步骤缺少 parallel 配置")
	}
	if len(p.Tasks) == 0 {
		return fmt.Errorf("parallel 步骤没有任何子任务")
	}

	results := make([]taskProduct, len(p.Tasks))
	var wg sync.WaitGroup
	for i := range p.Tasks {
		task := p.Tasks[i]
		idx := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			started := time.Now()
			name := task.Name
			if name == "" {
				name = fmt.Sprintf("task %d", idx+1)
			}
			// 子任务使用父上下文的快照沙箱，写入不回传父级。
			sub := &Engine{
				scenario: e.scenario,
				ctx:      e.ctx.Fork(),
				http:     e.http, // 线程安全（http.Client + CookieJar）
				onEvent:  e.onEvent,
				emitMu:   e.emitMu, // 共享事件锁，保证回调串行
				trace:    e.trace,  // 共享执行记录
			}
			for _, child := range task.Steps {
				if e.stopped() {
					break
				}
				if !child.Enabled {
					continue
				}
				if err := sub.runStep(child, depth+1); err != nil {
					results[idx] = taskProduct{name: name, err: err, index: idx,
						duration: time.Since(started).Milliseconds()}
					return
				}
			}
			// 计算产物：Value 常量 > Var 变量 > 整个沙箱快照
			var product interface{}
			switch {
			case strings.TrimSpace(task.Value) != "":
				product = sub.ctx.Render(task.Value)
			case strings.TrimSpace(task.Var) != "":
				product = sub.ctx.Get(strings.TrimSpace(task.Var))
			default:
				product = sub.ctx.Snapshot()
			}
			results[idx] = taskProduct{name: name, value: product, index: idx,
				duration: time.Since(started).Milliseconds()}
		}()
	}
	wg.Wait()

	// 登记任务状态（无论成败），供后续 merge 步骤做门控判断。
	e.registerTaskStatus(p, results)

	// 收集错误
	var firstErr error
	for _, r := range results {
		if r.err != nil {
			e.emit("log", step.ID, step.Name, fmt.Sprintf("子任务 %s 失败: %v", r.name, r.err))
			if firstErr == nil {
				firstErr = r.err
			}
		}
	}
	if firstErr != nil && p.FailFast {
		return firstErr
	}

	// 汇合产物（只纳入成功任务）
	merged := mergeProducts(p.Mode, p.Separator, results)
	target := e.ctx.Render(p.Var)
	if target == "" {
		target = "merged"
	}
	e.ctx.Set(target, merged)
	e.emit("log", step.ID, step.Name, fmt.Sprintf("汇合 %d 个任务 → 变量 %s (mode=%s)",
		countOK(results), target, defaultMode(p.Mode)))
	return nil
}

func countOK(rs []taskProduct) int {
	n := 0
	for _, r := range rs {
		if r.err == nil {
			n++
		}
	}
	return n
}

func defaultMode(mode string) string {
	m := strings.ToLower(strings.TrimSpace(mode))
	if m == "" {
		return "array"
	}
	return m
}

// mergeProducts 按模式把各任务产物汇合成一个值。
func mergeProducts(mode, sep string, rs []taskProduct) interface{} {
	switch defaultMode(mode) {
	case "object":
		out := make(map[string]interface{})
		for _, r := range rs {
			if r.err != nil {
				continue
			}
			out[r.name] = r.value
		}
		return out
	case "table":
		// 每个产物视为行（数组则展开，标量则包装），合并成一张表。
		out := make([]interface{}, 0)
		for _, r := range rs {
			if r.err != nil {
				continue
			}
			rows, ok := toSlice(r.value)
			if ok {
				out = append(out, rows...)
			}
		}
		return out
	case "concat":
		parts := make([]string, 0, len(rs))
		for _, r := range rs {
			if r.err != nil {
				continue
			}
			parts = append(parts, flattenToStrings(r.value)...)
		}
		return strings.Join(parts, sep)
	case "array":
		fallthrough
	default:
		out := make([]interface{}, 0, len(rs))
		for _, r := range rs {
			if r.err != nil {
				continue
			}
			out = append(out, r.value)
		}
		return out
	}
}

// flattenToStrings 把标量转为字符串，数组则逐元素转换，便于 concat 汇合文本列表。
func flattenToStrings(v interface{}) []string {
	if list, ok := toSlice(v); ok {
		// 标量 toSlice 会包装成单元素，需区分原始类型
		switch v.(type) {
		case []interface{}, []map[string]interface{}, []string:
			out := make([]string, 0, len(list))
			for _, item := range list {
				out = append(out, toStr(item))
			}
			return out
		}
	}
	return []string{toStr(v)}
}

// runMerge 扇入门控：等待上游 parallel 的所有任务完成后（parallel 本身已同步等待），
// 用高级表达式检查任务状态/产物。条件不成立即返回错误，中止整个场景。
// 表达式可引用 task.<任务名>.status/output/error/duration 及 task.success/failed/allSuccess 等汇总。
func (e *Engine) runMerge(step model.Step) error {
	m := step.Merge
	if m == nil {
		return fmt.Errorf("merge 步骤缺少 merge 配置")
	}
	when := strings.TrimSpace(m.When)
	if when == "" {
		// 未提供表达式时，默认要求全部任务成功。
		when = "task.allSuccess"
	}
	ok := truthy(e.ctx.Eval(when))
	if !ok {
		snapshot := e.ctx.Get(taskStatusVar)
		e.emit("log", step.ID, step.Name, fmt.Sprintf("扇入门控未通过 [%s]，任务状态: %v", when, snapshot))
		return fmt.Errorf("扇入门控失败：条件 %q 不成立，中止场景", when)
	}
	e.emit("log", step.ID, step.Name, fmt.Sprintf("扇入门控通过 [%s]，放行后续任务", when))
	return nil
}
