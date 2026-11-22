package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

// editorEmpty 未选中步骤时的引导页：把「选择一个步骤」的单薄提示，
// 升级为「场景概览 + 一键新建常用步骤 + 操作指引」，让主工作区不再大片空白。
func (a *App) editorEmpty(c *ui.Context, t *ui.Theme) {
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Width(560).MaxWidth(560).Padding(20, 8).Gap(14).Children(func() {
			// 欢迎区：图标 + 标题 + 说明，整体左对齐（与下方卡片同一竖线）。
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Width(46).Height(46).Radius(12).Background(t.Accent.Mix(t.Surface, 0.86)).Center().Children(func() {
					ui.Icon(c, iconFolder).Size(26, 26).TextColor(t.Accent)
				})
				ui.Column(c).Gap(3).Children(func() {
					ui.Text(c, "开始编排你的请求流程").FontSize(16).Bold()
					ui.Text(c, "选中左侧步骤即可在此配置；或点下方快捷入口新建一步。").FontSize(11).TextColor(t.TextMuted)
				})
			})

			// 当前场景概览：三步数据（步骤 / 变量 / Base URL）等宽排列。
			a.emptyOverview(c, t)

			// 快捷入口：常用步骤卡片，点击即新建并选中，直接可配置。
			ui.Column(c).Gap(8).Children(func() {
				ui.Text(c, "快捷新建").FontSize(12).Bold()
				fieldHint(c, t, "点击任意卡片新建一个步骤，并立即在右侧打开配置。")
				ui.Row(c).Gap(10).Children(func() {
					emptyStepCard(c, t, "http", iconSend, "HTTP 请求", "发送 HTTP / HTTPS 请求", stepColors[model.StepRequest], func() {
						a.appendStepAndSelect(model.StepRequest)
					})
					emptyStepCard(c, t, "loop", iconFolder, "循环 / 翻页", "重复执行子步骤翻页", stepColors[model.StepLoop], func() {
						a.appendStepAndSelect(model.StepLoop)
					})
				})
				ui.Row(c).Gap(10).Children(func() {
					emptyStepCard(c, t, "process", iconFile, "数据处理", "提取 / 组装 / 整形结果", stepColors[model.StepProcess], func() {
						a.appendStepAndSelect(model.StepProcess)
					})
					emptyStepCard(c, t, "script", iconNew, "自定义脚本", "用 JS / Python 扩展流程", stepColors[model.StepScript], func() {
						a.appendStepAndSelect(model.StepScript)
					})
				})
			})

			// 操作指引：三行关键交互说明。
			ui.Column(c).Gap(6).Background(t.Surface).Radius(8).Padding(12).Children(func() {
				ui.Text(c, "上手提示").FontSize(12).Bold()
				emptyTipRow(c, t, "1", "左侧文件树点选步骤配置，双击 HTTP 步骤进入接口调试。")
				emptyTipRow(c, t, "2", "URL 与字段支持 {{变量名}}，点输入框旁 ⊕变量 一键插入。")
				emptyTipRow(c, t, "3", "顶部「▶ 运行」一键执行整个场景，结果在「运行结果」标签查看。")
			})
		})
	})
}

// emptyOverview 场景概览：步骤数 / 变量数 / Base URL 三个等宽信息块。
func (a *App) emptyOverview(c *ui.Context, t *ui.Theme) {
	varCount := len(a.scenario.Variables)
	for _, vars := range a.scenario.Environments {
		varCount += len(vars)
	}
	base := a.scenario.BaseURL
	if base == "" {
		base = "（未设置）"
	}
	ui.Row(c).Gap(10).Children(func() {
		emptyStatBox(c, t, "步骤数", fmt.Sprintf("%d", len(a.scenario.Steps)))
		emptyStatBox(c, t, "变量数", fmt.Sprintf("%d", varCount))
		emptyStatBox(c, t, "Base URL", truncateRunes(base, 20))
	})
}

// emptyStatBox 单个概览信息块：上标签、下数值，等宽。
func emptyStatBox(c *ui.Context, t *ui.Theme, label, value string) {
	ui.Column(c).Grow(1).Gap(3).Background(t.Surface).Radius(8).Padding(10, 12).Children(func() {
		ui.Text(c, label).FontSize(10).TextColor(t.TextMuted)
		ui.Text(c, value).FontSize(13).Bold().SingleLine()
	})
}

// emptyStepCard 快捷新建卡片：图标 + 标题 + 说明，点击触发回调。
func emptyStepCard(c *ui.Context, t *ui.Theme, key string, icon *ui.SVG, title, desc string, col ui.Color, onClick func()) {
	card := ui.ButtonBase(c.Key(key)).Grow(1).Radius(8).Padding(10).
		Justify(ui.Start).AlignItems(ui.Center).Cursor(ui.CursorPointer)
	if card.Hovered() {
		card.Background(t.SurfaceHover)
	}
	card.Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icon).Size(20, 20).TextColor(col)
			ui.Column(c).Gap(2).Children(func() {
				ui.Text(c, title).FontSize(12).Bold()
				ui.Text(c, desc).FontSize(10).TextColor(t.TextMuted)
			})
		})
	})
	if card.Clicked() {
		onClick()
	}
}

// emptyTipRow 一行操作指引：序号徽章 + 说明文字。
func emptyTipRow(c *ui.Context, t *ui.Theme, num, text string) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Width(18).Height(18).Radius(9).Background(t.Accent.Mix(t.Surface, 0.86)).Center().Children(func() {
			ui.Text(c, num).FontSize(10).Bold().TextColor(t.Accent)
		})
		ui.Text(c, text).FontSize(11)
	})
}

// appendStepAndSelect 新建步骤并立即选中，供空状态快捷入口复用。
func (a *App) appendStepAndSelect(typ model.StepType) {
	a.appendStep(typ)
	a.selectedStep = len(a.scenario.Steps) - 1
}
