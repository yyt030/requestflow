package ui

import (
	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

// processOps 所有 process 步骤统一的处理操作列表，避免编排/子步骤下拉不一致。
func processOps() []string {
	return []string{"set", "append", "json_to_table", "filter", "sort", "pick", "flatten", "dedup",
		"zip", "row", "limit", "resolve_url", "format_time", "join", "base64encode", "base64decode", "urlencode", "urldecode"}
}

func (a *App) editorProcess(c *ui.Context, t *ui.Theme, step *model.Step) {
	ops := processOps()
	op := step.Op
	if op == "" {
		op = "limit"
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if ui.Select(c, &op, ops).Label("处理操作").Width(180).Changed() {
			step.Op = op
		}
		ui.Text(c, procOpLabel(op)).FontSize(11).Bold().TextColor(t.Accent)
	})
	names := collectVarNames(a.scenario)
	fieldHint(c, t, varSummary(names))
	a.renderProcessForm(c, t, step, a.processFormRawOpen(step), names)
	// 实时预览：复用最近响应提取的变量，自动展示处理结果（含默认例子）。
	a.processPreview(c, t, step)
}
