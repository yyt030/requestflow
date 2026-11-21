package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

func (a *App) editorLoop(c *ui.Context, t *ui.Theme, step *model.Step) {
	if step.Loop == nil {
		step.Loop = &model.Loop{Mode: "until", Max: 20, PageVar: "page", PageStart: 1, PageStep: 1}
	}
	lp := step.Loop
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "循环模式").FontSize(12).Bold()
		ui.Spacer(c)
		modes := []string{"until", "count", "foreach"}
		ui.Select(c, &lp.Mode, modes).Width(110)
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
		// 三个短变量一行排列。
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
	// 页码相关字段短，一行排布：页码变量 / 起始页 / 步长 / 最大轮数。
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

	// 累积去重（P1）：循环结束后对累积数组按字段去重并提示重复数。
	ui.Divider(c)
	ui.Text(c, "累积去重（可选）").FontSize(11).Bold()
	ui.Row(c).Gap(8).Children(func() {
		ui.TextInput(c, &lp.DedupVar).Placeholder("allRows").Label("去重数组变量").Grow(1)
		ui.TextInput(c, &lp.DedupField).Placeholder("link").Label("去重字段").Grow(1)
		ui.TextInput(c, &lp.DedupStat).Placeholder("dupCount").Label("重复数变量").Grow(1)
	})
	fieldHint(c, t, "循环结束后对「去重数组变量」按「去重字段」去重（字段为空按整行），去掉的重复数写入日志；填了「重复数变量」也会写入该变量。留空去重数组变量则不启用。")

	// 健壮性（P2）：限速 + 空页检测/重试。
	ui.Divider(c)
	ui.Text(c, "健壮性（可选）").FontSize(11).Bold()
	ui.Row(c).Gap(8).Children(func() {
		intInput(c, &lp.DelayMs, "间隔 ms", "0", 100)
		ui.TextInput(c, &lp.MinRowsVar).Placeholder("pageRows").Label("空页检测变量").Grow(1)
		intInput(c, &lp.MinRows, "最少行", "0", 80)
		intInput(c, &lp.RetryOnEmpty, "空页重试", "0", 80)
	})
	fieldHint(c, t, "间隔 ms：每轮前等待，限速防反爬；空页检测变量：本轮该数组少于「最少行」即视为抓空，按「空页重试」次数重跑本轮并记日志。")

	ui.Divider(c)
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
				if child.Args == nil {
					child.Args = map[string]string{}
				}
				argsText := mapToString(child.Args)
				if ui.TextArea(c, &argsText).Height(60).Changed() {
					child.Args = stringToMap(argsText, "=")
				}
			}
		})
	}
}

// processOps 所有 process 步骤统一的处理操作列表，避免编排/子步骤下拉不一致。

// processOps 所有 process 步骤统一的处理操作列表，避免编排/子步骤下拉不一致。
func processOps() []string {
	return []string{"set", "append", "json_to_table", "filter", "sort", "pick", "flatten", "dedup",
		"zip", "limit", "resolve_url", "format_time", "join", "base64encode", "base64decode", "urlencode", "urldecode"}
}

func (a *App) editorProcess(c *ui.Context, t *ui.Theme, step *model.Step) {
	ops := processOps()
	op := step.Op
	if op == "" {
		op = "json_to_table"
	}
	if ui.Select(c, &op, ops).Label("处理操作").Width(180).Changed() {
		step.Op = op
	}
	hints := map[string]string{
		"set":           "设置变量：name=变量名, value=值",
		"append":        "循环内累积：from=源变量, to=数组变量（每轮追加一项）",
		"json_to_table": "JSON 数组转表格：from=源变量, to=目标变量",
		"filter":        "过滤：from, field, op(eq/ne/contains/gt...), value, to",
		"sort":          "排序：from, field, order(asc/desc), to",
		"pick":          "取字段：from, fields(逗号分隔), to",
		"flatten":       "合并数组：from(逗号分隔的多个变量), to",
		"dedup":         "去重：from=源数组变量, to=目标变量；field 按对象字段去重，stat 写入去掉的重复数",
		"zip":           "按索引合成对象数组：from=titles,links, fields=title,url, to=rows（免写 JS）",
		"limit":         "取数组区间：from=源数组, count=条数(或 start/end), to=目标（取前 N 条免写 JS）",
		"resolve_url":   "URL 绝对化+跳转解码：from=源变量, base=基准地址(可选), to=目标",
		"format_time":   "时间格式化：from, in(unix/unixms), layout, to",
		"join":          "拼接：values(a,b,c), sep, to",
		"base64encode":  "Base64 编码：from=源变量, to=目标变量",
		"base64decode":  "Base64 解码：from=源变量, to=目标变量",
		"urlencode":     "URL 编码：from=源变量, to=目标变量",
		"urldecode":     "URL 解码：from=源变量, to=目标变量",
	}
	fieldHint(c, t, "参数（每行 key=value，支持 {{变量}}）")
	if h, ok := hints[op]; ok {
		fieldHint(c, t, "「"+op+"」："+h)
	}
	if step.Args == nil {
		step.Args = map[string]string{}
	}
	argsText := mapToString(step.Args)
	if ui.TextArea(c, &argsText).Height(120).Changed() {
		step.Args = stringToMap(argsText, "=")
	}
}
