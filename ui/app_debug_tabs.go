package ui

import (
	"encoding/json"

	"github.com/egoist/mygo/ui"
)

// 接口调试请求子标签的修改标记：与步骤编排标签同一套语义。
// 基线在初始化、载入步骤、载入历史请求时刷新（表示"已确认的来源内容"）；
// 之后用户编辑了哪个子标签，哪个标签名后就出现绿色圆点，发送不清除。

var debugReqTabNames = []string{"Params", "Headers", "Body", "Authorization"}

// debugTabFingerprint 把某个调试请求子标签涉及的字段序列化为指纹。
// 方法/URL/超时在请求行（不属于子标签），故不计入任何子标签指纹。
func debugTabFingerprint(a *App, tab int) string {
	var v []any
	switch tab {
	case 0: // Params
		v = []any{a.debugQuery}
	case 1: // Headers
		v = []any{a.debugHeaders}
	case 2: // Body
		v = []any{a.debugBody, a.debugBodyType, a.debugFormData, a.debugGQLQuery, a.debugGQLVars}
	case 3: // Authorization
		v = []any{a.debugAuthType, a.debugAuthToken, a.debugUser, a.debugPass,
			a.debugKeyName, a.debugKeyValue, a.debugKeyIn,
			a.debugTokenURL, a.debugClientID, a.debugClientSecret, a.debugScope, a.debugGrantType}
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// debugTabIsDirty 判断某个调试请求子标签相对基线是否被改过。
func (a *App) debugTabIsDirty(tab int) bool {
	if a.debugTabDirty == nil {
		a.debugTabDirty = map[string]string{}
	}
	name := debugReqTabNames[tab]
	cur := debugTabFingerprint(a, tab)
	base, ok := a.debugTabDirty[name]
	if !ok {
		a.debugTabDirty[name] = cur
		return false
	}
	return base != cur
}

// refreshDebugTabBaseline 把当前调试请求配置记为基线（清除所有圆点）。
func (a *App) refreshDebugTabBaseline() {
	if a.debugTabDirty == nil {
		a.debugTabDirty = map[string]string{}
	}
	for i, name := range debugReqTabNames {
		a.debugTabDirty[name] = debugTabFingerprint(a, i)
	}
}

// debugDirtyDotView 调试请求标签的绿色圆点，tooltip 说明相对"载入内容"被改。
func debugDirtyDotView(c *ui.Context, t *ui.Theme) {
	ui.Text(c, "•").FontSize(12).TextColor(t.Success).Tooltip("此请求配置已修改（相对最近载入的内容）")
}

// debugTabBarWithDots 用 TabsBase 渲染调试请求子标签，编辑过的标签名后跟绿色圆点。
func (a *App) debugTabBarWithDots(c *ui.Context, t *ui.Theme) int {
	parts := ui.TabsBase(c, &a.debugReqTab, len(debugReqTabNames))
	parts.List.Gap(2).Children(func() {
		for i, name := range debugReqTabNames {
			i := i
			tab := parts.Tab(i).Padding(6, 12).Radius(6)
			if i == a.debugReqTab {
				tab.Background(t.Surface)
			}
			tab.Children(func() {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					label := ui.Text(c, name).FontSize(12)
					if i == a.debugReqTab {
						label.Bold().TextColor(t.Accent)
					}
					if a.debugTabIsDirty(i) {
						debugDirtyDotView(c, t)
					}
				})
			})
		}
	})
	dirty := 0
	for i := range debugReqTabNames {
		if a.debugTabIsDirty(i) {
			dirty++
		}
	}
	return dirty
}
