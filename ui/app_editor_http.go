package ui

import (
	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

// editorRequestScripts 请求级脚本（Postman 风格 Pre-request / Tests）。
// Pre-request 在发请求前执行、可改写本次请求变量；Tests 在响应后执行、可读写变量。

// editorRequestScripts 请求级脚本（Postman 风格 Pre-request / Tests）。
// Pre-request 在发请求前执行、可改写本次请求变量；Tests 在响应后执行、可读写变量。
func (a *App) editorRequestScripts(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "请求脚本").FontSize(12).Bold()
		langs := []string{"js", "python"}
		lang := step.ScriptLang
		if lang == "" {
			lang = "js"
		}
		if ui.Select(c, &lang, langs).Label("语言").Width(130).Changed() {
			step.ScriptLang = lang
		}
	})
	fieldHint(c, t, "Pre-request 在请求前执行（vars 读写变量，可影响本次 URL/头/体）；Tests 在响应后执行（last 取响应）。")
	ui.Row(c).Gap(10).Children(func() {
		ui.Column(c).Grow(1).Children(func() {
			ui.Text(c, "Pre-request Script").FontSize(11).Bold()
			fieldHint(c, t, "示例：vars.token = 'abc'（JS）")
			resizableTextArea(c, t, &step.PreScript, &a.preScriptHeight,
				"// vars.foo = 'bar'", 48, 400)
		})
		ui.Column(c).Grow(1).Children(func() {
			ui.Text(c, "Tests (PostScript)").FontSize(11).Bold()
			fieldHint(c, t, "示例：vars.saved = last.json.data.id（JS）")
			resizableTextArea(c, t, &step.PostScript, &a.postScriptHeight,
				"// vars.saved = last.json.data.id", 48, 400)
		})
	})
}

// tcpFrameTypeLabels 是帧头类型下拉的中文展示项；存盘值由 tcpFrameTypeValue 映射为英文。
