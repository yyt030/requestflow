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
			// 类型色文件图标本身兼作启停开关：彩色=启用、淡化=禁用，点击切换。
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
			treeLine(c, fmt.Sprintf("step-%d", idx), nil,
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
						step.Enabled = !step.Enabled
					}
				},
				func() {
					// 步骤名：透明底，hover 浅底，选中高亮。
					row := ui.ButtonBase(c.Key("name")).Grow(1).Padding(3, 0).Radius(4).
						Justify(ui.Start).AlignItems(ui.Center).
						Tooltip("点击选中并配置；双击 HTTP 请求进入调试").Cursor(ui.CursorPointer)
					if selected {
						row.Background(t.Accent.Mix(t.Surface, 0.86))
					} else if row.Hovered() {
						row.Background(t.SurfaceHover)
					}
					row.Children(func() {
						nb := ui.Text(c, name).FontSize(11)
						if selected {
							nb.Bold()
						}
						nb.TextColor(nameColor)
					})
					if row.Clicked() {
						a.selectStep(idx)
					}
					if row.DoubleClicked() {
						a.selectStep(idx)
						a.openStepDebug(idx)
					}
				},
				func() {
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
	// 文件树底部：新建步骤入口（展开类型菜单）。
	label := "＋ 新建步骤"
	if a.adding {
		label = "− 收起"
	}
	treeLine(c, "add-step", nil, nil, func() {
		addRow := ui.ButtonBase(c.Key("btn")).Grow(1).Padding(3, 0).Radius(4).
			Justify(ui.Start).AlignItems(ui.Center).TextColor(t.TextMuted).Cursor(ui.CursorPointer)
		if addRow.Hovered() {
			addRow.Background(t.SurfaceHover)
		}
		addRow.Children(func() {
			ui.Text(c, label).FontSize(11).TextColor(t.TextMuted)
		})
		if addRow.Clicked() {
			a.adding = !a.adding
		}
	}, nil)
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
			a.editorEmpty(c, t)
			return
		}
		step := &a.scenario.Steps[a.selectedStep]

		// 顶部固定区：步骤概览常驻，切标签时不跟随滚动（接口调试风格）。
		// request 步骤带醒目的【发送】按钮，直接调试当前步骤配置。
		isReq := step.Type == model.StepRequest
		ui.Column(c).Gap(8).Padding(12, 14, 6, 14).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "步骤配置").FontSize(14).Bold()
				ui.Text(c, string(step.Type)).FontSize(11).TextColor(t.Accent)
				ui.Spacer(c)
				if isReq {
					if ui.PrimaryButton(c, "▶ 发送").Disabled(a.debugSending).Tooltip("用当前步骤配置发送请求，响应显示在下方").Clicked() {
						a.debugStep(step)
					}
				}
				ui.Checkbox(c, &step.Enabled, "启用")
				ui.Checkbox(c, &step.ContinueOnFail, "失败后继续")
			})
			ui.TextInput(c, &step.Name).Placeholder("给步骤起个名字，便于识别").Label("步骤名称").Grow(1)
		})

		// 配置子标签：HTTP 请求步骤像接口调试一样细粒度拆分，其余步骤单标签承载。
		// 内容相对上次确认被改过的标签，名字后带绿色小圆点，便于快速定位改动。
		tabs := stepTabNames(step)
		tab := clampStepTab(a, step)
		ui.Row(c).Padding(0, 14).AlignItems(ui.Center).Children(func() {
			dirtyCount := a.tabBarWithDots(c, t, step)
			ui.Spacer(c)
			if dirtyCount > 0 {
				ui.Text(c, "已修改").FontSize(10).TextColor(t.Success)
			}
			if isReq {
				ui.Text(c, "支持 {{变量}} · 下方为调试响应").FontSize(10).TextColor(t.TextMuted)
			}
		})
		ui.Divider(c)

		// 内容区：request 步骤用「配置 / 响应」上下分屏，编排中即可直接调试；
		// 其余步骤保持单一滚动配置区。底部加大留白，避免末行被状态栏截断。
		if isReq {
			ui.SplitVertical(c, &a.splitEditorResp, func() {
				ui.Scroll(c).Grow(1).Padding(12, 14, 24, 14).Gap(10).Children(func() {
					a.stepTabView(c, t, step, tabs[tab])
				})
			}, func() {
				a.editorResponseArea(c, t)
			}).Fill()
		} else {
			ui.Scroll(c).Grow(1).Padding(12, 14, 40, 14).Gap(10).Children(func() {
				a.stepTabView(c, t, step, tabs[tab])
			})
		}
	})
}

// debugPanel 接口调试面板：上半请求配置，下半响应展示。
