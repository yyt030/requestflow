package ui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"requestflow/engine"
	"requestflow/model"
)

// 本文件实现「提取 / 数据处理」组件的实时试算预览：
// 复用最近一次接口调试响应，按当前配置自动、实时展示处理结果，
// 并内置可一键套用的默认例子，解决「这些组件不知怎么用」的问题。

// previewVars 构造试算用的变量上下文：场景变量 + baseUrl，并把当前步骤之前
// 各步骤已配置的提取规则对最近响应试算出的值回填，让 filter/row 等下游 op 有真实来源。
func (a *App) previewVars(step *model.Step) map[string]string {
	vars := map[string]string{"baseUrl": a.scenario.BaseURL}
	for k, v := range a.scenario.Variables {
		vars[k] = v
	}
	if a.debugResult == nil || !a.debugResult.OK {
		return vars
	}
	resp := engine.PreviewResponseFromDebug(a.debugResult)
	if resp == nil {
		return vars
	}
	// 汇总场景中除当前步骤外所有提取规则，产出下游可用的变量。
	var extracts []model.Extract
	for i := range a.scenario.Steps {
		s := &a.scenario.Steps[i]
		if s == step {
			continue
		}
		extracts = append(extracts, s.Extract...)
	}
	if len(extracts) == 0 {
		return vars
	}
	for _, item := range engine.PreviewExtracts(extracts, vars, resp) {
		if item.Err == "" && item.Name != "" && item.Name != "(未命名)" {
			vars[item.Name] = engine.PreviewValueString(item.Value)
		}
	}
	// row op 在 foreach 循环内依赖 item 变量。预览没有真实循环，
	// 这里用第一个数组变量的首元素喂给 item，让「组装结果行」例子可直接试算。
	if _, ok := vars["item"]; !ok {
		vars["item"] = firstArrayElement(vars)
	}
	return vars
}

// firstArrayElement 从变量中找出第一个非空数组的首元素，序列化为字符串供 item 使用。
func firstArrayElement(vars map[string]string) string {
	for _, v := range vars {
		t := strings.TrimSpace(v)
		if strings.HasPrefix(t, "[") {
			var arr []interface{}
			if err := json.Unmarshal([]byte(t), &arr); err == nil && len(arr) > 0 {
				return engine.PreviewValueString(arr[0])
			}
		}
	}
	return ""
}

// extractPreview 在「提取」标签底部展示逐条提取规则的实时试算结果。
func (a *App) extractPreview(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Divider(c)
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "实时预览").FontSize(12).Bold().TextColor(t.Accent)
		ui.Text(c, "基于最近一次调试响应自动刷新").FontSize(10).TextColor(t.TextMuted)
		ui.Spacer(c)
		a.previewSourceBadge(c, t)
		if len(step.Extract) > 0 {
			if ui.Button(c, "清空提取").Clicked() {
				step.Extract = nil
			}
		}
	})

	if a.debugResult == nil || !a.debugResult.OK {
		a.previewEmpty(c, t)
		a.extractExamples(c, t, step)
		return
	}
	if len(step.Extract) == 0 {
		fieldHint(c, t, "尚未配置提取规则。下方「默认例子」可一键填入，预览会立即显示结果。")
		a.extractExamples(c, t, step)
		return
	}

	resp := engine.PreviewResponseFromDebug(a.debugResult)
	vars := a.previewVars(step)
	items := engine.PreviewExtracts(step.Extract, vars, resp)
	ui.Column(c).Gap(6).Children(func() {
		for i, item := range items {
			a.previewExtractRow(c, t, i, item)
		}
	})
	a.extractExamples(c, t, step)
}

// previewExtractRow 渲染单条提取结果：变量名 + 来源/方式 + 值 + 错误。
func (a *App) previewExtractRow(c *ui.Context, t *ui.Theme, idx int, item engine.PreviewExtractItem) {
	ui.Column(c).Padding(8).Gap(4).Background(t.Surface).Radius(8).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Badge(c, fmt.Sprintf("%d", idx+1)).TextColor(t.Accent)
			ui.Text(c, item.Name).FontSize(11).Bold()
			ui.Text(c, fmt.Sprintf("%s / %s · %s", item.Source, item.Kind, item.Path)).FontSize(10).TextColor(t.TextMuted)
			ui.Spacer(c)
			if item.Err == "" && item.Count > 0 {
				ui.Text(c, fmt.Sprintf("%d 条", item.Count)).FontSize(10).TextColor(t.Success)
			}
		})
		if item.Err != "" {
			ui.Text(c, "✗ "+item.Err).FontSize(10).TextColor(t.Danger)
		} else {
			ui.Column(c).Selectable().Children(func() {
				ui.Text(c, item.Text).FontSize(11)
			})
			if item.Default {
				fieldHint(c, t, "未提取到值，已使用默认值")
			}
		}
	})
}

// processPreview 在「处理」标签底部展示当前 op 的实时试算（来源 → 输出对比）。
func (a *App) processPreview(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Divider(c)
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "实时预览").FontSize(12).Bold().TextColor(t.Accent)
		ui.Text(c, "基于最近响应提取的变量，自动刷新").FontSize(10).TextColor(t.TextMuted)
		ui.Spacer(c)
		a.previewSourceBadge(c, t)
	})

	if a.debugResult == nil || !a.debugResult.OK {
		a.previewEmpty(c, t)
		a.processExamples(c, t, step)
		return
	}

	vars := a.previewVars(step)
	item := engine.PreviewProcess(*step, vars)
	ui.Column(c).Padding(10).Gap(6).Background(t.Surface).Radius(8).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "处理操作").FontSize(11).TextColor(t.TextMuted)
			ui.Text(c, procOpLabel(item.Op)).FontSize(11).Bold()
			ui.Text(c, "→ 输出变量").FontSize(11).TextColor(t.TextMuted)
			ui.Text(c, item.OutName).FontSize(11).Bold().TextColor(t.Accent)
		})
		if item.Err != "" {
			ui.Text(c, "✗ "+item.Err).FontSize(11).TextColor(t.Danger)
			fieldHint(c, t, "多半是来源变量为空或参数不匹配；可点下方默认例子套用一组可运行配置。")
		} else {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "结果").FontSize(11).TextColor(t.TextMuted)
				ui.Text(c, item.Delta).FontSize(10).TextColor(t.TextMuted)
			})
			ui.Column(c).Selectable().Children(func() {
				ui.Text(c, item.AfterT).FontSize(11)
			})
		}
	})
	a.processExamples(c, t, step)
}

// previewSourceBadge 显示当前预览数据来源（最近调试响应的状态）。
func (a *App) previewSourceBadge(c *ui.Context, t *ui.Theme) {
	if a.debugResult == nil {
		ui.Badge(c, "无调试响应").TextColor(t.TextMuted).Tooltip("请先在【接口调试】发送一次请求")
		return
	}
	if !a.debugResult.OK {
		ui.Badge(c, "调试失败").TextColor(t.Danger).Tooltip(a.debugResult.Err)
		return
	}
	label := fmt.Sprintf("数据源: %d %s", a.debugResult.StatusCode, a.debugResult.StatusText)
	ui.Badge(c, label).TextColor(t.Success).Tooltip("复用最近一次接口调试响应，无需重新发送")
}

// previewEmpty 预览无响应时的引导，提供一键载入内置示例响应。
func (a *App) previewEmpty(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Gap(6).Padding(12).Background(t.Surface).Radius(8).Children(func() {
		ui.Text(c, "还没有可预览的调试响应").FontSize(11).Bold()
		fieldHint(c, t, "到【接口调试】发送一次请求即可自动预览；也可直接载入内置示例响应体验下面的例子。")
		ui.Row(c).Gap(8).Children(func() {
			if ui.Button(c, "载入示例 JSON 响应").Clicked() {
				a.loadSampleDebugResponse(true)
			}
			if ui.Button(c, "载入示例 HTML 响应").Clicked() {
				a.loadSampleDebugResponse(false)
			}
		})
	})
}

// loadSampleDebugResponse 把内置示例写进调试结果，供预览立即使用。
func (a *App) loadSampleDebugResponse(asJSON bool) {
	body := sampleJSONResponse
	ct := "application/json"
	if !asJSON {
		body = sampleHTMLResponse
		ct = "text/html; charset=utf-8"
	}
	a.debugResult = &engine.DebugResult{
		OK: true, StatusCode: 200, StatusText: "OK",
		Method: "GET", URL: "sample://preview",
		Body: body, Formatted: body, IsJSON: asJSON, Size: len(body),
		Headers: []engine.KV{{Key: "Content-Type", Value: ct}},
	}
}

// truncatePreview 供示例文案使用，避免过长。
func truncatePreview(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// previewKind 区分提取/处理示例，避免文件名与函数名冲突。
type previewKind int

const (
	previewExtract previewKind = iota
	previewProcess
)

// previewExampleButton 统一的「套用默认例子」按钮样式前缀。
func previewExampleLabel(kind previewKind, name string) string {
	prefix := "提取例"
	if kind == previewProcess {
		prefix = "处理例"
	}
	if strings.TrimSpace(name) == "" {
		return prefix
	}
	return prefix + " · " + name
}
