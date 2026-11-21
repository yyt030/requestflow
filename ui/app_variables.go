package ui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// variableKind 返回变量的展示类型与元素/字段数量，供变量预览表格显示。
func variableKind(v interface{}) (string, int) {
	switch x := v.(type) {
	case []interface{}:
		return "数组", len(x)
	case []string:
		return "数组", len(x)
	case []map[string]interface{}:
		return "数组", len(x)
	case map[string]interface{}:
		return "对象", len(x)
	case map[string]string:
		return "对象", len(x)
	case string:
		var parsed interface{}
		t := strings.TrimSpace(x)
		if len(t) > 0 && (t[0] == '[' || t[0] == '{') && json.Unmarshal([]byte(t), &parsed) == nil {
			return variableKind(parsed)
		}
		return "文本", 0
	case nil:
		return "空", 0
	default:
		return "标量", 0
	}
}

// variableCell 把单个变量值渲染为可复制展示文本；数组/对象返回简短摘要。

// variableCell 把单个变量值渲染为可复制展示文本；数组/对象返回简短摘要。
func variableCell(v interface{}) string {
	b, err := json.Marshal(v)
	if err == nil && len(b) > 0 && b[0] == '"' {
		return string(b[1 : len(b)-1])
	}
	s := string(b)
	if len(s) > 160 {
		return s[:160] + "…"
	}
	return s
}

// variableElements 把数组/对象展开为 (键, 值) 列表，供变量预览展开区展示。

// variableElements 把数组/对象展开为 (键, 值) 列表，供变量预览展开区展示。
func variableElements(v interface{}) [][2]string {
	list, ok := toAnySlice(v)
	if ok {
		out := make([][2]string, 0, len(list))
		for i, item := range list {
			out = append(out, [2]string{fmt.Sprintf("[%d]", i), variableCell(item)})
		}
		return out
	}
	if m, ok := toAnyMap(v); ok {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make([][2]string, 0, len(keys))
		for _, k := range keys {
			out = append(out, [2]string{k, variableCell(m[k])})
		}
		return out
	}
	return nil
}

func toAnySlice(v interface{}) ([]interface{}, bool) {
	switch x := v.(type) {
	case []interface{}:
		return x, true
	case []string:
		out := make([]interface{}, 0, len(x))
		for _, s := range x {
			out = append(out, s)
		}
		return out, true
	case []map[string]interface{}:
		out := make([]interface{}, 0, len(x))
		for _, m := range x {
			out = append(out, m)
		}
		return out, true
	case string:
		var parsed interface{}
		t := strings.TrimSpace(x)
		if len(t) > 0 && t[0] == '[' && json.Unmarshal([]byte(t), &parsed) == nil {
			if list, ok := parsed.([]interface{}); ok {
				return list, true
			}
		}
	}
	return nil, false
}

func toAnyMap(v interface{}) (map[string]interface{}, bool) {
	switch x := v.(type) {
	case map[string]interface{}:
		return x, true
	case map[string]string:
		out := make(map[string]interface{}, len(x))
		for k, val := range x {
			out[k] = val
		}
		return out, true
	case string:
		var parsed interface{}
		t := strings.TrimSpace(x)
		if len(t) > 0 && t[0] == '{' && json.Unmarshal([]byte(t), &parsed) == nil {
			if m, ok := parsed.(map[string]interface{}); ok {
				return m, true
			}
		}
	}
	return nil, false
}

func (a *App) variablesView(c *ui.Context, t *ui.Theme) {
	if a.lastResult == nil {
		ui.Scroll(c).Grow(1).Padding(0, 12, 8, 12).Children(func() {
			ui.Text(c, "运行后在此查看全部变量（含步骤间传递的数据）").FontSize(11).TextColor(t.TextMuted)
		})
		return
	}
	keys := make([]string, 0, len(a.lastResult.Vars))
	for k := range a.lastResult.Vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	ui.Scroll(c).Grow(1).Padding(0, 12, 8, 12).Gap(2).Children(func() {
		// 表头
		ui.Row(c).Gap(8).Padding(2, 6).Children(func() {
			ui.Text(c, "变量名").FontSize(10).Bold().TextColor(t.TextMuted).Width(180)
			ui.Text(c, "类型/数量").FontSize(10).Bold().TextColor(t.TextMuted).Width(90)
			ui.Text(c, "值").FontSize(10).Bold().TextColor(t.TextMuted).Grow(1)
		})
		ui.Divider(c)
		for _, k := range keys {
			v := a.lastResult.Vars[k]
			kind, n := variableKind(v)
			cell := variableCell(v)
			elements := variableElements(v)
			if len(elements) == 0 || len(elements) == 1 && kind == "文本" {
				// 标量：一行展示完整值
				ui.Row(c.Key(k)).Gap(8).Padding(2, 6).Children(func() {
					ui.Text(c, k).FontSize(11).Bold().Width(180)
					ui.Text(c, kind).FontSize(10).TextColor(t.TextMuted).Width(90)
					ui.Text(c, cell).FontSize(11).TextColor(t.TextMuted).Grow(1)
				})
				continue
			}
			// 数组/对象：点击变量名展开/收起，展开区逐元素成表
			key := k
			open := a.varExpanded[k]
			arrow := "▸ "
			if open {
				arrow = "▾ "
			}
			ui.Row(c.Key(k)).Gap(8).Padding(2, 6).Children(func() {
				if ui.Button(c, arrow+key).Width(180).Clicked() {
					a.varExpanded[key] = !open
				}
				ui.Text(c, fmt.Sprintf("%s · %d", kind, n)).FontSize(10).TextColor(t.Accent).Width(90)
				ui.Text(c, cell).FontSize(11).TextColor(t.TextMuted).Grow(1).SingleLine()
			})
			if open {
				ui.Column(c.Key(k+"-items")).Padding(0, 0, 4, 28).Gap(1).Children(func() {
					for _, el := range elements {
						ui.Row(c).Gap(8).Padding(1, 4).Children(func() {
							ui.Text(c, el[0]).FontSize(10).TextColor(t.Accent).Width(120)
							ui.Text(c, el[1]).FontSize(10).SingleLine().Grow(1)
						})
					}
				})
			}
		}
	})
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// truncateRunes 按 rune 截断字符串（避免中文按字节截断乱码）。

// truncateRunes 按 rune 截断字符串（避免中文按字节截断乱码）。
func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// showErrorDetail 弹框显示完整错误，供状态栏失败信息点击查看。

// showErrorDetail 弹框显示完整错误，供状态栏失败信息点击查看。
func (a *App) showErrorDetail(detail string) {
	go func() {
		_, _ = mygo.Dialog.Message(mygo.MessageOptions{
			Parent:  a.win,
			Type:    mygo.MessageError,
			Title:   "运行失败详情",
			Message: detail,
			Buttons: []string{"确定"},
		})
	}()
}
