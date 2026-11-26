package ui

import (
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

// stringToMap 把多行文本解析为 key→value，兼容两种写法：
//   - "key: value"（界面提示格式，也是 Chrome DevTools「Copy request headers」的格式）
//   - "key = value"（mapToString 的输出格式）
//
// 以行内最先出现的 ":" 或 "=" 作为分隔符且只切一次，确保值里的 "=" 不会被误当分隔符
// （Cookie、Bearer token 等经常含 "="）。以 ":" 开头的 HTTP/2 伪头（:authority、:method 等）
// 会被跳过——Go 的 HTTP 客户端不接受这类头。
// bodyTypes 请求体类型下拉选项（Postman 风格）。
var bodyTypes = []string{"none", "raw", "form-data", "urlencoded", "binary", "graphql"}

// authTypes 认证方式下拉选项。

// authTypes 认证方式下拉选项。
var authTypes = []string{"none", "bearer", "basic", "apikey", "oauth2"}

// bodyEditor 渲染请求体类型选择 + 对应编辑控件。 pointers 让步骤编辑器与调试面板复用。

// bodyEditor 渲染请求体类型选择 + 对应编辑控件。 pointers 让步骤编辑器与调试面板复用。
func bodyEditor(c *ui.Context, t *ui.Theme, bodyType *string, body *string, form *[]model.FormField,
	gqlQuery *string, gqlVars *string, height *float32, gqlVarsHeight *float32, placeholder string, fmtMsg *string) {
	bt := *bodyType
	if bt == "" {
		bt = "raw"
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "Body 类型").FontSize(11).Bold()
		if ui.Select(c, &bt, bodyTypes).Width(130).Changed() {
			*bodyType = bt
		}
	})
	switch strings.ToLower(bt) {
	case "none", "raw", "binary":
		hint := "支持 {{变量}}；JSON 会自动设 Content-Type。"
		if bt == "binary" {
			hint = "填写要上传的文件路径，以 application/octet-stream 发送。"
		}
		fieldHint(c, t, hint)
		if fmtMsg != nil {
			formattedTextArea(c, t, body, height, placeholder, 40, 800, fmtMsg)
		} else {
			resizableTextArea(c, t, body, height, placeholder, 40, 800)
		}
	case "form-data", "urlencoded":
		fieldHint(c, t, "逐行添加字段；类型选 file 时值为本地文件路径（form-data 支持上传）。")
		fields := *form
		removeIdx := -1 // 删除延后到循环结束，避免遍历中修改切片导致索引越界闪退
		for i := range fields {
			f := &fields[i]
			idx := i
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.TextInput(c, &f.Key).Placeholder("字段名").Width(130)
				ft := f.Type
				if ft == "" {
					ft = "text"
				}
				if bt == "urlencoded" {
					ft = "text" // urlencoded 无文件类型
				}
				if ui.Select(c, &ft, []string{"text", "file"}).Width(76).Changed() {
					f.Type = ft
				}
				ui.TextInput(c, &f.Value).Placeholder("值或文件路径").Grow(1)
				if ui.Button(c.Key("delf"), "×").Width(26).Clicked() {
					removeIdx = idx
				}
			})
		}
		if removeIdx >= 0 && removeIdx < len(fields) {
			*form = append(fields[:removeIdx], fields[removeIdx+1:]...)
		}
		if ui.Button(c, "+ 添加字段").Clicked() {
			*form = append(*form, model.FormField{Type: "text"})
		}
	case "graphql":
		fieldHint(c, t, "Query 填写 GraphQL 查询；Variables 为 JSON 对象（可留空 {}）。")
		ui.Text(c, "Query").FontSize(10).TextColor(t.TextMuted)
		resizableTextArea(c, t, gqlQuery, height, "query { me { id } }", 40, 600)
		ui.Text(c, "Variables (JSON)").FontSize(10).TextColor(t.TextMuted)
		resizableTextArea(c, t, gqlVars, gqlVarsHeight, "{}", 40, 600)
	}
}

// authEditor 渲染认证方式选择与对应凭据输入，复用指针以兼容步骤编辑器与调试面板。

// authEditor 渲染认证方式选择与对应凭据输入，复用指针以兼容步骤编辑器与调试面板。
func authEditor(c *ui.Context, t *ui.Theme, authType *string, token, user, pass *string,
	keyName, keyValue, keyIn *string, tokenURL, clientID, clientSecret, scope, grantType *string) {
	at := *authType
	if at == "" {
		at = "none"
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "认证方式").FontSize(11).Bold()
		if ui.Select(c, &at, authTypes).Width(130).Changed() {
			*authType = at
		}
	})
	fieldHint(c, t, "认证头/参数自动加入请求；none 表示不认证。")
	switch strings.ToLower(at) {
	case "bearer":
		ui.TextInput(c, token).Placeholder("{{token}}").Label("Bearer Token")
	case "basic":
		ui.Row(c).Gap(8).Children(func() {
			ui.TextInput(c, user).Placeholder("用户名").Label("用户名").Grow(1)
			ui.TextInput(c, pass).Password().Placeholder("密码").Label("密码").Grow(1)
		})
	case "apikey":
		fieldHint(c, t, "在请求头或 URL 查询参数中加入指定的 Key。")
		ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
			ui.TextInput(c, keyName).Placeholder("如 X-API-Key").Label("参数名").Grow(1)
			ki := *keyIn
			if ki == "" {
				ki = "header"
			}
			if ui.Select(c, &ki, []string{"header", "query"}).Width(96).Label("位置").Changed() {
				*keyIn = ki
			}
		})
		ui.TextInput(c, keyValue).Placeholder("{{apiKey}}").Label("参数值")
	case "oauth2":
		fieldHint(c, t, "自动向 Token URL 换取 access_token 并以 Bearer 注入；同一凭据会缓存复用。")
		ui.TextInput(c, tokenURL).Placeholder("https://api.example.com/oauth/token").Label("Token URL")
		ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
			gt := *grantType
			if gt == "" {
				gt = "client_credentials"
			}
			if ui.Select(c, &gt, []string{"client_credentials", "password"}).Width(170).Label("授权类型").Changed() {
				*grantType = gt
			}
			ui.TextInput(c, scope).Placeholder("可选，如 read write").Label("Scope").Grow(1)
		})
		ui.Row(c).Gap(8).Children(func() {
			ui.TextInput(c, clientID).Placeholder("Client ID").Label("Client ID").Grow(1)
			ui.TextInput(c, clientSecret).Password().Placeholder("Client Secret").Label("Client Secret").Grow(1)
		})
		if strings.EqualFold(*grantType, "password") {
			ui.Row(c).Gap(8).Children(func() {
				ui.TextInput(c, user).Placeholder("用户名").Label("用户名").Grow(1)
				ui.TextInput(c, pass).Password().Placeholder("密码").Label("密码").Grow(1)
			})
		}
	}
}

// formDataToString 把表单字段序列化为多行文本，值以 @ 开头表示文件字段。

// formDataToString 把表单字段序列化为多行文本，值以 @ 开头表示文件字段。
func formDataToString(fields []model.FormField) string {
	lines := make([]string, 0, len(fields))
	for _, f := range fields {
		v := f.Value
		if strings.EqualFold(f.Type, "file") && !strings.HasPrefix(v, "@") {
			v = "@" + v
		}
		lines = append(lines, f.Key+": "+v)
	}
	return strings.Join(lines, "\n")
}

// stringToFormData 解析表单文本：每行 key: value；值以 @ 开头视为文件路径（Type=file）。

// stringToFormData 解析表单文本：每行 key: value；值以 @ 开头视为文件路径（Type=file）。
func stringToFormData(s string) []model.FormField {
	out := []model.FormField{}
	for _, raw := range strings.Split(s, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		i := strings.IndexAny(line, ":=")
		if i <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:i])
		val := strings.TrimSpace(line[i+1:])
		if key == "" {
			continue
		}
		f := model.FormField{Key: key, Type: "text", Value: val}
		if strings.HasPrefix(val, "@") && len(val) > 1 {
			f.Type = "file"
			f.Value = strings.TrimSpace(val[1:])
		}
		out = append(out, f)
	}
	return out
}

func stringToMap(s, _ string) map[string]string {
	out := map[string]string{}
	for _, raw := range strings.Split(s, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		i := strings.IndexAny(line, ":=")
		if i <= 0 { // 无分隔符，或以 ":" 开头（HTTP/2 伪头 / 空 key）
			continue
		}
		key := strings.TrimSpace(line[:i])
		if key == "" {
			continue
		}
		out[key] = strings.TrimSpace(line[i+1:])
	}
	return out
}

var _ = filepath.Join
