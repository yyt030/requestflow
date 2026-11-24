package ui

import (
	"github.com/egoist/mygo/ui"
)

// editorResponseArea 是合并进「步骤编排」的调试响应区：上半配置、下半响应。
// 直接复用独立调试面板的响应渲染（Body/Headers/Cookies/代码 + 复制 + 状态），
// 保证合并前后体验一致；发送入口在步骤配置顶部【▶ 发送】。
func (a *App) editorResponseArea(c *ui.Context, t *ui.Theme) {
	a.debugResponseArea(c, t)
}
