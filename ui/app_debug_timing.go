package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"requestflow/engine"
	"requestflow/model"
)

// timingColors 对应 HTTPTiming.Phases() 顺序的分段配色（贴近 Chrome Network Timing）。
var timingColors = []ui.Color{
	ui.Hex("#94a3b8"), // 排队/停滞 灰
	ui.Hex("#8b5cf6"), // DNS 解析 紫
	ui.Hex("#f59e0b"), // TCP 建连 橙
	ui.Hex("#ec4899"), // TLS 握手 品红
	ui.Hex("#3b82f6"), // 请求发送 蓝
	ui.Hex("#22c55e"), // 等待响应 TTFB 绿
	ui.Hex("#06b6d4"), // 内容下载 青
}

// debugTimingView 以堆叠条 + 明细列表展示一次 HTTP 请求的分段耗时（对齐 Chrome Network Timing）。
func (a *App) debugTimingView(c *ui.Context, t *ui.Theme, res *engine.DebugResult) {
	tm := res.Timing
	if tm == nil || !tm.Available {
		ui.Column(c).Fill().Center().Gap(6).Children(func() {
			ui.Text(c, "该响应没有分段耗时数据。").FontSize(11).TextColor(t.TextMuted)
			ui.Text(c, "TCP 协议或请求失败时无法采集 HTTP 各阶段耗时。").FontSize(10).TextColor(t.TextMuted)
		})
		return
	}
	phases := tm.Phases()
	ui.Column(c).Gap(10).Children(func() {
		// 概览行。
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "总耗时").FontSize(11).TextColor(t.TextMuted)
			ui.Text(c, fmt.Sprintf("%.2f ms", tm.Total)).FontSize(15).Bold()
			conn := "新建连接"
			if tm.Reused {
				conn = "复用连接"
			}
			ui.Badge(c, conn).TextColor(t.Success)
			ui.Spacer(c)
			ui.Text(c, "分段时长之和 ≈ 总耗时").FontSize(10).TextColor(t.TextMuted)
		})
		// 堆叠比例条：各段按耗时占比分配宽度，极小段给最小可见宽度。
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "时间轴（按耗时比例）").FontSize(11).Bold()
			ui.Row(c).Height(20).Radius(3).Gap(1).Children(func() {
				for i, p := range phases {
					w := float32(p.MS)
					if w < 0.5 {
						w = 0.5
					}
					ui.Box(c).Grow(w).Height(20).Radius(2).Background(timingColor(i)).Tooltip(timingTip(p))
				}
			})
		})
		// 明细列表：色块 + 段名 + 耗时 + 占比。
		ui.Column(c).Gap(5).Children(func() {
			ui.Text(c, "分段明细").FontSize(11).Bold()
			if len(phases) == 0 {
				ui.Text(c, "各阶段耗时均小于 0.001 ms，无可展示分段。").FontSize(10).TextColor(t.TextMuted)
				return
			}
			for i, p := range phases {
				i, p := i, p
				pct := 0.0
				if tm.Total > 0 {
					pct = p.MS / tm.Total * 100
				}
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Width(10).Height(10).Radius(2).Background(timingColor(i))
					ui.Text(c, p.Name).FontSize(11).Width(120)
					ui.Text(c, fmt.Sprintf("%.2f ms", p.MS)).FontSize(11).Width(90)
					ui.Text(c, fmt.Sprintf("%5.1f%%", pct)).FontSize(11).TextColor(t.TextMuted)
					// 明细条：段内占比的相对长度。
					ui.Box(c).Grow(1).Height(6).Radius(3).Background(timingColor(i)).Opacity(0.35)
				})
			}
		})
		ui.Text(c, "说明：排队/停滞含连接池等待与调度间隙；TTFB 为等待首字节时间。").FontSize(10).TextColor(t.TextMuted)
	})
}

// timingColor 返回第 i 段颜色，超出配色表时回退到灰色。
func timingColor(i int) ui.Color {
	if i >= 0 && i < len(timingColors) {
		return timingColors[i]
	}
	return ui.Hex("#94a3b8")
}

// timingTip 生成堆叠条悬浮提示。
func timingTip(p model.TimingPhase) string {
	return fmt.Sprintf("%s：%.2f ms", p.Name, p.MS)
}
