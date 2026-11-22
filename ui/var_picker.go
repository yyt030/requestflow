package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

// 本文件实现「可用变量」的收集与一键插入，减少手敲 {{变量名}} 带来的拼写错误。
// 设计：变量按钮直接在目标输入框旁，点击后从弹层选择变量名，追加 {{name}} 到目标指针，
// 借助 mygo TextInput 的外部值同步机制（见 textInputBase）即时反映到界面。

// collectVarNames 聚合当前场景中所有可引用变量名：全局变量、环境变量、各步骤提取变量、
// 循环页码/遍历变量，以及内置变量。返回去重后有序列表。
func collectVarNames(sc *model.Scenario) []string {
	set := map[string]bool{}
	for k := range sc.Variables {
		set[k] = true
	}
	for _, vars := range sc.Environments {
		for k := range vars {
			set[k] = true
		}
	}
	// 内置：场景 Base URL 常用占位符。
	set["baseUrl"] = true
	for _, s := range sc.Steps {
		collectStepVars(s, set)
	}
	names := make([]string, 0, len(set))
	for k := range set {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// collectStepVars 递归收集单步骤（含所有子步骤分支）贡献的变量名。
func collectStepVars(s model.Step, set map[string]bool) {
	for _, ex := range s.Extract {
		if n := strings.TrimSpace(ex.Name); n != "" {
			set[n] = true
		}
	}
	// loop 的子步骤挂在 Step.Children 上，统一递归。
	for _, ch := range s.Children {
		collectStepVars(ch, set)
	}
	if s.Loop != nil {
		for _, n := range []string{s.Loop.PageVar, s.Loop.ListVar, s.Loop.ItemVar, s.Loop.IndexVar,
			s.Loop.DedupVar, s.Loop.DedupStat, s.Loop.MinRowsVar} {
			if n = strings.TrimSpace(n); n != "" {
				set[n] = true
			}
		}
	}
	if s.Branch != nil {
		for _, ch := range append(append([]model.Step{}, s.Branch.Then...), s.Branch.Else...) {
			collectStepVars(ch, set)
		}
	}
	if s.Switch != nil {
		for _, cs := range s.Switch.Cases {
			for _, ch := range cs.Steps {
				collectStepVars(ch, set)
			}
		}
		for _, ch := range s.Switch.Default {
			collectStepVars(ch, set)
		}
	}
	if s.Parallel != nil {
		for _, task := range s.Parallel.Tasks {
			for _, ch := range task.Steps {
				collectStepVars(ch, set)
			}
		}
	}
}

// varInsertButton 在输入框旁渲染一个「⊕变量」按钮，点击后列出可用变量，
// 选中即把 {{name}} 追加到 target 指向的字符串。names 为空时不渲染按钮。
func (a *App) varInsertButton(c *ui.Context, t *ui.Theme, target *string, names []string) {
	if len(names) == 0 {
		return
	}
	ui.PopoverMenuButton(c, "⊕变量", func(m *ui.Menu) {
		m.MaxWidth(260)
		for _, name := range names {
			n := name
			if m.Item("{{" + n + "}}").Chosen() {
				insertVarRef(target, n)
			}
		}
	}).MaxWidth(70).Tooltip("插入一个已定义的变量名，避免手敲 {{}}")
	_ = t
}

// insertVarRef 把 {{name}} 追加到目标字符串末尾；已有内容时自动补一个空格分隔，
// 方便在 URL 里连续插入多个变量片段。
func insertVarRef(target *string, name string) {
	ref := "{{" + name + "}}"
	if strings.TrimSpace(*target) == "" {
		*target = ref
		return
	}
	// 末尾已是 {{...}} 结束或空白，直接拼接；否则补空格，避免粘连。
	if strings.HasSuffix(*target, "}}") || strings.HasSuffix(*target, " ") {
		*target += ref
	} else {
		*target += " " + ref
	}
}

// varSummary 返回可用变量的一行摘要文本，用于编辑器顶部提示。
func varSummary(names []string) string {
	if len(names) == 0 {
		return "当前无可用变量（在全局变量或提取步骤中定义后即可一键插入）"
	}
	shown := names
	suffix := ""
	if len(shown) > 12 {
		shown = shown[:12]
		suffix = fmt.Sprintf(" …共 %d 个", len(names))
	}
	return "可用变量（点输入框旁 ⊕变量 一键插入）：" + strings.Join(shown, ", ") + suffix
}
