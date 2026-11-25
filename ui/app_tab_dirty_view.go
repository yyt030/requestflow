package ui

import (
	"requestflow/model"

	"github.com/egoist/mygo/ui"
)

// tabIsDirty 判断标签相对基线是否被修改；首次见到的标签记录基线并视为干净。
func (a *App) tabIsDirty(step *model.Step, tab string) bool {
	key := tabKey(step, tab)
	cur := tabFingerprint(step, tab)
	base, ok := a.tabDirty[key]
	if !ok {
		a.tabDirty[key] = cur
		return false
	}
	return base != cur
}

// tabDirtyFlags 返回每个标签是否被修改，以及被修改的标签数量。
func (a *App) tabDirtyFlags(step *model.Step) ([]bool, int) {
	tabs := stepTabNames(step)
	flags := make([]bool, len(tabs))
	dirty := 0
	for i, name := range tabs {
		if a.tabIsDirty(step, name) {
			flags[i] = true
			dirty++
		}
	}
	return flags, dirty
}

// refreshTabBaseline 把当前所有步骤的所有标签指纹记为基线（表示"已确认"）。
func (a *App) refreshTabBaseline() {
	if a.tabDirty == nil {
		a.tabDirty = map[string]string{}
	}
	for i := range a.scenario.Steps {
		step := &a.scenario.Steps[i]
		tabs := stepTabNames(step)
		// 请求行的 Method/URL 已不占标签位，但仍保留「常规」作为隐藏指纹分组，
		// 持续追踪顶部请求行的改动（UI 不显示该标签）。
		if step.Type == model.StepRequest {
			tabs = append(tabs, "常规")
		}
		for _, tab := range tabs {
			a.tabDirty[tabKey(step, tab)] = tabFingerprint(step, tab)
		}
	}
}

// dirtyDotView 在 dirty 标签名后画一个绿色圆点，标记该标签内容被改过。
func dirtyDotView(c *ui.Context, t *ui.Theme) {
	ui.Text(c, "•").FontSize(12).TextColor(t.Success).Tooltip("此标签内容已修改，未保存")
}

// tabBarWithDots 用 TabsBase 自定义渲染标签栏：选中态保留主题外观，
// 每个标签名后按需跟一个绿色小圆点。返回被修改的标签数量。
func (a *App) tabBarWithDots(c *ui.Context, t *ui.Theme, step *model.Step) int {
	tabs := stepTabNames(step)
	flags, dirtyCount := a.tabDirtyFlags(step)
	parts := ui.TabsBase(c, &a.stepTab, len(tabs))
	parts.List.Gap(2).Children(func() {
		for i, name := range tabs {
			i := i
			tab := parts.Tab(i).Padding(6, 12).Radius(6)
			if i == a.stepTab {
				tab.Background(t.Surface)
			}
			tab.Children(func() {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					label := ui.Text(c, name).FontSize(12)
					if i == a.stepTab {
						label.Bold().TextColor(t.Accent)
					}
					if flags[i] {
						dirtyDotView(c, t)
					}
				})
			})
		}
	})
	return dirtyCount
}

// clampTabTo 把 a.stepTab 限制在给定标签列表范围内（支持自定义标签子集）。
func (a *App) clampTabTo(tabs []string) int {
	if a.stepTab < 0 || a.stepTab >= len(tabs) {
		a.stepTab = 0
	}
	return a.stepTab
}

// tabBarWithNames 与 tabBarWithDots 类似，但用显式传入的标签名列表渲染，
// 供"引用集合请求的步骤只显示提取/断言/脚本"这类标签子集场景使用。
func (a *App) tabBarWithNames(c *ui.Context, t *ui.Theme, step *model.Step, tabs []string) int {
	flags, dirtyCount := a.tabDirtyFlags(step)
	parts := ui.TabsBase(c, &a.stepTab, len(tabs))
	parts.List.Gap(2).Children(func() {
		for i, name := range tabs {
			i := i
			tab := parts.Tab(i).Padding(6, 12).Radius(6)
			if i == a.stepTab {
				tab.Background(t.Surface)
			}
			tab.Children(func() {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					label := ui.Text(c, name).FontSize(12)
					if i == a.stepTab {
						label.Bold().TextColor(t.Accent)
					}
					if i < len(flags) && flags[i] {
						dirtyDotView(c, t)
					}
				})
			})
		}
	})
	return dirtyCount
}
