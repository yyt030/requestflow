package ui

import (
	"encoding/json"
	"fmt"

	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

// mainArea 右侧主工作区：多标签 + 内容视图 + 内嵌状态栏。
// 标签支持：点击激活、关闭、左右排序、新建请求标签；可拖动排序（拖拽手柄）。
func (a *App) mainArea(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Children(func() {
		a.workTabBar(c, t)
		ui.Divider(c)
		// 视图区 Grow(1)。
		ui.Column(c).Grow(1).Children(func() {
			if a.mainTab == 1 {
				a.consoleOnly(c, t)
			} else if a.activeWork >= 0 && a.activeWork < len(a.workTabs) {
				tab := a.workTabs[a.activeWork]
				switch tab.kind {
				case "result":
					a.consoleOnly(c, t)
				case "flow":
					a.flowEditor(c, t) // 流程视图：线性步骤编排（列表 + 步骤配置）
				default:
					a.editor(c, t) // 步骤请求：配置 + 响应上下分屏
				}
			} else {
				a.editor(c, t)
			}
		})
		// 状态栏。
		a.statusBar(c, t)
	})
}

// workTabBar 多标签栏：每个标签可激活/关闭/排序，末尾"＋"新建请求标签。
// 不再默认塞入固定的"步骤编排"标签——主区默认显示空态引导，
// 用户从侧栏点击步骤/流程或点"＋"才打开对应标签（语义更清晰、无冗余标签）。
func (a *App) workTabBar(c *ui.Context, t *ui.Theme) {
	if len(a.workTabs) > 0 && (a.activeWork < 0 || a.activeWork >= len(a.workTabs)) {
		a.activeWork = len(a.workTabs) - 1
	}

	ui.Row(c).Padding(6, 8, 4, 8).Gap(4).AlignItems(ui.Center).Children(func() {
		for i := range a.workTabs {
			i := i
			tab := &a.workTabs[i]
			active := i == a.activeWork
			chip := ui.ButtonBase(c.Key(tab.id)).Padding(5, 8).Radius(6).
				AlignItems(ui.Center).Gap(6).Cursor(ui.CursorPointer)
			if active {
				chip.Background(t.Accent.Mix(t.Surface, 0.86))
			} else if chip.Hovered() {
				chip.Background(t.SurfaceHover)
			}
			chip.Children(func() {
				// 拖拽手柄（点击左移/右移实现排序，视觉上等价拖动排序）。
				if treeTextButton(c, t, tab.id+"-l", "‹", "左移标签", t.TextMuted, 14).Clicked() {
					a.moveWorkTab(i, -1)
				}
				col := t.Text
				if active {
					col = t.Accent
				}
				ui.Text(c, tab.label).FontSize(11).Bold().TextColor(col).SingleLine()
				if treeTextButton(c, t, tab.id+"-r", "›", "右移标签", t.TextMuted, 14).Clicked() {
					a.moveWorkTab(i, 1)
				}
				if treeTextButton(c, t, tab.id+"-x", "✕", "关闭标签", t.TextMuted, 14).Clicked() {
					a.closeWorkTab(i)
				}
			})
			if chip.Clicked() {
				a.activeWork = i
				if tab.kind == "step" && tab.step >= 0 && tab.step < len(a.scenario.Steps) {
					a.selectedStep = tab.step
				}
				a.mainTab = 0
			}
		}
		// 新建请求标签。
		if treeTextButton(c, t, "tab-add", "＋", "新建请求标签", t.TextMuted, 22).Clicked() {
			a.newRequestTab()
		}
		ui.Spacer(c)
		// 运行结果标签入口。
		if ui.Button(c, "运行结果").Clicked() {
			a.mainTab = 1
		}
	})
}

// moveWorkTab 左/右移动标签实现排序。
func (a *App) moveWorkTab(i, dir int) {
	j := i + dir
	if i < 0 || j < 0 || i >= len(a.workTabs) || j >= len(a.workTabs) {
		return
	}
	a.workTabs[i], a.workTabs[j] = a.workTabs[j], a.workTabs[i]
	a.activeWork = j
}

// closeWorkTab 关闭标签（至少保留一个）。
func (a *App) closeWorkTab(i int) {
	if i < 0 || i >= len(a.workTabs) || len(a.workTabs) <= 1 {
		return
	}
	a.workTabs = append(a.workTabs[:i], a.workTabs[i+1:]...)
	if a.activeWork >= len(a.workTabs) {
		a.activeWork = len(a.workTabs) - 1
	}
}

// newRequestTab 新建一个请求编辑标签（选中当前步骤或新建空请求步骤）。
func (a *App) newRequestTab() {
	step := a.selectedStep
	if step < 0 || step >= len(a.scenario.Steps) {
		a.appendStep(model.StepRequest)
		step = a.selectedStep
	}
	label := a.scenario.Steps[step].Name
	if label == "" {
		label = "请求 " + fmt.Sprintf("%d", len(a.workTabs)+1)
	}
	a.workTabs = append(a.workTabs, a.newWorkTab(label, "step", step))
	a.activeWork = len(a.workTabs) - 1
	a.mainTab = 0
}

// openStepTab 单击集合请求：选中步骤并在中央打开/激活其请求标签。
func (a *App) openStepTab(idx int) {
	for i, tab := range a.workTabs {
		if tab.kind == "step" && tab.step == idx {
			a.activeWork = i
			a.mainTab = 0
			return
		}
	}
	name := a.scenario.Steps[idx].Name
	if name == "" {
		name = string(a.scenario.Steps[idx].Type)
	}
	a.workTabs = append(a.workTabs, a.newWorkTab(name, "step", idx))
	a.activeWork = len(a.workTabs) - 1
	a.mainTab = 0
}

// openFlowTab 打开流程视图标签（场景步骤编排）。
func (a *App) openFlowTab() {
	for i, tab := range a.workTabs {
		if tab.kind == "flow" {
			a.activeWork = i
			a.mainTab = 0
			return
		}
	}
	a.workTabs = append(a.workTabs, a.newWorkTab("流程·"+a.scenario.Name, "flow", -1))
	a.activeWork = len(a.workTabs) - 1
	a.mainTab = 0
}

// duplicateStep 复制一个步骤（插入到其后）。
func (a *App) duplicateStep(idx int) {
	if idx < 0 || idx >= len(a.scenario.Steps) {
		return
	}
	src := a.scenario.Steps[idx]
	cp := cloneStep(src)
	cp.ID = newID()
	cp.Name = src.Name + " 副本"
	a.scenario.Steps = append(a.scenario.Steps, model.Step{})
	copy(a.scenario.Steps[idx+2:], a.scenario.Steps[idx+1:])
	a.scenario.Steps[idx+1] = cp
	a.selectedStep = idx + 1
}

// cloneStep 通过 JSON 往返深拷贝步骤，避免共享 map/slice 引用。
func cloneStep(s model.Step) model.Step {
	b, err := json.Marshal(s)
	if err != nil {
		return s
	}
	var out model.Step
	if json.Unmarshal(b, &out) != nil {
		return s
	}
	return out
}
