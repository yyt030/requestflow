package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// rightDrawer 右侧抽屉：默认收起为窄条，按需展开承载环境变量 / 代码片段 /
// 历史详情 / 流程变量四类辅助内容。用抽屉而非常驻右栏，避免简单版界面过重。
func (a *App) rightDrawer(c *ui.Context, t *ui.Theme) {
	if !a.drawerOpen {
		// 收起态：一条窄竖栏，仅一个展开按钮，纵向排布。
		ui.Column(c).Fill().AlignItems(ui.Center).Padding(6, 4).Gap(10).Background(t.Surface).Children(func() {
			if ui.Button(c, "‹").Tooltip("展开右侧抽屉（变量 / 代码 / 历史 / 流程变量）").Width(24).Clicked() {
				a.drawerOpen = true
			}
			ui.Text(c, "辅").FontSize(10).TextColor(t.TextMuted)
			ui.Text(c, "助").FontSize(10).TextColor(t.TextMuted)
			ui.Text(c, "面").FontSize(10).TextColor(t.TextMuted)
			ui.Text(c, "板").FontSize(10).TextColor(t.TextMuted)
		})
		return
	}

	ui.Column(c).Fill().Children(func() {
		// 抽屉标题栏：四块切换 + 收起按钮。
		ui.Row(c).Padding(8, 10).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "辅助面板").FontSize(12).Bold()
			ui.Spacer(c)
			if ui.Button(c, "›").Tooltip("收起抽屉").Width(24).Clicked() {
				a.drawerOpen = false
			}
		})
		ui.Row(c).Padding(0, 8, 8, 8).Children(func() {
			ui.Segmented(c, &a.drawerSection, "变量", "代码", "历史", "流程")
		})
		ui.Divider(c)
		ui.Scroll(c).Grow(1).Padding(10, 12).Children(func() {
			switch a.drawerSection {
			case 0:
				a.drawerVariables(c, t)
			case 1:
				a.drawerSnippet(c, t)
			case 2:
				a.drawerHistory(c, t)
			case 3:
				a.drawerFlowVars(c, t)
			}
		})
	})
}

// drawerVariables 环境变量：复用侧栏「变量」标签的环境编辑器。
func (a *App) drawerVariables(c *ui.Context, t *ui.Theme) {
	ui.Text(c, "环境变量").FontSize(12).Bold()
	fieldHint(c, t, "环境变量覆盖同名全局变量；运行时按 环境变量 > 全局变量 解析。")
	ui.Box(c).Height(6)
	a.environmentEditor(c, t)
}

// drawerSnippet 请求代码片段：复用响应区「代码」标签的代码生成视图。
func (a *App) drawerSnippet(c *ui.Context, t *ui.Theme) {
	ui.Text(c, "请求代码片段").FontSize(12).Bold()
	fieldHint(c, t, "基于最近一次调试请求生成；先发送一次即可得到对应代码。")
	ui.Box(c).Height(6)
	a.codeSnippetView(c, t)
}

// drawerHistory 历史详情：紧凑版历史列表（复用历史数据，独立布局）。
func (a *App) drawerHistory(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "历史详情").FontSize(12).Bold()
		ui.Spacer(c)
		if ui.Button(c, "↻").Tooltip("刷新历史").Clicked() {
			a.openHistory()
		}
	})
	fieldHint(c, t, fmt.Sprintf("共 %d 条请求记录。", len(a.historyList)))
	ui.Box(c).Height(6)
	if len(a.historyList) == 0 {
		ui.Text(c, "暂无历史。发送一次调试请求后会自动记录。").FontSize(11).TextColor(t.TextMuted)
		return
	}
	ui.Column(c).Gap(4).Children(func() {
		for _, e := range a.historyList {
			ent := e
			ui.Row(c).Gap(6).Padding(6, 8).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Width(4).Height(20).Radius(2).Background(methodColor(ent.Method))
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, ent.Method+"  "+previewURL(ent.URL)).FontSize(10).Bold().SingleLine()
					status := ""
					if ent.Error != "" {
						status = "失败: " + ent.Error
					} else {
						status = fmt.Sprintf("%d · %d ms", ent.Status, ent.DurationMS)
					}
					ui.Text(c, ent.Time+"  "+status).FontSize(9).TextColor(t.TextMuted)
				})
				if ui.Button(c, "载入").Clicked() {
					a.loadHistoryToDebug(ent)
				}
			})
		}
	})
}

// drawerFlowVars 流程变量：展示最近一次运行/数据驱动的运行时变量。
func (a *App) drawerFlowVars(c *ui.Context, t *ui.Theme) {
	ui.Text(c, "流程变量").FontSize(12).Bold()
	fieldHint(c, t, "最近一次运行的运行时变量快照（含提取变量与数据驱动列）。")
	ui.Box(c).Height(6)
	if a.lastResult == nil && len(a.dataResults) == 0 {
		ui.Text(c, "尚未运行。点击顶栏【▶ 运行】后查看变量。").FontSize(11).TextColor(t.TextMuted)
		return
	}
	if a.lastResult != nil {
		ui.Text(c, fmt.Sprintf("单次运行 · %d 个变量", len(a.lastResult.Vars))).FontSize(11).Bold()
		ui.Box(c).Height(4)
		a.drawerVarList(c, t, a.lastResult.Vars)
	}
	if len(a.dataResults) > 0 {
		ui.Box(c).Height(10)
		ui.Text(c, fmt.Sprintf("数据驱动 · %d 轮", len(a.dataResults))).FontSize(11).Bold()
		for i, dr := range a.dataResults {
			if i >= 8 {
				break
			}
			ui.Text(c, fmt.Sprintf("第 %d 轮 · %d 变量", i+1, len(dr.Vars))).FontSize(10).TextColor(t.TextMuted)
		}
	}
}

// drawerVarList 变量名 / 值列表。
func (a *App) drawerVarList(c *ui.Context, t *ui.Theme, vars map[string]interface{}) {
	if len(vars) == 0 {
		ui.Text(c, "（无变量）").FontSize(10).TextColor(t.TextMuted)
		return
	}
	ui.Column(c).Gap(3).Children(func() {
		for k, v := range vars {
			key, val := k, v
			ui.Row(c).Gap(6).AlignItems(ui.Start).Children(func() {
				ui.Text(c, key).FontSize(10).Bold().Width(90).TextColor(t.Accent)
				ui.Text(c, truncateRunes(fmt.Sprint(val), 80)).FontSize(10).Grow(1).Selectable()
			})
		}
	})
}
