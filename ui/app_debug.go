package ui

import (
	"fmt"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

// debugPanel 接口调试面板：上半请求配置，下半响应展示。
func (a *App) debugPanel(c *ui.Context, t *ui.Theme) {
	ui.SplitVertical(c, &a.splitDebug, func() {
		a.debugRequestArea(c, t)
	}, func() {
		a.debugResponseArea(c, t)
	}).Fill()
}

// debugRequestArea Postman 风格请求构建区：请求行(方法徽章+URL+Send) + 配置子标签。

// debugRequestArea Postman 风格请求构建区：请求行(方法徽章+URL+Send) + 配置子标签。
func (a *App) debugRequestArea(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Children(func() {
		ui.Column(c).Gap(8).Padding(10, 12, 6, 12).Children(func() {
			// 标题行。
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "接口调试").FontSize(13).Bold()
				ui.Text(c, "单独发送请求并查看响应").FontSize(10).TextColor(t.TextMuted)
				ui.Spacer(c)
				if a.debugResult != nil && a.debugResult.Curl != "" {
					if ui.Button(c, "⇥ 导入此 cURL").Clicked() {
						a.importText = a.debugResult.Curl
						a.showImport = true
						a.mainTab = 0
					}
				}
			})
			// 请求行：方法色标 + 方法选择 + URL + 醒目 Send（Postman 标志性布局）。
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				method := a.debugMethod
				if method == "" {
					method = "GET"
				}
				ui.Box(c).Width(6).Height(30).Radius(3).Background(methodColor(method))
				ui.Select(c, &a.debugMethod, []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"}).Width(96)
				ui.TextInput(c, &a.debugURL).Placeholder("{{baseUrl}}/path 或完整 URL").Grow(1).Height(30)
				if ui.PrimaryButton(c, "▶ 发送").Disabled(a.debugSending).Clicked() {
					a.doDebugSend()
				}
			})
			// 超时与证书：收起为一行小字，默认不占视觉重心。
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "超时(秒)").FontSize(10).TextColor(t.TextMuted)
				ui.TextInput(c, &a.debugTimeout).Width(60)
				ui.Checkbox(c, &a.debugInsecure, "跳过 TLS 校验")
			})
		})
		// 配置子标签（Postman：Params / Headers / Body / Authorization）。
		// 相对最近载入内容改过的标签，名字后带绿色圆点，便于定位改动。
		ui.Row(c).Padding(0, 12).AlignItems(ui.Center).Children(func() {
			dirtyCount := a.debugTabBarWithDots(c, t)
			ui.Spacer(c)
			if dirtyCount > 0 {
				ui.Text(c, "已修改").FontSize(10).TextColor(t.Success)
			}
			ui.Text(c, "支持 {{变量}}").FontSize(10).TextColor(t.TextMuted)
		})
		ui.Divider(c)
		ui.Scroll(c).Grow(1).Padding(10, 12, 8, 12).Children(func() {
			switch a.debugReqTab {
			case 0: // Params
				fieldHint(c, t, "查询参数，每行 key: value；发送时拼接到 URL。")
				resizableTextArea(c, t, &a.debugQuery, &a.queryHeight, "page: 1", 40, 600)
			case 1: // Headers
				fieldHint(c, t, "请求头，每行 key: value；可直接粘贴 Chrome 复制的请求头。")
				resizableTextArea(c, t, &a.debugHeaders, &a.headersHeight, "Authorization: Bearer {{token}}", 40, 600)
			case 2: // Body
				bodyEditor(c, t, &a.debugBodyType, &a.debugBody, &a.debugFormData,
					&a.debugGQLQuery, &a.debugGQLVars, &a.bodyHeight, `{"key": "{{value}}"}`, &a.debugBodyFmtMsg)
			case 3: // Authorization
				authEditor(c, t, &a.debugAuthType, &a.debugAuthToken, &a.debugUser, &a.debugPass,
					&a.debugKeyName, &a.debugKeyValue, &a.debugKeyIn,
					&a.debugTokenURL, &a.debugClientID, &a.debugClientSecret, &a.debugScope, &a.debugGrantType)
			}
		})
	})
}

// debugResponseArea Postman 风格响应区：状态/耗时/大小概览 + 分段标签。

// debugResponseArea Postman 风格响应区：状态/耗时/大小概览 + 分段标签。
func (a *App) debugResponseArea(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Padding(8, 12).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Segmented(c, &a.debugView, "Body", "Headers", "Cookies", "代码")
			if a.debugResult != nil && a.debugResult.OK && a.debugView < 3 {
				if ui.Button(c, "⧉ 复制").Tooltip("复制当前标签内容到剪贴板").Clicked() {
					a.copyDebugResponse()
				}
			}
			ui.Spacer(c)
			if a.debugSending {
				ui.Text(c, "发送中…").FontSize(11).TextColor(t.Accent)
			} else if a.debugResult != nil {
				res := a.debugResult
				if res.OK {
					statusColor := t.Success
					if res.StatusCode >= 400 {
						statusColor = t.Danger
					}
					ui.Badge(c, fmt.Sprintf("%d %s", res.StatusCode, res.StatusText)).TextColor(statusColor)
					ui.Text(c, fmt.Sprintf("%d ms", res.DurationMS)).FontSize(11).TextColor(t.TextMuted)
					ui.Text(c, fmt.Sprintf("%d B", res.Size)).FontSize(11).TextColor(t.TextMuted)
				} else {
					ui.Text(c, "✗ 请求失败").FontSize(11).Bold().TextColor(t.Danger)
				}
			}
		})
		ui.Divider(c)
		// 底部留白：抵消 Selectable 文本末行实测高度小于绘制高度的误差，
		// 保证滚到底时末行（如 JSON 结尾大括号）完整露出、不被底部状态栏压住。
		ui.Scroll(c).Grow(1).Padding(10, 12, 40, 12).Children(func() {
			// 代码生成标签：基于最近一次调试请求生成，无需请求成功即可查看。
			if a.debugView == 3 {
				a.codeSnippetView(c, t)
				return
			}
			if a.debugSending || a.debugResult == nil {
				ui.Column(c).Fill().Center().Gap(6).Children(func() {
					ui.Icon(c, iconSend).Size(24, 24).TextColor(t.TextMuted)
					ui.Text(c, "配置好请求后点击【发送】，响应将显示在这里。").FontSize(11).TextColor(t.TextMuted)
				})
				return
			}
			res := a.debugResult
			if !res.OK {
				ui.Text(c, "错误: "+res.Err).FontSize(11).TextColor(t.Danger)
				return
			}
			a.responseBodyView(c, t, res)
		})
	})
}

// codeSnippetView 代码生成面板：把当前请求转成 Python/Go/JS/cURL 代码片段。

// codeSnippetView 代码生成面板：把当前请求转成 Python/Go/JS/cURL 代码片段。
func (a *App) codeSnippetView(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Gap(8).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "代码生成").FontSize(12).Bold()
			langs := []string{"python", "go", "javascript", "curl"}
			lang := a.snippetLang
			if ui.Select(c, &lang, langs).Label("语言").Width(150).Changed() {
				a.snippetLang = lang
			}
			ui.Spacer(c)
			if ui.Button(c, "⧉ 复制代码").Clicked() {
				if a.snippetText != "" {
					_ = mygo.Clipboard.Write(transfer.New(transfer.NewItem(
						transfer.Bytes(transfer.Text, []byte(a.snippetText)),
					)))
					a.log("代码片段已复制到剪贴板")
				}
			}
		})
		fieldHint(c, t, "基于最近一次调试的请求（已解析 {{变量}}）生成；先发送一次即可得到对应代码。")
		if a.debugResult == nil || len(a.debugResult.Snippets) == 0 {
			ui.Column(c).Fill().Center().Gap(6).Children(func() {
				ui.Text(c, "先点击【发送】，再切换到此标签查看生成的代码。").FontSize(11).TextColor(t.TextMuted)
			})
			return
		}
		code := a.debugResult.Snippets[a.snippetLang]
		a.snippetText = code
		resizableTextArea(c, t, &a.snippetText, &a.importHeight, "", 120, 800)
	})
}

// doDebugSend 异步发送调试请求，避免阻塞 UI 线程。
