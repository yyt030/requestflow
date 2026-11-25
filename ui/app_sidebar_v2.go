package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

// ── 统一侧栏树 ──
// 树里只有场景（文件夹节点）与它们的步骤——请求字段内联在步骤中，步骤即请求。
// 点步骤即在主区打开该步骤的完整编辑器（请求字段 + 提取/断言）。

// unifiedTree 从根开始渲染整棵侧栏树：场景 → 步骤，底部折叠 History。
func (a *App) unifiedTree(c *ui.Context, t *ui.Theme) {
	a.scenesTreeFolder(c, t)
	ui.Box(c).Height(8)
	a.historyTreeFolder(c, t)
}

// treeFlag 读取分组展开态；未记录时返回默认值（当前场景默认展开）。
func (a *App) treeFlag(m map[string]bool, key string, def bool) bool {
	if v, ok := m[key]; ok {
		return v
	}
	return def
}

// sceneSteps 返回某个场景的步骤摘要：当前场景直接用内存模型，其它场景按需读盘并缓存。
func (a *App) sceneSteps(path string) []model.Step {
	if path == a.filePath || (path == "" && a.filePath == "") {
		return a.scenario.Steps
	}
	if a.sceneStepCache == nil {
		a.sceneStepCache = map[string][]model.Step{}
	}
	if steps, ok := a.sceneStepCache[path]; ok {
		return steps
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var sc model.Scenario
	if json.Unmarshal(data, &sc) != nil {
		return nil
	}
	a.sceneStepCache[path] = sc.Steps
	return sc.Steps
}

// scenesTreeFolder 工作区每个场景直接作为顶层文件夹节点（不再套 Flows 分组），
// 展开后显示该场景的步骤。
func (a *App) scenesTreeFolder(c *ui.Context, t *ui.Theme) {
	paths := a.workspace.files
	if len(paths) == 0 {
		paths = []string{a.filePath} // 无工作区时显示当前（demo）场景
	}
	for _, path := range paths {
		a.sceneTreeFolder(c, t, path)
	}
	// 新建场景入口，紧跟在场景列表之后。
	treeLine(c, "tree-new-scene", nil, nil, func() {
		add := ui.ButtonBase(c.Key("btn")).Grow(1).Padding(2, 0).Radius(4).
			Justify(ui.Start).AlignItems(ui.Center).Cursor(ui.CursorPointer)
		if add.Hovered() {
			add.Background(t.SurfaceHover)
		}
		add.Children(func() {
			ui.Text(c, "＋ 新建场景").FontSize(11).TextColor(t.TextMuted)
		})
		if add.Clicked() {
			a.openWorkspaceNew()
		}
	}, nil)
}

// sceneTreeFolder 单个场景文件夹节点：点箭头展开步骤，点名称切换并打开流程。
func (a *App) sceneTreeFolder(c *ui.Context, t *ui.Theme, path string) {
	if a.sceneExpanded == nil {
		a.sceneExpanded = map[string]bool{}
	}
	isCurrent := path == a.filePath || (path == "" && a.filePath == "")
	name := scenarioNameFromFile(path)
	if isCurrent {
		name = a.scenario.Name
	}
	key := path
	if key == "" {
		key = "current-demo"
	}
	open := a.treeFlag(a.sceneExpanded, key, isCurrent)
	steps := a.sceneSteps(path)
	a.treeFolderRow(c, t, "tree-scene-"+key, &open, iconFile, brandOrange, func() {
		nb := ui.Text(c, name).FontSize(11).SingleLine()
		if isCurrent {
			nb.Bold().TextColor(t.Accent)
		}
		ui.Text(c, fmt.Sprintf("(%d)", len(steps))).FontSize(10).TextColor(t.TextMuted)
	}, func() {
		btn := ui.ButtonBase(c.Key("open")).Padding(2, 4).Radius(4).Tooltip("打开流程视图")
		if btn.Hovered() {
			btn.Background(t.SurfaceHover)
		}
		btn.Children(func() { ui.Text(c, "↗").FontSize(11).TextColor(t.TextMuted) })
		if btn.Clicked() {
			if !isCurrent {
				a.switchWorkspaceScene(path)
			}
			a.openFlowTab()
		}
	})
	a.sceneExpanded[key] = open
	if !open {
		return
	}
	q := strings.ToLower(strings.TrimSpace(a.stepSearch))
	shown := 0
	for i := range steps {
		step := &steps[i]
		if q != "" && !strings.Contains(strings.ToLower(step.Name), q) &&
			!strings.Contains(strings.ToLower(string(step.Type)), q) {
			continue
		}
		a.sceneStepRow(c, t, path, i, step, isCurrent)
		shown++
	}
	if shown == 0 {
		text := "  （空场景，点下方 ＋ 新建请求）"
		if q != "" {
			text = "  没有匹配的步骤"
		}
		ui.Text(c, text).FontSize(10).TextColor(t.TextMuted).Padding(4)
	}
	// 新建请求入口：在当前场景末尾追加一个内联请求步骤。
	treeLine(c, "tree-new-req-"+key, nil, nil, func() {
		add := ui.ButtonBase(c.Key("btn")).Grow(1).Padding(2, 0).Radius(4).
			Justify(ui.Start).AlignItems(ui.Center).Cursor(ui.CursorPointer)
		if add.Hovered() {
			add.Background(t.SurfaceHover)
		}
		add.Children(func() {
			label := "＋ 新建请求"
			if !isCurrent {
				label = "＋ 新建请求（先切换到此场景）"
			}
			ui.Text(c, label).FontSize(11).TextColor(t.TextMuted)
		})
		if add.Clicked() {
			if !isCurrent {
				a.switchWorkspaceScene(path)
			}
			a.addRequestStepToCurrentScene()
		}
	}, nil)
}

// addRequestStepToCurrentScene 在当前场景末尾新建一个纯内联的 HTTP 步骤并打开编辑。
// 步骤自带全部请求字段，是唯一真相源；直接编辑、直接发送。
func (a *App) addRequestStepToCurrentScene() {
	step := model.Step{
		ID:      newID(),
		Name:    "新建请求",
		Type:    model.StepRequest,
		Enabled: true,
		Method:  "GET",
		Extract: []model.Extract{},
		Assert:  []model.Assert{},
	}
	a.scenario.Steps = append(a.scenario.Steps, step)
	a.selectedStep = len(a.scenario.Steps) - 1
	if a.filePath != "" {
		a.saveTo(a.filePath)
	}
	a.openStepTab(a.selectedStep)
}

// sceneStepRow 场景下的单个步骤节点：点击选中并打开配置，双击 HTTP 步骤进入调试。
// 非当前场景的步骤在被操作前会先切换到该场景，保证后续编辑落在正确场景上。
func (a *App) sceneStepRow(c *ui.Context, t *ui.Theme, path string, idx int, step *model.Step, isCurrent bool) {
	selected := isCurrent && idx == a.selectedStep
	sc := stepColors[step.Type]
	iconColor := sc
	enableTip := "点击禁用此步骤"
	if !step.Enabled {
		iconColor = t.TextMuted
		enableTip = "点击启用此步骤"
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
	treeLine(c, fmt.Sprintf("tree-step-%s-%d", path, idx), nil,
		func() {
			fileIconBtn := ui.ButtonBase(c.Key("en")).Size(16, 16).Radius(4).
				Tooltip(enableTip).Cursor(ui.CursorPointer)
			fileIconBtn.Children(func() {
				ui.Icon(c, iconFile).Size(13, 13).TextColor(iconColor)
			})
			if fileIconBtn.Hovered() {
				fileIconBtn.Background(t.SurfaceHover)
			}
			if fileIconBtn.Clicked() {
				if !isCurrent {
					a.switchWorkspaceScene(path)
				}
				a.scenario.Steps[idx].Enabled = !a.scenario.Steps[idx].Enabled
			}
		},
		func() {
			row := ui.ButtonBase(c.Key("name")).Grow(1).Padding(2, 0).Radius(4).
				Justify(ui.Start).AlignItems(ui.Center).Cursor(ui.CursorPointer).
				Tooltip("点击选中并配置；双击 HTTP 请求进入调试")
			if selected {
				row.Background(t.Accent.Mix(t.Surface, 0.86))
			} else if row.Hovered() {
				row.Background(t.SurfaceHover)
			}
			row.Children(func() {
				nb := ui.Text(c, name).FontSize(11).SingleLine()
				if selected {
					nb.Bold()
				}
				nb.TextColor(nameColor)
			})
			if row.Clicked() {
				if !isCurrent {
					a.switchWorkspaceScene(path)
				}
				// 方案 C：点步骤即在主区打开该步骤的完整编辑标签（请求 + 流程一屏）。
				a.selectStep(idx)
				a.openStepTab(idx)
			}
			if row.DoubleClicked() {
				if !isCurrent {
					a.switchWorkspaceScene(path)
				}
				a.selectStep(idx)
				a.openStepTab(idx)
				a.openStepDebug(idx)
			}
		}, nil)
}
