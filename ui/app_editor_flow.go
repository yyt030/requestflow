package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

// editorBranchChildren 编辑分支/多分支/并行任务的子步骤（复用简化的子步骤表单）。
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
