package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

// loopTabView 按 Postman 风格标签渲染循环步骤的单个配置维度。
func (a *App) loopTabView(c *ui.Context, t *ui.Theme, step *model.Step, tab string) {
	if step.Loop == nil {
		step.Loop = &model.Loop{Mode: "until", Max: 20, PageVar: "page", PageStart: 1, PageStep: 1}
	}
	switch tab {
	case "循环配置":
		a.loopConfigView(c, t, step, step.Loop)
	case "去重":
		a.loopDedupView(c, t, step.Loop)
	case "健壮性":
		a.loopRobustView(c, t, step.Loop)
	case "循环体":
		a.loopBodyView(c, t, step)
	}
}

func (a *App) loopConfigView(c *ui.Context, t *ui.Theme, step *model.Step, lp *model.Loop) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "循环模式").FontSize(12).Bold()
		ui.Spacer(c)
		ui.Select(c, &lp.Mode, []string{"until", "count", "foreach"}).Width(110)
	})
	fieldHint(c, t, map[string]string{
		"until":   "翻页场景常用：每轮执行循环体，直到终止条件成立（如返回空数组）退出。",
		"count":   "固定执行 N 次，适合已知页数的场景。",
		"foreach": "遍历一个数组变量，逐个元素执行循环体。",
	}[lp.Mode])

	switch lp.Mode {
	case "count":
		intInput(c, &lp.Count, "次数 Count", "", 140)
	case "foreach":
		ui.Row(c).Gap(8).Children(func() {
			ui.TextInput(c, &lp.ListVar).Placeholder("rows").Label("数组变量 ListVar").Grow(1)
			ui.TextInput(c, &lp.ItemVar).Placeholder("item").Label("元素变量 ItemVar").Grow(1)
			ui.TextInput(c, &lp.IndexVar).Placeholder("index").Label("索引变量 IndexVar").Grow(1)
		})
		fieldHint(c, t, "逐项取出数组元素：当前元素存入 ItemVar，下标存入 IndexVar。")
	case "until":
		ui.TextInput(c, &lp.BreakWhen).Placeholder("{{len(rows)}} == 0").Label("终止条件 BreakWhen")
		fieldHint(c, t, "表达式为真时退出循环，例如响应数组为空：{{len(rows)}} == 0。")
	}

	ui.Divider(c)
	ui.Text(c, "翻页 / 计数").FontSize(11).Bold()
	ui.Row(c).Gap(8).Children(func() {
		pageVar := lp.PageVar
		if ui.TextInput(c, &pageVar).Label("页码变量").Grow(1).Changed() {
			lp.PageVar = pageVar
		}
		intInput(c, &lp.PageStart, "起始页", "", 80)
		intInput(c, &lp.PageStep, "步长", "", 70)
		intInput(c, &lp.Max, "最大轮数", "", 88)
	})
	fieldHint(c, t, "翻页时在 URL 用 {{页码变量}}（默认 page）；最大轮数用于兜底防止死循环。")
	_ = step
}

func (a *App) loopDedupView(c *ui.Context, t *ui.Theme, lp *model.Loop) {
	ui.Text(c, "累积去重（可选）").FontSize(12).Bold()
	fieldHint(c, t, "循环结束后对「去重数组变量」按「去重字段」去重（字段为空按整行），去掉的重复数写入日志；填了「重复数变量」也会写入该变量。留空去重数组变量则不启用。")
	ui.Row(c).Gap(8).Children(func() {
		ui.TextInput(c, &lp.DedupVar).Placeholder("allRows").Label("去重数组变量").Grow(1)
		ui.TextInput(c, &lp.DedupField).Placeholder("link").Label("去重字段").Grow(1)
		ui.TextInput(c, &lp.DedupStat).Placeholder("dupCount").Label("重复数变量").Grow(1)
	})
}

func (a *App) loopRobustView(c *ui.Context, t *ui.Theme, lp *model.Loop) {
	ui.Text(c, "健壮性（可选）").FontSize(12).Bold()
	fieldHint(c, t, "间隔 ms：每轮前等待，限速防反爬；空页检测变量：本轮该数组少于「最少行」即视为抓空，按「空页重试」次数重跑本轮并记日志。")
	ui.Row(c).Gap(8).Children(func() {
		intInput(c, &lp.DelayMs, "间隔 ms", "0", 100)
		ui.TextInput(c, &lp.MinRowsVar).Placeholder("pageRows").Label("空页检测变量").Grow(1)
		intInput(c, &lp.MinRows, "最少行", "0", 80)
		intInput(c, &lp.RetryOnEmpty, "空页重试", "0", 80)
	})
}

func (a *App) loopBodyView(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, fmt.Sprintf("循环体（%d 个子步骤）", len(step.Children))).FontSize(12).Bold()
		ui.Spacer(c)
		if ui.Button(c, "+ HTTP 请求").Clicked() {
			s := model.Step{ID: newID(), Name: "循环内请求", Type: model.StepRequest, Enabled: true, Method: "GET"}
			step.Children = append(step.Children, s)
		}
		if ui.Button(c, "+ 处理").Clicked() {
			s := model.Step{ID: newID(), Name: "循环内处理", Type: model.StepProcess, Enabled: true}
			step.Children = append(step.Children, s)
		}
	})
	for i := range step.Children {
		child := &step.Children[i]
		idx := i
		ui.Column(c.Key(fmt.Sprintf("child-%d", idx))).Padding(8).Gap(6).Background(t.Surface).Radius(8).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, a.stepLabel(*child)).Grow(1)
				if ui.Button(c, "移除").Clicked() {
					step.Children = append(step.Children[:idx], step.Children[idx+1:]...)
				}
			})
			ui.TextInput(c, &child.Name).Label("名称")
			if child.Type == model.StepRequest {
				ui.TextInput(c, &child.Method).Label("方法")
				ui.TextInput(c, &child.URL).Placeholder("{{baseUrl}}/list?page={{page}}").Label("URL")
				ui.TextInput(c, &child.Body).Label("Body")
				a.editorExtract(c, t, child)
			} else {
				ops := processOps()
				op := child.Op
				if op == "" {
					op = "set"
				}
				if ui.Select(c, &op, ops).Label("处理操作").Changed() {
					child.Op = op
				}
				a.renderProcessForm(c, t, child, a.processFormRawOpen(child), collectVarNames(a.scenario))
			}
		})
	}
}
