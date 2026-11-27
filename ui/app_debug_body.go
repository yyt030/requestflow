package ui

import (
	"encoding/json"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/tidwall/gjson"

	"requestflow/engine"
)

// responseBodyView 渲染调试响应的 Body / Headers / Cookies 内容。
// Body 支持原始/格式化/Hex 切换与关键字过滤；输入 JSONPath 时左右分屏对照，
// 左为完整原文本、右为实时提取结果，互不影响。所有内容可选中复制。
func (a *App) responseBodyView(c *ui.Context, t *ui.Theme, res *engine.DebugResult) {
	switch a.debugView {
	case 1: // 响应头
		a.bodyKVList(c, t, res.Headers, "（无响应头）", ": ")
	case 2: // Cookie
		a.bodyKVList(c, t, res.Cookies, "（无 Cookie）", " = ")
	default: // Body（JSONPath 分屏由上层 debugResponseArea 处理，这里只渲染单一视图）
		a.bodyTextView(c, t, res)
	}
}

// bodyJSONPathSplit 左右分屏：左=完整原文本（按当前 原始/格式化 模式），右=JSONPath 实时结果。
// 两栏各有独立 Scroll，且 Split 用 Grow(1) 撑满响应区，确保左右比例正确。
func (a *App) bodyJSONPathSplit(c *ui.Context, t *ui.Theme, res *engine.DebugResult) {
	ui.SplitPercent(c, &a.splitJSONPath, func() {
		ui.Column(c).Fill().Children(func() {
			ui.Row(c).Padding(8, 12, 4, 12).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "完整响应体").FontSize(10).Bold().TextColor(t.TextMuted)
				ui.Text(c, "（原文本，未应用 JSONPath）").FontSize(9).TextColor(t.TextMuted)
			})
			ui.Divider(c)
			ui.Scroll(c).Grow(1).Padding(8, 12, 12, 12).Children(func() {
				a.bodyTextView(c, t, res)
			})
		})
	}, func() {
		ui.Column(c).Fill().Children(func() {
			ui.Row(c).Padding(8, 12, 4, 12).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "JSONPath 结果 · "+strings.TrimSpace(a.respJSONPath)).FontSize(10).Bold().TextColor(t.Accent)
			})
			ui.Divider(c)
			ui.Scroll(c).Grow(1).Padding(8, 12, 12, 12).Children(func() {
				text, errMsg := a.responseJSONPathText(res)
				if errMsg != "" {
					ui.Text(c, errMsg).FontSize(11).TextColor(t.Danger)
					return
				}
				ui.Text(c, text).FontSize(11).Selectable()
			})
		})
	}).Grow(1)
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

// filterNoopHint 检测「输入了过滤词但按行过滤无可见变化」的场景并给出提示。
// 典型：原始模式下 JSON 是一整行，按行过滤时整行命中→整行保留，用户误以为过滤失效。
// 格式化模式为多行、Hex 模式另计，均不提示。
func (a *App) filterNoopHint(res *engine.DebugResult) string {
	q := strings.TrimSpace(a.respFilter)
	if q == "" || a.debugView != 0 || a.respBodyMode == 2 || a.respBodyMode == 1 {
		return ""
	}
	base := res.Body
	if !strings.Contains(strings.ToLower(base), strings.ToLower(q)) {
		return "" // 未命中：内容区会显示空结果，无需提示
	}
	if strings.Contains(base, "\n") {
		return "" // 多行文本，按行过滤有意义
	}
	return "提示：原文为单行，按行过滤会保留整行。切到「格式化」模式可逐行筛选。"
}

// bodyTextView 渲染响应体：按原始/格式化模式、JSONPath、关键字过滤计算结果。
func (a *App) bodyTextView(c *ui.Context, t *ui.Theme, res *engine.DebugResult) {
	if hint := a.filterNoopHint(res); hint != "" {
		ui.Text(c, hint).FontSize(10).TextColor(t.Accent)
		ui.Box(c).Height(4)
	}
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

// responseBodyText 计算完整响应体展示文本（展示与复制共用，保证所见即所得）。
// 流程：原始/格式化/Hex 模式选基准文本 → 关键字过滤。
// 重要：此函数不含 JSONPath 逻辑——JSONPath 单独在右侧分屏展示，绝不改动原文本。
// 返回 (文本, 错误提示)。
func (a *App) responseBodyText(res *engine.DebugResult) (string, string) {
	if a.respBodyMode == 2 {
		// Hex：转储响应体原始字节的十六进制（含 ASCII 区）。
		return filterLines(hexDump([]byte(res.Body)), a.respFilter), ""
	}
	base := res.Body // 原始：服务端返回原文，不经任何加工
	if a.respBodyMode == 1 {
		// 格式化：复用通用格式化，支持 JSON/XML/HTML/键值对（无法识别时返回原文）。
		base, _, _ = FormatContent(res.Body)
	}
	return filterLines(base, a.respFilter), ""
}

// responseJSONPathText 计算 JSONPath 提取结果文本，供分屏右侧实时展示。
// 格式化模式美化该节点，原始模式保留节点原文；不改动完整响应体。
// 返回 (文本, 错误提示)，错误提示非空时表示路径无效或未匹配。
func (a *App) responseJSONPathText(res *engine.DebugResult) (string, string) {
	path := strings.TrimSpace(a.respJSONPath)
	if path == "" {
		return "", ""
	}
	if !gjson.Valid(res.Body) {
		return "", "响应体不是合法 JSON，无法按 JSONPath 过滤。"
	}
	r := gjson.Get(res.Body, path)
	if !r.Exists() {
		return "", "JSONPath 未匹配到值：" + path
	}
	if a.respBodyMode == 1 {
		var v interface{}
		if err := json.Unmarshal([]byte(r.Raw), &v); err == nil {
			if b, err := json.MarshalIndent(v, "", "  "); err == nil {
				return filterLines(string(b), a.respFilter), ""
			}
		}
		return filterLines(r.String(), a.respFilter), ""
	}
	return filterLines(r.Raw, a.respFilter), ""
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
