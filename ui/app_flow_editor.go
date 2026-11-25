package ui

import (
	"github.com/egoist/mygo/ui"
)

// flowEditor 流程视图：线性步骤编排。
// 左侧场景的步骤列表，右侧当前步骤的流程配置（提取/断言/脚本）。
// 请求字段内联在步骤中，与普通步骤编辑器一致。
func (a *App) flowEditor(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Children(func() {
		// 顶部说明条：强调 Flow 做线性编排与步骤间数据传递。
		ui.Row(c).Padding(8, 14, 6, 14).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "流程编排").FontSize(13).Bold()
			ui.Text(c, "按序执行步骤 · 支持流程变量与步骤间提取").
				FontSize(10).TextColor(t.TextMuted)
			ui.Spacer(c)
			ui.Text(c, itoa(len(a.scenario.Steps))+" 个步骤").FontSize(10).TextColor(t.TextMuted)
		})
		ui.Divider(c)
		// 左：步骤列表；右：当前步骤配置（editor 内含请求引用行 + 提取/断言/脚本子标签）。
		ui.Split(c, &a.splitFlowSteps, func() {
			ui.Column(c).Fill().Padding(6, 4, 6, 4).Children(func() {
				ui.Scroll(c).Grow(1).Children(func() {
					a.stepList(c, t)
				})
			})
		}, func() {
			a.editor(c, t)
		}).Grow(1)
	})
}
