package ui

import (
	"github.com/egoist/mygo/ui"

	"requestflow/engine"
)

// responseBodyView 渲染调试响应的 Body / Headers / Cookies 内容。
// 所有内容都加 Selectable：可用鼠标拖选、双击选词、三击选行，右键复制或 Cmd+C，
// 与标签栏的「⧉ 复制」（整段复制）互补，满足「返回内容可选择复制」的诉求。
func (a *App) responseBodyView(c *ui.Context, t *ui.Theme, res *engine.DebugResult) {
	switch a.debugView {
	case 1: // 响应头：整块共享一个选区，跨行复制按换行拼接
		if len(res.Headers) == 0 {
			ui.Text(c, "（无响应头）").FontSize(11).TextColor(t.TextMuted)
			return
		}
		ui.Column(c).Gap(4).Selectable().Children(func() {
			for _, kv := range res.Headers {
				kv := kv
				ui.Row(c).Gap(8).Children(func() {
					ui.Text(c, kv.Key+":").FontSize(11).Bold().Width(180)
					ui.Text(c, kv.Value).FontSize(11)
				})
			}
		})
	case 2: // Cookie：整块共享一个选区
		if len(res.Cookies) == 0 {
			ui.Text(c, "（无 Cookie）").FontSize(11).TextColor(t.TextMuted)
			return
		}
		ui.Column(c).Gap(4).Selectable().Children(func() {
			for _, kv := range res.Cookies {
				ui.Text(c, kv.Key+" = "+kv.Value).FontSize(11)
			}
		})
	default: // 响应体：单个文本段内可选（含多行，拖动/三击选行）
		ui.Text(c, res.Formatted).FontSize(11).Selectable()
	}
}
