package ui

import (
	"fmt"
	"sort"

	"github.com/egoist/mygo/ui"
)

// sidebarModals 挂载侧边栏的三个编辑弹窗（场景名 / 环境 / 变量）。
// 独立成方法以避免 app.go 触及单文件 300 行门禁。
func (a *App) sidebarModals(c *ui.Context, t *ui.Theme) {
	ui.Modal(c, &a.nameEditOpen, func() { a.nameEditModal(c, t) })
	ui.Modal(c, &a.envEditOpen, func() { a.envEditModal(c, t) })
	ui.Modal(c, &a.varEditOpen, func() { a.varEditModal(c, t) })
	ui.Modal(c, &a.workspace.newOpen, func() { a.workspaceNewModal(c, t) })
}

// environmentEditor 环境变量分层编辑树：环境选择节点 + 全局变量节点 + 当前环境变量节点。
// 变量以紧凑树节点展示（图标 + key + 值摘要），点击节点弹出 Modal 编辑 key=value。
func (a *App) environmentEditor(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Gap(2).Children(func() {
		// 环境选择节点：点击弹出 Modal 切换/新建/删除环境，取代常驻 Select + 输入框。
		envLabel := "(无环境)"
		if a.scenario.ActiveEnv != "" {
			envLabel = a.scenario.ActiveEnv
		}
		varCount := 0
		if env := a.scenario.EnvVars(a.scenario.ActiveEnv); env != nil {
			varCount = len(env)
		}
		treeLine(c, "env-pick", nil,
			func() { ui.Text(c, "◈").FontSize(11).TextColor(t.Accent) },
			func() {
				envRow := ui.ButtonBase(c.Key("name")).Padding(2, 0).Radius(4).
					Justify(ui.Start).AlignItems(ui.Center).
					Tooltip("点击切换 / 新建 / 删除环境").Cursor(ui.CursorPointer)
				if envRow.Hovered() {
					envRow.Background(t.SurfaceHover)
				}
				envRow.Children(func() {
					ui.Text(c, envLabel).FontSize(11).TextColor(t.Text)
				})
				if envRow.Clicked() {
					a.openEnvEdit()
				}
				ui.Spacer(c)
				ui.Text(c, fmt.Sprintf("%d 项", varCount)).FontSize(10).TextColor(t.TextMuted)
			}, nil)

		// 全局变量节点组。
		a.varGroup(c, t, "全局变量", "global", a.scenario.Variables, "◇", t.TextMuted)
		// 当前环境变量节点组。
		if a.scenario.ActiveEnv != "" {
			a.varGroup(c, t, "环境 ["+a.scenario.ActiveEnv+"]", a.scenario.ActiveEnv,
				a.scenario.EnvVars(a.scenario.ActiveEnv), "◆", t.Accent)
		}
		// 可用变量速查：聚合全局 + 各步骤提取变量，提示可引用名。
		ui.Row(c).Padding(4, 6, 0, 6).Children(func() {
			fieldHint(c, t, varSummary(collectVarNames(a.scenario)))
		})
	})
}

// varGroup 渲染一组变量：分组标题行 + 若干变量节点 + "新建变量"节点。
// target 为 "global" 或环境名，标识该组变量归属（编辑时据此定位 map）。
func (a *App) varGroup(c *ui.Context, t *ui.Theme, title, target string, vars map[string]string, icon string, iconColor ui.Color) {
	// 分组标题：图标入图标槽，标题入文字区，与其他节点文字左对齐。
	treeLine(c, "group-"+target, nil,
		func() { ui.Text(c, icon).FontSize(11).TextColor(iconColor) },
		func() { ui.Text(c, title).FontSize(11).Bold().TextColor(t.TextMuted) }, nil)

	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		key := k
		summary := truncateRunes(vars[k], 28)
		treeLine(c, "var-"+target+"-"+key, nil,
			func() { ui.Text(c, "▸").FontSize(10).TextColor(t.TextMuted) },
			func() {
				nameRow := ui.ButtonBase(c.Key("name")).Width(120).Padding(2, 0).Radius(4).
					Justify(ui.Start).AlignItems(ui.Center).
					Tooltip("点击编辑 key=value").Cursor(ui.CursorPointer)
				if nameRow.Hovered() {
					nameRow.Background(t.SurfaceHover)
				}
				nameRow.Children(func() {
					ui.Text(c, key).FontSize(11).TextColor(t.Text)
				})
				if nameRow.Clicked() {
					a.openVarEdit(target, key)
				}
				ui.Text(c, "= "+summary).FontSize(10).TextColor(t.TextMuted).SingleLine().Grow(1)
			},
			func() {
				if treeTextButton(c, t, "del", "×", "删除变量", t.Danger, 20).Clicked() {
					a.deleteVar(target, key)
				}
			})
	}
	// 新建变量节点：透明文字行，与其他节点文字左对齐。
	treeLine(c, "varadd-"+target, nil, nil, func() {
		add := ui.ButtonBase(c.Key("add")).Grow(1).Padding(2, 0).Radius(4).
			Justify(ui.Start).AlignItems(ui.Center).Cursor(ui.CursorPointer)
		if add.Hovered() {
			add.Background(t.SurfaceHover)
		}
		add.Children(func() {
			ui.Text(c, "＋ 新建变量").FontSize(11).TextColor(t.TextMuted)
		})
		if add.Clicked() {
			a.openVarEdit(target, "")
		}
	}, nil)
}

// sceneNameNode 场景名树节点：普通文本展示，点击触发重命名 Modal。
func (a *App) sceneNameNode(c *ui.Context, t *ui.Theme) {
	name := a.scenario.Name
	if name == "" {
		name = "(未命名场景)"
	}
	nameRow := ui.ButtonBase(c.Key("scene-name")).Grow(1).Padding(2, 0).Radius(4).
		Justify(ui.Start).AlignItems(ui.Center).
		Tooltip("点击重命名场景").Cursor(ui.CursorPointer)
	if nameRow.Hovered() {
		nameRow.Background(t.SurfaceHover)
	}
	nameRow.Children(func() {
		ui.Text(c, name).FontSize(11).Bold().SingleLine()
	})
	if nameRow.Clicked() {
		a.openNameEdit()
	}
}

// ---------- 弹窗打开逻辑 ----------

func (a *App) openNameEdit() {
	a.nameEditBuf = a.scenario.Name
	a.nameEditOpen = true
}

func (a *App) openEnvEdit() {
	a.envEditBuf = a.scenario.ActiveEnv
	a.envEditOpen = true
}

// openVarEdit 打开变量编辑弹窗；origKey 为空表示新建。
func (a *App) openVarEdit(target, origKey string) {
	a.varEditTarget = target
	a.varEditOrigKey = origKey
	if origKey == "" {
		a.varEditKey, a.varEditValue = "", ""
	} else {
		a.varEditKey, a.varEditValue = origKey, a.varsFor(target)[origKey]
	}
	a.varEditOpen = true
}

// varsFor 按 target 返回可写的变量 map（"global" 或环境名）。
func (a *App) varsFor(target string) map[string]string {
	if target == "global" {
		if a.scenario.Variables == nil {
			a.scenario.Variables = map[string]string{}
		}
		return a.scenario.Variables
	}
	if a.scenario.Environments == nil {
		a.scenario.Environments = map[string]map[string]string{}
	}
	if a.scenario.Environments[target] == nil {
		a.scenario.Environments[target] = map[string]string{}
	}
	return a.scenario.Environments[target]
}

// deleteVar 直接删除指定变量。
func (a *App) deleteVar(target, key string) {
	delete(a.varsFor(target), key)
}
