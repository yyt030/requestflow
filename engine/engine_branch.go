package engine

import (
	"fmt"
	"strings"

	"requestflow/model"
)

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
