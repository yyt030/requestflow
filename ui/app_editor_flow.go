package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

// editorBranch 条件分支编辑器：可视化拼装条件 + then/else 子步骤。
func (a *App) editorBranch(c *ui.Context, t *ui.Theme, step *model.Step) {
	if step.Branch == nil {
		step.Branch = &model.Branch{Logic: "and", Conditions: []model.Condition{{Op: "大于"}}}
	}
	br := step.Branch

	fieldHint(c, t, "根据变量条件决定走 then 还是 else：例如 amount 大于 0 请求页面1，否则请求页面2。")
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "条件").FontSize(12).Bold()
		ui.Spacer(c)
		logics := []string{"and", "or"}
		if br.Logic == "" {
			br.Logic = "and"
		}
		ui.Select(c, &br.Logic, logics).Width(80)
		ui.Text(c, map[string]string{"and": "全部满足", "or": "任一满足"}[br.Logic]).FontSize(10).TextColor(t.TextMuted)
	})

	// 条件列表：每行 = 变量名 + 操作符 + 比较值
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
				// 存在/为空/为真 无需比较值
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

	ui.Divider(c)
	ui.Text(c, "高级：直接写条件表达式（留空则用上方条件）").FontSize(10).TextColor(t.TextMuted)
	ui.TextInput(c, &br.When).Placeholder("{{amount}} > 0  或  {{len(rows)}} == 0")

	ui.Divider(c)
	ui.Text(c, "条件成立 → then").FontSize(12).Bold().TextColor(t.Accent)
	a.editorBranchChildren(c, t, step, &br.Then, "then")
	ui.Text(c, "条件不成立 → else").FontSize(12).Bold().TextColor(t.TextMuted)
	a.editorBranchChildren(c, t, step, &br.Else, "else")
}

// editorBranchChildren 编辑分支的 then/else 子步骤（复用简化的子步骤表单）。

// editorBranchChildren 编辑分支的 then/else 子步骤（复用简化的子步骤表单）。
func (a *App) editorBranchChildren(c *ui.Context, t *ui.Theme, step *model.Step, children *[]model.Step, slot string) {
	ui.Row(c).Gap(8).Children(func() {
		if ui.Button(c.Key("http"), "+ 请求").Clicked() {
			*children = append(*children, model.Step{ID: newID(), Name: "请求", Type: model.StepRequest, Enabled: true, Method: "GET"})
		}
		if ui.Button(c.Key("tcp"), "+ TCP").Clicked() {
			*children = append(*children, model.Step{ID: newID(), Name: "TCP", Type: model.StepTCP, Enabled: true, TCPPort: "8080", TCPSendMode: "line", TCPReadMode: "line", TCPTimeout: "5000"})
		}
		if ui.Button(c.Key("script"), "+ 脚本").Clicked() {
			*children = append(*children, model.Step{ID: newID(), Name: "脚本", Type: model.StepScript, Enabled: true})
		}
		if ui.Button(c.Key("process"), "+ 处理").Clicked() {
			*children = append(*children, model.Step{ID: newID(), Name: "处理", Type: model.StepProcess, Enabled: true, Op: "set"})
		}
	})
	if len(*children) == 0 {
		ui.Text(c, "（无子步骤）").FontSize(10).TextColor(t.TextMuted)
	}
	for i := range *children {
		child := &(*children)[i]
		idx := i
		ui.Column(c.Key(fmt.Sprintf("%s-child-%d", slot, idx))).Padding(8).Gap(5).Background(t.Surface).Radius(8).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, a.stepLabel(*child)).Grow(1)
				if ui.Button(c.Key("del"), "移除").Clicked() {
					*children = append((*children)[:idx], (*children)[idx+1:]...)
				}
			})
			ui.TextInput(c, &child.Name).Label("名称")
			switch child.Type {
			case model.StepRequest:
				ui.Row(c).Gap(6).Children(func() {
					methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE"}
					ui.Select(c, &child.Method, methods).Width(90)
					ui.TextInput(c, &child.URL).Placeholder("{{baseUrl}}/path").Grow(1)
				})
			case model.StepTCP:
				ui.Row(c).Gap(6).Children(func() {
					ui.TextInput(c, &child.TCPHost).Placeholder("主机").Grow(2)
					ui.TextInput(c, &child.TCPPort).Placeholder("端口").Grow(1)
				})
				ui.TextInput(c, &child.TCPSend).Placeholder("发送数据（支持 {{变量}}）")
				ui.TextInput(c, &child.Body).Placeholder("Body（可选）")
				a.editorExtract(c, t, child)
			case model.StepScript:
				ui.TextArea(c, &child.Code).Placeholder("vars.result = 'ok';").Height(60)
			case model.StepProcess:
				ops := processOps()
				op := child.Op
				if op == "" {
					op = "set"
				}
				if ui.Select(c, &op, ops).Label("处理").Changed() {
					child.Op = op
				}
				a.renderProcessForm(c, t, child, a.processFormRawOpen(child), collectVarNames(a.scenario))
			}
		})
	}
}

// editorSwitch 多分支编辑器：多个 case 自上而下匹配，命中即执行；支持 default。

// editorSwitch 多分支编辑器：多个 case 自上而下匹配，命中即执行；支持 default。
func (a *App) editorSwitch(c *ui.Context, t *ui.Theme, step *model.Step) {
	if step.Switch == nil {
		step.Switch = &model.Switch{}
	}
	sw := step.Switch
	fieldHint(c, t, "多分支：自上而下匹配，命中第一个 case 后执行并跳出；都不满足则执行 default。")

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
	ui.Divider(c)
	ui.Text(c, "default（都不命中时执行）").FontSize(11).Bold().TextColor(t.TextMuted)
	a.editorBranchChildren(c, t, step, &sw.Default, "swdefault")
}

// editorParallel 扇出编辑器：配置多个并发子任务及其产物汇合方式。
