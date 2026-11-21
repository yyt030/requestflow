package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

func (a *App) stepList(c *ui.Context, t *ui.Theme) {
	steps := a.scenario.Steps
	q := strings.ToLower(strings.TrimSpace(a.stepSearch))
	// 搜索过滤：匹配步骤名或类型名，保持原始索引用于选择/移动。
	visible := make([]int, 0, len(steps))
	for i := range steps {
		if q == "" || strings.Contains(strings.ToLower(steps[i].Name), q) ||
			strings.Contains(strings.ToLower(string(steps[i].Type)), q) {
			visible = append(visible, i)
		}
	}
	if len(visible) == 0 {
		if q != "" {
			ui.Text(c, "没有匹配的步骤").FontSize(11).TextColor(t.TextMuted).Padding(8)
		} else {
			ui.Text(c, "暂无步骤，点击下方 + 新建。").FontSize(11).TextColor(t.TextMuted).Padding(8)
		}
	} else {
		for _, idx := range visible {
			step := &steps[idx]
			selected := idx == a.selectedStep
			sc := stepColors[step.Type]
			ui.Row(c.Key(fmt.Sprintf("step-%d", idx))).Gap(4).Padding(2, 6).Radius(6).
				AlignItems(ui.Center).Children(func() {
				// 文件树横折线 ├─，与上层 guide 竖线相接。
				treeBranch(c, t)
				// 类型色文件图标本身兼作启停开关：彩色=启用、淡化=禁用，点击切换。
				iconColor := sc
				enableTip := "点击禁用此步骤"
				if !step.Enabled {
					iconColor = t.TextMuted
					enableTip = "点击启用此步骤"
				}
				fileIconBtn := ui.ButtonBase(c.Key("en")).Size(16, 16).Radius(4).
					Tooltip(enableTip).Cursor(ui.CursorPointer)
				fileIconBtn.Children(func() {
					ui.Icon(c, iconFile).Size(13, 13).TextColor(iconColor)
				})
				if fileIconBtn.Hovered() {
					fileIconBtn.Background(t.SurfaceHover)
				}
				if fileIconBtn.Clicked() {
					step.Enabled = !step.Enabled
				}
				name := step.Name
				if name == "" {
					name = string(step.Type)
				}
				b := ui.Button(c, name).Grow(1).
					Tooltip("点击选中并配置；双击 HTTP 请求进入调试")
				if selected {
					b = b.Background(t.Accent.Mix(t.Surface, 0.86)).TextColor(t.Accent).Bold()
				}
				if !step.Enabled {
					b = b.TextColor(t.TextMuted)
				}
				if b.Clicked() {
					a.selectStep(idx)
				}
				if b.DoubleClicked() {
					a.selectStep(idx)
					a.openStepDebug(idx)
				}
				if stepToolButton(c, t, "up", iconMoveUp, "上移步骤", idx == 0, false).Clicked() {
					a.moveStep(idx, -1)
				}
				if stepToolButton(c, t, "dn", iconMoveDown, "下移步骤", idx == len(steps)-1, false).Clicked() {
					a.moveStep(idx, 1)
				}
				if stepToolButton(c, t, "del", iconDelete, "删除步骤", false, true).Clicked() {
					a.deleteStep(idx)
				}
			})
		}
	}
	// 文件树底部：新建步骤入口（展开类型菜单），同样带横折线。
	ui.Row(c).Gap(4).Padding(2, 6).AlignItems(ui.Center).Children(func() {
		treeBranch(c, t)
		label := "＋ 新建步骤"
		if a.adding {
			label = "− 收起"
		}
		if ui.Button(c, label).Grow(1).TextColor(t.TextMuted).Clicked() {
			a.adding = !a.adding
		}
	})
	if a.adding {
		a.addMenu(c, t)
	}
}

func (a *App) stepLabel(s model.Step) string {
	icon := map[model.StepType]string{
		model.StepRequest: "⇄", model.StepTCP: "TCP", model.StepLoop: "↻", model.StepBranch: "分", model.StepSwitch: "选",
		model.StepParallel: "扇出", model.StepMerge: "扇入", model.StepProcess: "⚙",
		model.StepScript: "JS", model.StepAssert: "✓", model.StepExport: "⇩",
	}[s.Type]
	name := s.Name
	if name == "" {
		name = string(s.Type)
	}
	return fmt.Sprintf("%s  %s", icon, name)
}

// ---------- 中间：步骤编辑器 ----------

// ---------- 中间：步骤编辑器 ----------
func (a *App) editor(c *ui.Context, t *ui.Theme) {
	// cURL / API 定义导入模式：覆盖编辑器区域
	if a.showImport {
		a.importPanel(c, t)
		return
	}
	// 请求历史模式
	if a.showHistory {
		a.historyPanel(c, t)
		return
	}
	ui.Column(c).Fill().Children(func() {
		if a.selectedStep < 0 || a.selectedStep >= len(a.scenario.Steps) {
			emptyState(c, t, "选择一个步骤进行配置",
				"单击左侧步骤查看配置，双击请求步骤进入调试；内置登录提取、HTTP、循环翻页、数据处理、脚本、断言、导出")
			return
		}
		step := &a.scenario.Steps[a.selectedStep]
		ui.Scroll(c).Grow(1).Padding(14).Gap(10).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "步骤配置").FontSize(14).Bold()
				ui.Text(c, string(step.Type)).FontSize(11).TextColor(t.Accent)
				ui.Spacer(c)
				ui.Checkbox(c, &step.Enabled, "启用")
				ui.Checkbox(c, &step.ContinueOnFail, "失败后继续")
			})
			ui.TextInput(c, &step.Name).Placeholder("给步骤起个名字，便于识别").Label("步骤名称")
			ui.Divider(c)
			switch step.Type {
			case model.StepRequest:
				a.editorRequest(c, t, step)
			case model.StepTCP:
				a.editorTCP(c, t, step)
			case model.StepLoop:
				a.editorLoop(c, t, step)
			case model.StepBranch:
				a.editorBranch(c, t, step)
			case model.StepSwitch:
				a.editorSwitch(c, t, step)
			case model.StepParallel:
				a.editorParallel(c, t, step)
			case model.StepMerge:
				a.editorMerge(c, t, step)
			case model.StepProcess:
				a.editorProcess(c, t, step)
			case model.StepScript:
				a.editorScript(c, t, step)
			case model.StepAssert:
				a.editorAssert(c, t, step)
			case model.StepExport:
				a.editorExport(c, t, step)
			}
		})
	})
}

// debugPanel 接口调试面板：上半请求配置，下半响应展示。
