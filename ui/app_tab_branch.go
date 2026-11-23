package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

// branchTabView 按 Postman 风格标签渲染条件分支步骤的单个配置维度。
func (a *App) branchTabView(c *ui.Context, t *ui.Theme, step *model.Step, tab string) {
	if step.Branch == nil {
		step.Branch = &model.Branch{Logic: "and", Conditions: []model.Condition{{Op: "大于"}}}
	}
	br := step.Branch
	switch tab {
	case "条件":
		a.branchCondView(c, t, br)
	case "高级":
		ui.Text(c, "直接写条件表达式").FontSize(12).Bold()
		fieldHint(c, t, "留空则用「条件」标签里的结构化条件；非空时以本表达式为准。表达式为真走 then，为假走 else。")
		ui.TextInput(c, &br.When).Placeholder("{{amount}} > 0  或  {{len(rows)}} == 0")
	case "then":
		ui.Text(c, "条件成立 → then").FontSize(12).Bold().TextColor(t.Accent)
		a.editorBranchChildren(c, t, step, &br.Then, "then")
	case "else":
		ui.Text(c, "条件不成立 → else").FontSize(12).Bold().TextColor(t.TextMuted)
		a.editorBranchChildren(c, t, step, &br.Else, "else")
	}
}

func (a *App) branchCondView(c *ui.Context, t *ui.Theme, br *model.Branch) {
	fieldHint(c, t, "根据变量条件决定走 then 还是 else：例如 amount 大于 0 请求页面1，否则请求页面2。")
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "条件组合").FontSize(12).Bold()
		ui.Spacer(c)
		logics := []string{"and", "or"}
		if br.Logic == "" {
			br.Logic = "and"
		}
		ui.Select(c, &br.Logic, logics).Width(80)
		ui.Text(c, map[string]string{"and": "全部满足", "or": "任一满足"}[br.Logic]).FontSize(10).TextColor(t.TextMuted)
	})

	ops := []string{"大于", "大于等于", "小于", "小于等于", "等于", "不等于", "包含", "不包含", "存在", "为空", "为真"}
	for i := range br.Conditions {
		cond := &br.Conditions[i]
		idx := i
		ui.Column(c.Key(fmt.Sprintf("cond-%d", idx))).Padding(8).Gap(5).Background(t.Surface).Radius(8).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, fmt.Sprintf("条件 %d", idx+1)).FontSize(10).TextColor(t.TextMuted).Width(48)
				ui.Spacer(c)
				if ui.Button(c.Key("del"), "移除").Clicked() {
					br.Conditions = append(br.Conditions[:idx], br.Conditions[idx+1:]...)
				}
			})
			ui.Row(c).Gap(6).Children(func() {
				ui.TextInput(c, &cond.Var).Placeholder("变量名，如 amount").Grow(1)
				if cond.Op == "" {
					cond.Op = "大于"
				}
				ui.Select(c, &cond.Op, ops).Width(90)
				needValue := cond.Op != "存在" && cond.Op != "为空" && cond.Op != "为真"
				if needValue {
					ui.TextInput(c, &cond.Value).Placeholder("值，如 0").Width(90)
				}
			})
		})
	}
	ui.Row(c).Gap(8).Children(func() {
		if ui.Button(c, "+ 添加条件").Clicked() {
			br.Conditions = append(br.Conditions, model.Condition{Op: "等于"})
		}
	})
}

// switchTabView 按 Postman 风格标签渲染多分支步骤的单个配置维度。
func (a *App) switchTabView(c *ui.Context, t *ui.Theme, step *model.Step, tab string) {
	if step.Switch == nil {
		step.Switch = &model.Switch{}
	}
	sw := step.Switch
	switch tab {
	case "分支":
		a.switchCasesView(c, t, step, sw)
	case "默认":
		ui.Text(c, "default（都不命中时执行）").FontSize(12).Bold().TextColor(t.TextMuted)
		a.editorBranchChildren(c, t, step, &sw.Default, "swdefault")
	}
}

func (a *App) switchCasesView(c *ui.Context, t *ui.Theme, step *model.Step, sw *model.Switch) {
	fieldHint(c, t, "多分支：自上而下匹配，命中第一个 case 后执行并跳出；都不满足则执行「默认」标签里的 default。")
	ops := []string{"大于", "大于等于", "小于", "小于等于", "等于", "不等于", "包含", "不包含", "存在", "为空", "为真"}
	for i := range sw.Cases {
		cs := &sw.Cases[i]
		idx := i
		ui.Column(c.Key(fmt.Sprintf("switch-case-%d", idx))).Padding(10).Gap(6).Background(t.Surface).Radius(8).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, fmt.Sprintf("case %d", idx+1)).FontSize(11).Bold().TextColor(t.Accent)
				ui.TextInput(c, &cs.Name).Placeholder("分支名称（可选）").Grow(1)
				if ui.Button(c.Key("up"), "↑").Width(28).Disabled(idx == 0).Clicked() {
					sw.Cases[idx], sw.Cases[idx-1] = sw.Cases[idx-1], sw.Cases[idx]
				}
				if ui.Button(c.Key("dn"), "↓").Width(28).Disabled(idx == len(sw.Cases)-1).Clicked() {
					sw.Cases[idx], sw.Cases[idx+1] = sw.Cases[idx+1], sw.Cases[idx]
				}
				if ui.Button(c.Key("del"), "移除").Clicked() {
					sw.Cases = append(sw.Cases[:idx], sw.Cases[idx+1:]...)
				}
			})
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if cs.Logic == "" {
					cs.Logic = "and"
				}
				ui.Select(c, &cs.Logic, []string{"and", "or"}).Width(80)
				ui.Text(c, map[string]string{"and": "全部满足", "or": "任一满足"}[cs.Logic]).FontSize(10).TextColor(t.TextMuted)
			})
			for j := range cs.Conditions {
				cond := &cs.Conditions[j]
				ui.Row(c).Gap(6).Children(func() {
					ui.TextInput(c, &cond.Var).Placeholder("变量名").Grow(1)
					if cond.Op == "" {
						cond.Op = "等于"
					}
					ui.Select(c, &cond.Op, ops).Width(90)
					if cond.Op != "存在" && cond.Op != "为空" && cond.Op != "为真" {
						ui.TextInput(c, &cond.Value).Placeholder("值").Width(90)
					}
				})
			}
			if ui.Button(c.Key("addcond"), "+ 条件").Clicked() {
				cs.Conditions = append(cs.Conditions, model.Condition{Op: "等于"})
			}
			ui.Text(c, "高级表达式（留空则用上方条件）").FontSize(10).TextColor(t.TextMuted)
			ui.TextInput(c, &cs.When).Placeholder("{{status}} == 200")
			ui.Text(c, "命中后执行").FontSize(10).TextColor(t.TextMuted)
			a.editorBranchChildren(c, t, step, &cs.Steps, fmt.Sprintf("sw%d", idx))
		})
	}
	if ui.Button(c, "+ 添加 case").Clicked() {
		sw.Cases = append(sw.Cases, model.SwitchCase{Name: fmt.Sprintf("case %d", len(sw.Cases)+1), Logic: "and"})
	}
}
