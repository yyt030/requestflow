package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// historyTreeFolder History 顶层文件夹：折叠收起，展开显示历史请求行。
func (a *App) historyTreeFolder(c *ui.Context, t *ui.Theme) {
	if a.treeTopExpanded == nil {
		a.treeTopExpanded = map[string]bool{}
	}
	open := a.treeFlag(a.treeTopExpanded, "history", false)
	key := "tree-top-history"
	a.treeFolderRow(c, t, key, &open, iconFolder, brandOrange, func() {
		ui.Text(c, "History").FontSize(11).Bold()
		ui.Text(c, fmt.Sprintf("(%d)", len(a.historyList))).FontSize(10).TextColor(t.TextMuted)
	}, func() {
		if treeTextButton(c, t, key+"-refresh", "↻", "刷新历史", t.TextMuted, 20).Clicked() {
			a.openHistory()
		}
	})
	a.treeTopExpanded["history"] = open
	if !open {
		return
	}
	if len(a.historyList) == 0 {
		ui.Text(c, "  暂无历史，发送一次请求后自动记录").FontSize(10).TextColor(t.TextMuted).Padding(4)
		return
	}
	for i, e := range a.historyList {
		ent := e
		treeLine(c, fmt.Sprintf("tree-hist-%d", i), nil,
			func() { ui.Box(c).Width(4).Height(16).Radius(2).Background(methodColor(ent.Method)) },
			func() {
				btn := ui.ButtonBase(c.Key("name")).Grow(1).Padding(2, 0).Radius(4).
					Justify(ui.Start).AlignItems(ui.Center).Tooltip(ent.Time)
				if btn.Hovered() {
					btn.Background(t.SurfaceHover)
				}
				btn.Children(func() {
					ui.Text(c, ent.Method+" "+previewURL(ent.URL)).FontSize(10).SingleLine()
				})
				if btn.Clicked() {
					a.loadHistoryToDebug(ent)
					a.selectedStep = -1
				}
			}, nil)
	}
}
