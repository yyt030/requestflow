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

// bodyTextView 渲染响应体：按原始/格式化模式、JSONPath、关键字过滤计算结果。
func (a *App) bodyTextView(c *ui.Context, t *ui.Theme, res *engine.DebugResult) {
	text, errMsg := a.responseBodyText(res)
	if errMsg != "" {
		ui.Text(c, errMsg).FontSize(11).TextColor(t.Danger)
		return
	}
	tx := ui.Text(c, text).FontSize(11).Selectable()
	if a.respBodyMode == 2 {
		// Hex 转储靠空格对齐列，必须用等宽字体，否则字节列与 ASCII 区会错位。
		tx.Font("monospace")
	}
}

// responseBodyText 计算响应体最终展示文本，展示与复制共用，保证所见即所得。
// 流程：原始/格式化/Hex 模式选基准文本 → JSONPath 取值 → 关键字过滤。
// 返回 (文本, 错误提示)，错误提示非空时调用方应展示错误。
func (a *App) responseBodyText(res *engine.DebugResult) (string, string) {
	if a.respBodyMode == 2 {
		// Hex：转储响应体原始字节的十六进制（含 ASCII 区），忽略 JSONPath（字节级视图无节点概念）。
		return filterLines(hexDump([]byte(res.Body)), a.respFilter), ""
	}
	base := res.Body // 原始：服务端返回原文，不经任何加工
	if a.respBodyMode == 1 {
		// 格式化：复用通用格式化，支持 JSON/XML/HTML/键值对（无法识别时返回原文）。
		base, _, _ = FormatContent(res.Body)
	}
	// JSONPath 过滤：仅在 JSON 响应且输入了路径时生效。
	path := strings.TrimSpace(a.respJSONPath)
	if path != "" {
		if !gjson.Valid(res.Body) {
			return "", "响应体不是合法 JSON，无法按 JSONPath 过滤。"
		}
		r := gjson.Get(res.Body, path)
		if !r.Exists() {
			return "", "JSONPath 未匹配到值：" + path
		}
		// 按当前模式决定是否美化该节点：格式化模式美化，原始模式保留节点原文。
		if a.respBodyMode == 1 {
			var v interface{}
			if err := json.Unmarshal([]byte(r.Raw), &v); err == nil {
				if b, err := json.MarshalIndent(v, "", "  "); err == nil {
					base = string(b)
				} else {
					base = r.Raw
				}
			} else {
				base = r.String()
			}
		} else {
			base = r.Raw
		}
	}
	return filterLines(base, a.respFilter), ""
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
