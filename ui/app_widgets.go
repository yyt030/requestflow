package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

// clampHeight 把拖拽后的高度限制在 [min, max] 内。
// emptyState 渲染居中的空状态占位：主提示 + 可选副提示，统一各面板无数据时的视觉。
func emptyState(c *ui.Context, t *ui.Theme, title, subtitle string) {
	ui.Column(c).Fill().Center().Gap(6).Children(func() {
		ui.Text(c, title).FontSize(13).Bold().TextColor(t.TextMuted)
		if subtitle != "" {
			ui.Text(c, subtitle).FontSize(11).TextColor(t.TextMuted)
		}
	})
}

// intInput 渲染一个整数输入框，把字符串编辑值同步回 *int；解析失败时保留原值。
// 统一处理 fmt.Sprint/parseInt 样板，供循环/翻页等数字字段复用。

// intInput 渲染一个整数输入框，把字符串编辑值同步回 *int；解析失败时保留原值。
// 统一处理 fmt.Sprint/parseInt 样板，供循环/翻页等数字字段复用。
func intInput(c *ui.Context, target *int, label, placeholder string, width float32) {
	s := fmt.Sprint(*target)
	if placeholder != "" && *target == 0 {
		s = ""
	}
	input := ui.TextInput(c, &s).Label(label).Placeholder(placeholder)
	if width > 0 {
		input.Width(width)
	} else {
		input.Grow(1)
	}
	if s != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			*target = n
		}
	}
}

func clampHeight(v, min, max float32) float32 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// fieldHint 在字段下方渲染一行浅灰色注释，说明该字段的用途或格式。
// 组件配置统一用它给出填写指引，降低界面配置的学习成本。

// fieldHint 在字段下方渲染一行浅灰色注释，说明该字段的用途或格式。
// 组件配置统一用它给出填写指引，降低界面配置的学习成本。
func fieldHint(c *ui.Context, t *ui.Theme, text string) {
	// 提示文字略提亮，避免 10px 灰字在浅色主题下对比度不足难以阅读。
	// 用 Row + 固定宽图标槽，让说明文字从同一竖线起，与标题左对齐（不再用前导空格凑缩进）。
	ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "ⓘ").FontSize(10).TextColor(t.TextMuted.Mix(t.Text, 0.3))
		ui.Text(c, text).FontSize(10).TextColor(t.TextMuted.Mix(t.Text, 0.3))
	})
}

// resizableTextArea 渲染一个可拖拽调整高度的多行文本框。
// 手柄是文本框右下角的标准斜线小三角（同浏览器 textarea 的 resize handle），
// 鼠标悬停显示上下调整光标，上下拖动即改变高度。
// height 指向 App 中保存该文本框高度的变量；min/max 限制避免缩到不可用或撑满屏幕。
// 返回文本框内容本帧是否变化，便于调用方把临时缓冲写回 Map 等结构（旧调用可忽略返回值）。
func resizableTextArea(c *ui.Context, t *ui.Theme, value *string, height *float32, placeholder string, min, max float32) bool {
	*height = clampHeight(*height, min, max)
	h := *height
	changed := false
	// 外层 Box 作为定位参照，手柄绝对定位到右下角、悬浮在文本框之上。
	ui.Box(c).Children(func() {
		if ui.TextArea(c, value).Placeholder(placeholder).Height(*height).Changed() {
			changed = true
		}
		// 手柄用 currentColor 跟随文字色；命中区稍大于图标本身，便于拖动。
		grip := ui.Box(c).Absolute().Right(1).Bottom(1).Width(20).Height(20).
			Cursor(ui.CursorResizeNS).Center().Tooltip("拖动右下角调整高度")
		if _, dy, held := grip.Dragged(); held && dy != 0 {
			*height = clampHeight(h+dy, min, max)
		}
		grip.Children(func() {
			ui.Icon(c, resizeGripIcon).Size(16, 16).TextColor(t.Text.Mix(t.TextMuted, 0.35))
		})
	})
	return changed
}

// formattedTextArea 在可拖拽文本框上方加一个「格式化」按钮，点击后自动美化内容
// （JSON/XML/HTML/键值对），并在按钮旁短暂显示识别到的类型。
// status 是每次渲染重置的临时提示缓冲，由 App 各编辑区持有。

// formattedTextArea 在可拖拽文本框上方加一个「格式化」按钮，点击后自动美化内容
// （JSON/XML/HTML/键值对），并在按钮旁短暂显示识别到的类型。
// status 是每次渲染重置的临时提示缓冲，由 App 各编辑区持有。
func formattedTextArea(c *ui.Context, t *ui.Theme, value *string, height *float32,
	placeholder string, min, max float32, status *string) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if ui.Button(c, "✨ 格式化").Tooltip("自动美化内容：JSON / XML / HTML / 键值对").Clicked() {
			out, kind, changed := FormatContent(*value)
			if changed {
				*value = out
				*status = "已按 " + kind + " 格式化"
			} else if kind != "" {
				*status = kind + " 已是规范格式"
			} else {
				*status = "未识别可格式化的内容"
			}
		}
		if *status != "" {
			ui.Text(c, *status).FontSize(10).TextColor(t.TextMuted)
		}
	})
	resizableTextArea(c, t, value, height, placeholder, min, max)
}

// previewURL 截断 URL 用于日志显示。

// previewURL 截断 URL 用于日志显示。
func previewURL(u string) string {
	if len(u) > 48 {
		return u[:48] + "…"
	}
	return u
}
