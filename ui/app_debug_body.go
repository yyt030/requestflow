package ui

import (
	"encoding/json"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/tidwall/gjson"

	"requestflow/engine"
)

// responseBodyView 渲染调试响应的 Body / Headers / Cookies 内容。
// Body 支持 JSON/Text 切换、JSONPath 过滤与关键字过滤；所有内容可选中复制。
func (a *App) responseBodyView(c *ui.Context, t *ui.Theme, res *engine.DebugResult) {
	switch a.debugView {
	case 1: // 响应头
		a.bodyKVList(c, t, res.Headers, "（无响应头）", ": ")
	case 2: // Cookie
		a.bodyKVList(c, t, res.Cookies, "（无 Cookie）", " = ")
	default: // Body
		a.bodyTextView(c, t, res)
	}
}

// bodyKVList 渲染 Header/Cookie 键值列表，支持关键字过滤。
func (a *App) bodyKVList(c *ui.Context, t *ui.Theme, kvs []engine.KV, empty, sep string) {
	q := strings.TrimSpace(a.respFilter)
	shown := make([]engine.KV, 0, len(kvs))
	for _, kv := range kvs {
		if q == "" || strings.Contains(strings.ToLower(kv.Key+sep+kv.Value), strings.ToLower(q)) {
			shown = append(shown, kv)
		}
	}
	if len(shown) == 0 {
		ui.Text(c, empty).FontSize(11).TextColor(t.TextMuted)
		return
	}
	ui.Column(c).Gap(4).Selectable().Children(func() {
		for _, kv := range shown {
			kv := kv
			ui.Row(c).Gap(8).Children(func() {
				ui.Text(c, kv.Key+sep).FontSize(11).Bold().Width(180)
				ui.Text(c, kv.Value).FontSize(11)
			})
		}
	})
}

// bodyTextView 渲染响应体：先按 JSONPath 取值，再按 JSON/Text 模式展示，最后按关键字过滤行。
func (a *App) bodyTextView(c *ui.Context, t *ui.Theme, res *engine.DebugResult) {
	text := res.Formatted
	if a.respJSONMode == 0 {
		text = res.Body // Text：原文
	}
	// JSONPath 过滤：仅在 JSON 响应且输入了路径时生效。
	path := strings.TrimSpace(a.respJSONPath)
	if path != "" {
		if !gjson.Valid(res.Body) {
			ui.Text(c, "响应体不是合法 JSON，无法按 JSONPath 过滤。").FontSize(11).TextColor(t.Danger)
			return
		}
		r := gjson.Get(res.Body, path)
		if !r.Exists() {
			ui.Text(c, "JSONPath 未匹配到值："+path).FontSize(11).TextColor(t.Danger)
			return
		}
		// 美化该节点。
		var v interface{}
		if err := json.Unmarshal([]byte(r.Raw), &v); err == nil {
			if b, err := json.MarshalIndent(v, "", "  "); err == nil {
				text = string(b)
			} else {
				text = r.Raw
			}
		} else {
			text = r.String()
		}
	}
	ui.Text(c, filterLines(text, a.respFilter)).FontSize(11).Selectable()
}

// filterLines 按关键字过滤文本行（保留包含关键字的行；关键字为空则原文返回）。
func filterLines(text, q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return text
	}
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	for _, ln := range lines {
		if strings.Contains(strings.ToLower(ln), strings.ToLower(q)) {
			kept = append(kept, ln)
		}
	}
	return strings.Join(kept, "\n")
}
