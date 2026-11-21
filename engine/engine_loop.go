package engine

import (
	"fmt"
	"strings"
	"time"

	"requestflow/model"
)

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

		// 限速：每轮执行前等待，降低被反爬概率。
		if lp.DelayMs > 0 {
			time.Sleep(time.Duration(lp.DelayMs) * time.Millisecond)
		}

		// 执行本轮子步骤；空页时按 RetryOnEmpty 重试。
		maxRetry := lp.RetryOnEmpty
		if maxRetry < 0 {
			maxRetry = 0
		}
		for attempt := 0; attempt <= maxRetry; attempt++ {
			var childErr error
			for _, child := range step.Children {
				if !child.Enabled {
					continue
				}
				if err := e.runStep(child, depth+1); err != nil {
					childErr = err
					break
				}
			}
			if childErr != nil {
				return childErr
			}
			empty := false
			if lp.MinRowsVar != "" {
				n := 0
				if list, ok := toSlice(e.ctx.Get(lp.MinRowsVar)); ok {
					n = len(list)
				}
				threshold := lp.MinRows
				if threshold <= 0 {
					threshold = 1
				}
				empty = n < threshold
			}
			if !empty {
				break
			}
			if attempt < maxRetry {
				e.emit("log", step.ID, step.Name, fmt.Sprintf("第 %d 轮变量 %s 命中不足，重试 %d/%d", i+1, lp.MinRowsVar, attempt+1, maxRetry))
				if lp.DelayMs > 0 {
					time.Sleep(time.Duration(lp.DelayMs) * time.Millisecond)
				}
			} else {
				e.emit("log", step.ID, step.Name, fmt.Sprintf("警告：第 %d 轮变量 %s 命中不足（疑似反爬页或抓空页）", i+1, lp.MinRowsVar))
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

	// 循环结束后的累积去重（P1）：对指定数组变量按字段去重，并提示去掉的重复数。
	if strings.TrimSpace(lp.DedupVar) != "" {
		name := strings.TrimSpace(lp.DedupVar)
		list, ok := toSlice(e.ctx.Get(name))
		if !ok {
			e.emit("log", step.ID, step.Name, fmt.Sprintf("去重跳过：变量 %s 不是数组", name))
		} else {
			deduped, removed := dedupByField(list, strings.TrimSpace(lp.DedupField))
			e.ctx.Set(name, deduped)
			field := strings.TrimSpace(lp.DedupField)
			if field == "" {
				field = "整行"
			}
			e.emit("log", step.ID, step.Name, fmt.Sprintf("累积去重[%s=%s]：%d 行 → %d 行，去掉 %d 条重复",
				name, field, len(list), len(deduped), removed))
			if stat := strings.TrimSpace(lp.DedupStat); stat != "" {
				e.ctx.Set(stat, removed)
			}
		}
	}
	return nil
}

// evalBreak 求值 until 的终止条件，支持 "{{len(rows)}} == 0" 与内置函数。

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
