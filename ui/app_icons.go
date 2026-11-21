package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

// resizeGripIcon 是标准 textarea 右下角的缩放手柄（两条斜线的小三角）。
// 用 currentColor 绘制，随主题文字色变化；比底部整条横线更通用、克制，
// 与浏览器/主流编辑器的 textarea resize handle 视觉一致。
var resizeGripIcon = ui.MustParseSVG([]byte(
	`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"><path d="M14 5L5 14"/><path d="M14 10l-4 4"/></svg>`))

// 步骤列表右侧的工具按钮图标：细线箭头与垃圾桶，用 currentColor 随文字色变化。

// 步骤列表右侧的工具按钮图标：细线箭头与垃圾桶，用 currentColor 随文字色变化。
var (
	iconMoveUp = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M8 13V3.5"/><path d="M4.5 7L8 3.5 11.5 7"/></svg>`))
	iconMoveDown = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M8 3v9.5"/><path d="M4.5 9L8 12.5 11.5 9"/></svg>`))
	iconDelete = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"><path d="M2.7 4.2h10.6"/><path d="M6.2 4.2V2.7h3.6v1.5"/><path d="M4.2 4.2l.6 8.9h6.4l.6-8.9"/><path d="M6.6 6.6v4.1M9.4 6.6v4.1"/></svg>`))
	// 顶栏与 Collection 图标。
	iconSearch = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><circle cx="7" cy="7" r="4.2"/><path d="M10.2 10.2L14 14"/></svg>`))
	iconFolder = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"><path d="M2 4.5h4l1.4 1.6H14v6.4H2z"/></svg>`))
	iconSend = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M2 8l12-5-4 12-2.2-5.2L2 8z"/><path d="M7.8 9.8L14 3"/></svg>`))
	iconPlus = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><path d="M8 3v10M3 8h10"/></svg>`))
	iconNew = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"><path d="M4 2h5l3 3v9H4z"/><path d="M9 2v3h3"/></svg>`))
	iconOpen = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"><path d="M2 5V3.5h4l1.3 1.5H14V13H2z"/></svg>`))
	iconSave = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"><path d="M3 2.5h8.5L13.5 5v8.5h-11z"/><path d="M5 2.5v4h6v-4M5 13.5v-4h6v4"/></svg>`))
	iconImport = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"><path d="M8 2v8M5 7l3 3 3-3"/><path d="M3 11.5V14h10v-2.5"/></svg>`))
	// 文件树：通用文件图标（步骤作为文件节点，用类型色着色）。
	iconFile = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"><path d="M4 1.8h5l3 3v9.4H4z"/><path d="M9 1.8v3h3"/></svg>`))
)

// Postman 风格配色：品牌橙与 HTTP 方法的语义色。

// Postman 风格配色：品牌橙与 HTTP 方法的语义色。
var (
	brandOrange  = ui.RGB(255, 108, 55) // Postman 标志橙
	methodColors = map[string]ui.Color{
		"GET": ui.RGB(97, 175, 254), "POST": ui.RGB(73, 204, 144),
		"PUT": ui.RGB(252, 161, 48), "PATCH": ui.RGB(80, 227, 194),
		"DELETE": ui.RGB(249, 62, 62), "HEAD": ui.RGB(144, 18, 254),
	}
	// 步骤类型图标色，让左侧 Collection 树一眼区分组件类型。
	stepColors = map[model.StepType]ui.Color{
		model.StepRequest: ui.RGB(97, 175, 254), model.StepTCP: ui.RGB(34, 197, 94), model.StepLoop: ui.RGB(144, 18, 254),
		model.StepBranch: ui.RGB(252, 161, 48), model.StepSwitch: ui.RGB(80, 227, 194),
		model.StepParallel: ui.RGB(59, 130, 246), model.StepMerge: ui.RGB(59, 130, 246),
		model.StepProcess: ui.RGB(148, 163, 184), model.StepScript: ui.RGB(234, 179, 8),
		model.StepAssert: ui.RGB(73, 204, 144), model.StepExport: ui.RGB(249, 115, 22),
	}
)

// methodColor 返回 HTTP 方法的语义色，未知方法用强调色。

// methodColor 返回 HTTP 方法的语义色，未知方法用强调色。
func methodColor(m string) ui.Color {
	if c, ok := methodColors[strings.ToUpper(strings.TrimSpace(m))]; ok {
		return c
	}
	return ui.RGB(148, 163, 184)
}

// iconButton 是通用图标按钮：透明底、hover 高亮，用于顶栏与侧栏。

// iconButton 是通用图标按钮：透明底、hover 高亮，用于顶栏与侧栏。
func iconButton(c *ui.Context, t *ui.Theme, key string, icon *ui.SVG, tip string, size float32, col ui.Color) ui.Element {
	b := ui.ButtonBase(c.Key(key)).Size(size, size).Radius(5).Label(tip).Tooltip(tip).Cursor(ui.CursorPointer)
	b.Children(func() {
		ui.Icon(c, icon).Size(size-8, size-8).TextColor(col)
	})
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	return b
}

// stepToolButton 渲染步骤列表右侧的轻量图标按钮：默认透明无边框，仅 hover 时淡色高亮，
// 比标准小方块按钮更克制；danger 用于删除按钮，图标着危险色。

// stepToolButton 渲染步骤列表右侧的轻量图标按钮：默认透明无边框，仅 hover 时淡色高亮，
// 比标准小方块按钮更克制；danger 用于删除按钮，图标着危险色。
func stepToolButton(c *ui.Context, t *ui.Theme, key string, icon *ui.SVG, tip string, disabled, danger bool) ui.Element {
	b := ui.ButtonBase(c.Key(key)).Size(22, 22).Radius(4).Label(tip).Tooltip(tip).
		Cursor(ui.CursorPointer).Disabled(disabled)
	col := t.TextMuted
	if danger {
		col = t.Danger
	}
	b.Children(func() {
		ui.Icon(c, icon).Size(13, 13).TextColor(col)
	})
	if b.Hovered() && !disabled {
		b.Background(t.SurfaceHover)
	}
	return b
}

// App 保存界面所有可变状态。view 是纯函数：从状态构建界面。
