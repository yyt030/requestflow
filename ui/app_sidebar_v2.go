package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

// ── Collections：请求集合树（步骤树）+ 右键菜单 ──

// collectionsView 把当前场景的步骤渲染为集合树。单击选中并在中央打开请求标签，
// 右键提供 新建/重命名/删除/复制。支持顶栏/侧栏全局搜索过滤。
func (a *App) collectionsView(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Padding(4, 4, 8, 4).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "Collections").FontSize(11).Bold()
		ui.Text(c, fmt.Sprintf("%d 项", len(a.scenario.Steps))).FontSize(10).TextColor(t.TextMuted)
	})
	q := strings.ToLower(strings.TrimSpace(a.globalSearch))
	visible := make([]int, 0, len(a.scenario.Steps))
	for i := range a.scenario.Steps {
		s := a.scenario.Steps[i]
		if q == "" || strings.Contains(strings.ToLower(s.Name), q) || strings.Contains(strings.ToLower(string(s.Type)), q) {
			visible = append(visible, i)
		}
	}
	// 根节点：场景名 + 新建。
	a.treeFolderRow(c, t, "col-root", &a.showSteps, iconFolder, brandOrange, func() {
		ui.Text(c, a.scenario.Name).FontSize(11).Bold().SingleLine()
	}, func() {
		if treeTextButton(c, t, "col-add", "＋", "新建请求步骤", t.TextMuted, 22).Clicked() {
			a.appendStep(model.StepRequest)
		}
	})
	if a.showSteps {
		for _, idx := range visible {
			a.collectionRow(c, t, idx)
		}
		if len(visible) == 0 {
			ui.Text(c, "（无匹配项）").FontSize(10).TextColor(t.TextMuted).Padding(6)
		}
	}
}

// collectionRow 单个集合节点：单击在中央打开请求标签；右键弹菜单。
func (a *App) collectionRow(c *ui.Context, t *ui.Theme, idx int) {
	step := &a.scenario.Steps[idx]
	selected := idx == a.selectedStep
	sc := stepColors[step.Type]
	iconColor := sc
	if !step.Enabled {
		iconColor = t.TextMuted
	}
	name := step.Name
	if name == "" {
		name = string(step.Type)
	}
	nameColor := t.Text
	if selected {
		nameColor = t.Accent
	} else if !step.Enabled {
		nameColor = t.TextMuted
	}

	treeLine(c, fmt.Sprintf("col-%d", idx), nil,
		func() {
			en := ui.ButtonBase(c.Key("en")).Size(16, 16).Radius(4).Tooltip("启用/禁用")
			en.Children(func() { ui.Icon(c, iconFile).Size(13, 13).TextColor(iconColor) })
			if en.Hovered() {
				en.Background(t.SurfaceHover)
			}
			if en.Clicked() {
				step.Enabled = !step.Enabled
			}
		},
		func() {
			btn := ui.ButtonBase(c.Key("name")).Grow(1).Padding(3, 0).Radius(4).
				Justify(ui.Start).AlignItems(ui.Center).Tooltip("单击打开请求标签；右键更多操作")
			if selected {
				btn.Background(t.Accent.Mix(t.Surface, 0.86))
			} else if btn.Hovered() {
				btn.Background(t.SurfaceHover)
			}
			btn.Children(func() {
				nb := ui.Text(c, name).FontSize(11)
				if selected {
					nb.Bold()
				}
				nb.TextColor(nameColor)
			})
			if btn.Clicked() {
				a.selectStep(idx)
				a.openStepTab(idx)
			}
			if btn.DoubleClicked() {
				a.openStepDebug(idx)
			}
			// 右键菜单绑定到该行。
			a.attachContextMenu(c, fmt.Sprintf("col-%d", idx), btn, "collection", idx)
		}, nil)
}

// ── Flows：请求流列表（场景工作区） ──

func (a *App) flowsView(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Padding(4, 4, 8, 4).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "Flows").FontSize(11).Bold()
		ui.Text(c, fmt.Sprintf("%d 个场景", len(a.workspace.files))).FontSize(10).TextColor(t.TextMuted)
	})
	// 当前场景作为一个 Flow 节点。
	treeLine(c, "flow-current", nil,
		func() { ui.Icon(c, iconFile).Size(13, 13).TextColor(brandOrange) },
		func() {
			btn := ui.ButtonBase(c.Key("name")).Grow(1).Padding(3, 0).Radius(4).
				Justify(ui.Start).AlignItems(ui.Center).Tooltip("当前流程视图")
			btn.Background(t.Accent.Mix(t.Surface, 0.9))
			btn.Children(func() {
				ui.Text(c, "▶ "+a.scenario.Name).FontSize(11).Bold().TextColor(t.Accent).SingleLine()
			})
			if btn.Clicked() {
				a.openFlowTab()
			}
			a.attachContextMenu(c, "flow-current", btn, "flow", -1)
		}, nil)
	ui.Box(c).Height(6)
	// 工作区内其它场景。
	for _, path := range a.workspace.files {
		if path == a.filePath {
			continue
		}
		p := path
		treeLine(c, "flow-"+p, nil,
			func() { ui.Icon(c, iconFile).Size(13, 13).TextColor(t.TextMuted) },
			func() {
				btn := ui.ButtonBase(c.Key("name")).Grow(1).Padding(3, 0).Radius(4).
					Justify(ui.Start).AlignItems(ui.Center).Tooltip("打开流程：" + p)
				if btn.Hovered() {
					btn.Background(t.SurfaceHover)
				}
				btn.Children(func() {
					ui.Text(c, scenarioNameFromFile(p)).FontSize(11).SingleLine()
				})
				if btn.Clicked() {
					a.switchWorkspaceScene(p)
					a.openFlowTab()
				}
			}, nil)
	}
	treeLine(c, "flow-new", nil, nil, func() {
		add := ui.ButtonBase(c.Key("btn")).Grow(1).Padding(3, 0).Radius(4).
			Justify(ui.Start).AlignItems(ui.Center)
		if add.Hovered() {
			add.Background(t.SurfaceHover)
		}
		add.Children(func() { ui.Text(c, "＋ 新建流程").FontSize(11).TextColor(t.TextMuted) })
		if add.Clicked() {
			a.openWorkspaceNew()
		}
	}, nil)
}

// ── History：历史记录 ──

func (a *App) historyView(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Padding(4, 4, 8, 4).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "History").FontSize(11).Bold()
		ui.Spacer(c)
		if treeTextButton(c, t, "hist-refresh", "↻", "刷新", t.TextMuted, 22).Clicked() {
			a.openHistory()
		}
	})
	if len(a.historyList) == 0 {
		ui.Text(c, "暂无历史，发送一次请求后自动记录。").FontSize(10).TextColor(t.TextMuted).Padding(6)
		return
	}
	for i, e := range a.historyList {
		ent := e
		treeLine(c, fmt.Sprintf("hist-%d", i), nil,
			func() { ui.Box(c).Width(5).Height(18).Radius(2).Background(methodColor(ent.Method)) },
			func() {
				btn := ui.ButtonBase(c.Key("name")).Grow(1).Padding(3, 0).Radius(4).
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

// ── 通用右键菜单 ──

// attachContextMenu 给目标元素绑定右键菜单（每行独立 open 状态）。
func (a *App) attachContextMenu(c *ui.Context, key string, target ui.Element, kind string, idx int) {
	if a.ctxMenuOpens == nil {
		a.ctxMenuOpens = map[string]*bool{}
	}
	open, ok := a.ctxMenuOpens[key]
	if !ok {
		open = new(bool)
		a.ctxMenuOpens[key] = open
	}
	m := ui.ContextMenuBase(c, target, open)
	m.Popup(func(panel ui.Element) {
		ui.Column(c).Padding(4).Gap(2).Background(c.Theme().Surface).Radius(6).Children(func() {
			switch kind {
			case "collection":
				if ui.Button(c, "＋ 新建请求").Clicked() {
					a.appendStep(model.StepRequest)
					*open = false
				}
				if ui.Button(c, "✎ 重命名").Clicked() {
					a.selectedStep = idx
					a.openNameEdit()
					*open = false
				}
				if ui.Button(c, "⧉ 复制").Clicked() {
					a.duplicateStep(idx)
					*open = false
				}
				if ui.Button(c, "🗑 删除").Clicked() {
					a.deleteStep(idx)
					*open = false
				}
			case "flow":
				if ui.Button(c, "＋ 新建流程").Clicked() {
					a.openWorkspaceNew()
					*open = false
				}
				if ui.Button(c, "✎ 重命名场景").Clicked() {
					a.openNameEdit()
					*open = false
				}
			}
		})
	})
}

// sidebarContextMenu 侧栏空白区右键：新建请求（占位，确保全局右键可用）。
func (a *App) sidebarContextMenu(c *ui.Context, t *ui.Theme) {
	// 菜单项已绑定到各行 target，这里无需额外内容；保留函数供后续扩展。
}
