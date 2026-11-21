package ui

import (
	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

func (a *App) editorRequest(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Text(c, "请求").FontSize(12).Bold()
	fieldHint(c, t, "发送一个 HTTP 请求；响应可提取为变量、供后续步骤使用。URL 与字段均支持 {{变量名}}。")
	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE"}
	method := step.Method
	if method == "" {
		method = "GET"
	}
	// 方法与 URL 同一行：方法短，不必各占一列。
	ui.Row(c).Gap(8).Children(func() {
		if ui.Select(c, &method, methods).Label("方法").Width(110).Changed() {
			step.Method = method
		}
		ui.TextInput(c, &step.URL).Placeholder("{{baseUrl}}/path 或完整 URL").Label("URL").Grow(1)
	})
	fieldHint(c, t, "相对路径自动拼接场景 Base URL；写完整 http(s):// 则忽略 Base URL。")

	// Headers 与 Query 并排两列：都是短小的 key=value 列表。
	ui.Row(c).Gap(10).Children(func() {
		ui.Column(c).Grow(1).Children(func() {
			ui.Text(c, "请求头 Headers").FontSize(11).Bold()
			fieldHint(c, t, "每行 key: value，如 Authorization: Bearer {{token}}")
			headerText := mapToString(step.Headers)
			if ui.TextArea(c, &headerText).Height(64).Changed() {
				step.Headers = stringToMap(headerText, ":")
			}
		})
		ui.Column(c).Grow(1).Children(func() {
			ui.Text(c, "查询参数 Query").FontSize(11).Bold()
			fieldHint(c, t, "每行 key: value，如 page: {{page}}，自动拼到 URL")
			queryText := mapToString(step.Query)
			if ui.TextArea(c, &queryText).Height(64).Changed() {
				step.Query = stringToMap(queryText, ":")
			}
		})
	})

	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "请求体 Body").FontSize(11).Bold()
	})
	bodyEditor(c, t, &step.BodyType, &step.Body, &step.FormData,
		&step.GraphQLQuery, &step.GraphQLVariables, &a.bodyHeight, `{"key": "{{value}}"}`, &a.bodyFmtMsg)

	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if ui.Button(c, "⚡ 试发送此请求").Clicked() {
			a.debugStep(step)
		}
		ui.Text(c, "用当前步骤配置单独发起调试，不影响场景").FontSize(10).TextColor(t.TextMuted)
	})

	ui.Divider(c)
	ui.Text(c, "认证").FontSize(12).Bold()
	authEditor(c, t, &step.AuthType, &step.AuthToken, &step.AuthUser, &step.AuthPass,
		&step.AuthKeyName, &step.AuthKeyValue, &step.AuthKeyIn,
		&step.AuthTokenURL, &step.AuthClientID, &step.AuthClientSecret, &step.AuthScope, &step.AuthGrantType)

	ui.Divider(c)
	a.editorRequestScripts(c, t, step)
	a.editorExtract(c, t, step)
	a.editorAssertList(c, t, step)
}

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
