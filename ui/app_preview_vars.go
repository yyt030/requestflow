package ui

import (
	"fmt"
	"sort"

	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

// 本文件实现「可用变量速览」：在处理配置区顶部列出当前场景变量与
// 从最近调试响应提取出的变量的「名称 + 当前试算值 + 来源」，
// 让用户一眼知道 filter/row 等操作的「来源变量 from」该填什么。

// previewVarEntry 一个可用变量的展示条目。
type previewVarEntry struct {
	Name   string
	Value  string
	Origin string // 场景 / 响应提取 / 内置 / 循环模拟
}

// originRank 决定展示顺序：响应提取最靠前（通常是 from 的目标），其次场景、内置、循环模拟。
func originRank(origin string) int {
	switch origin {
	case "响应提取":
		return 0
	case "场景":
		return 1
	case "内置":
		return 2
	default:
		return 3
	}
}

// previewVarEntries 汇总当前可用变量，附带来源标注并排序。
// 复用 previewVars 的计算结果，因此这里展示的值与「实时预览」试算值完全一致。
func (a *App) previewVarEntries(step *model.Step) []previewVarEntry {
	base := map[string]bool{}
	if a.scenario != nil {
		for k := range a.scenario.Variables {
			base[k] = true
		}
		for _, env := range a.scenario.Environments {
			for k := range env {
				base[k] = true
			}
		}
	}
	base["baseUrl"] = true

	full := a.previewVars(step)
	entries := make([]previewVarEntry, 0, len(full))
	for name, value := range full {
		origin := "响应提取"
		switch {
		case name == "baseUrl":
			origin = "内置"
		case name == "item":
			origin = "循环模拟"
		case base[name]:
			origin = "场景"
		}
		entries = append(entries, previewVarEntry{Name: name, Value: value, Origin: origin})
	}
	sort.Slice(entries, func(i, j int) bool {
		ri, rj := originRank(entries[i].Origin), originRank(entries[j].Origin)
		if ri != rj {
			return ri < rj
		}
		return entries[i].Name < entries[j].Name
	})
	return entries
}

// processVarOverview 在处理表单顶部渲染可用变量速览（紧凑列表，值截断、可选中）。
func (a *App) processVarOverview(c *ui.Context, t *ui.Theme, step *model.Step) {
	entries := a.previewVarEntries(step)
	ui.Column(c).Gap(5).Padding(10).Background(t.Surface).Radius(8).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "可用变量").FontSize(12).Bold().TextColor(t.Accent)
			ui.Text(c, "当前值 · 直接作为「来源变量」填写").FontSize(10).TextColor(t.TextMuted)
			ui.Spacer(c)
			ui.Text(c, fmt.Sprintf("共 %d 个", len(entries))).FontSize(10).TextColor(t.TextMuted)
		})
		if len(entries) == 0 {
			fieldHint(c, t, "暂无可用变量。可先在【接口调试】载入示例响应或配置场景变量。")
			return
		}
		for _, e := range entries {
			entry := e
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, entry.Name).FontSize(11).Bold().Width(96)
				ui.Column(c).Selectable().Grow(1).Children(func() {
					ui.Text(c, truncatePreview(entry.Value, 72)).FontSize(10).TextColor(t.TextMuted)
				})
				ui.Badge(c, entry.Origin).TextColor(originColor(t, entry.Origin))
			})
		}
		fieldHint(c, t, "值可选中复制；「响应提取」来自最近调试响应，是 filter/sort/limit 等操作的典型来源。")
	})
}

// originColor 按来源给徽章着色，便于快速区分变量出处。
func originColor(t *ui.Theme, origin string) ui.Color {
	switch origin {
	case "响应提取":
		return t.Success
	case "场景":
		return t.Accent
	case "内置":
		return t.TextMuted
	default:
		return t.TextMuted
	}
}
