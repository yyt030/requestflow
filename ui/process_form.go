package ui

import (
	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

// 本文件按 process op 的字段元数据渲染结构化表单，替代裸 key=value 文本框。
// 每个文本字段旁挂「⊕变量」按钮；底部保留折叠的「原始参数」高级区，供进阶用户直接编辑。
// 注意：Go 的 map value 不可取地址，故采用「局部字符串编辑 + 渲染后写回 step.Args」模式。

// renderProcessForm 渲染一个 process 步骤的结构化参数表单。
// rawOpen 指向该步骤「原始参数」折叠区的展开状态（按步骤 ID 存于 App）。
func (a *App) renderProcessForm(c *ui.Context, t *ui.Theme, step *model.Step, rawOpen *bool, varNames []string) {
	if step.Args == nil {
		step.Args = map[string]string{}
	}
	def, ok := processOpDefs()[step.Op]
	if !ok {
		fieldHint(c, t, "未知处理操作，请在下方原始参数中编辑。")
		a.renderRawArgs(c, t, step)
		return
	}
	ui.Text(c, def.Desc).FontSize(11).TextColor(t.TextMuted)
	ui.Column(c).Gap(6).Children(func() {
		for _, f := range def.Fields {
			if f.Kind == procSelect {
				renderProcSelect(c, t, step, f)
			} else {
				a.renderProcText(c, t, step, f, varNames)
			}
		}
	})
	ui.Collapsible(c, "⚙ 原始参数（key=value，进阶）", rawOpen, func() {
		fieldHint(c, t, "每行 key=value，支持 {{变量}}。与上方结构化字段双向同步。")
		a.renderRawArgs(c, t, step)
	})
}

// renderProcSelect 渲染下拉字段并写回 step.Args。
func renderProcSelect(c *ui.Context, t *ui.Theme, step *model.Step, f procField) {
	cur := step.Args[f.Key]
	if cur == "" {
		cur = f.Options[0]
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if ui.Select(c, &cur, f.Options).Label(f.Label).Width(150).Changed() {
			step.Args[f.Key] = cur
		}
		ui.Text(c, f.Placeholder).FontSize(10).TextColor(t.TextMuted)
	})
}

// renderProcText 渲染文本字段（带变量按钮），编辑后写回 step.Args。
// PopoverMenuButton 的选择回调在构建阶段执行、可能改写 local，故渲染后统一回填。
func (a *App) renderProcText(c *ui.Context, t *ui.Theme, step *model.Step, f procField, varNames []string) {
	local := step.Args[f.Key]
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		if ui.TextInput(c, &local).Label(f.Label).Placeholder(f.Placeholder).Grow(1).Changed() {
			step.Args[f.Key] = local
		}
		a.varInsertButton(c, t, &local, varNames)
	})
	if local != step.Args[f.Key] {
		step.Args[f.Key] = local
	}
}

// renderRawArgs 渲染原始 key=value 文本框（高级区与未知 op 共用）。
func (a *App) renderRawArgs(c *ui.Context, t *ui.Theme, step *model.Step) {
	argsText := mapToString(step.Args)
	if ui.TextArea(c, &argsText).Height(100).Changed() {
		step.Args = stringToMap(argsText, "=")
	}
}

// processFormRawOpen 返回步骤对应的折叠区状态指针；App 用 map 按步骤 ID 保存展开状态。
func (a *App) processFormRawOpen(step *model.Step) *bool {
	if a.processRawOpen == nil {
		a.processRawOpen = map[string]*bool{}
	}
	key := step.ID
	if key == "" {
		key = step.Name
	}
	if a.processRawOpen[key] == nil {
		v := false
		a.processRawOpen[key] = &v
	}
	return a.processRawOpen[key]
}
