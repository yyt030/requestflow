package ui

import "github.com/egoist/mygo/ui"

// mainArea 右侧主工作区：Postman 风格标签 + 内容视图 + 内嵌状态栏。
// 状态栏作为主区 Column 的最后一行，只占主区宽度，不再横跨整个窗口
// （左侧资源管理器下方不再有状态栏），因此不会再"浮"在内容框之上。
func (a *App) mainArea(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Children(func() {
		// 标签行固定高度。
		ui.Row(c).Padding(6, 12, 0, 12).Children(func() {
			ui.Tabs(c, &a.mainTab, "步骤编排", "接口调试", "运行结果")
		})
		ui.Divider(c)
		// 视图区 Grow(1) 吃满除状态栏外的剩余高度；各视图内部 Fill() 相对此容器分配。
		ui.Column(c).Grow(1).Children(func() {
			switch a.mainTab {
			case 1:
				a.debugPanel(c, t)
			case 2:
				a.consoleOnly(c, t)
			default:
				a.editor(c, t)
			}
		})
		// 状态栏嵌入主区底部，独立占据一行（Divider + 固定高度行）。
		a.statusBar(c, t)
	})
}
