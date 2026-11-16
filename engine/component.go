package engine

import (
	"fmt"
	"strings"
	"sync"

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
	name  string
	value interface{}
	err   error
	index int
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
			}
			for _, child := range task.Steps {
				if e.stopped() {
					break
				}
				if !child.Enabled {
					continue
				}
				if err := sub.runStep(child, depth+1); err != nil {
					results[idx] = taskProduct{name: name, err: err, index: idx}
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
			results[idx] = taskProduct{name: name, value: product, index: idx}
		}()
	}
	wg.Wait()

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
