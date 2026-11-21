package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

// editorParallel 扇出编辑器：配置多个并发子任务及其产物汇合方式。
func (a *App) editorParallel(c *ui.Context, t *ui.Theme, step *model.Step) {
	if step.Parallel == nil {
		step.Parallel = &model.Parallel{Mode: "table", Var: "merged"}
	}
	p := step.Parallel
	ui.Text(c, "扇出：多个子任务并发执行（类似 Go 的 goroutine），全部完成后汇合产物。后续用【扇入合并】步骤做门控。").FontSize(10).TextColor(t.TextMuted)
	ui.Row(c).Gap(8).Children(func() {
		ui.Select(c, &p.Mode, []string{"array", "object", "table", "concat"}).Label("汇合方式").Width(140)
		ui.TextInput(c, &p.Var).Label("写入变量").Grow(1)
	})
	ui.Row(c).Gap(8).Children(func() {
		ui.TextInput(c, &p.StatusVar).Label("状态表名(供扇入引用)").Grow(1)
	})
	if p.StatusVar == "" {
		p.StatusVar = "task"
	}
	ui.Text(c, "执行后自动登记：task.<任务名>.status(success/fail) / .output / .error / .duration，以及 task.success/failed/total/allSuccess，供扇入门控表达式引用。").FontSize(10).TextColor(t.TextMuted)
	ui.Text(c, map[string]string{
		"array":  "array：各任务产物按顺序组成数组",
		"object": "object：以任务名为 key 组成对象",
		"table":  "table：各任务产物（数组）展开合并成一张表，可直接导出",
		"concat": "concat：各任务产物转为文本后拼接",
	}[defaultModeStr(p.Mode)]).FontSize(10).TextColor(t.TextMuted)
	if defaultModeStr(p.Mode) == "concat" {
		ui.TextInput(c, &p.Separator).Label("分隔符")
	}
	ui.Checkbox(c, &p.FailFast, "任一子任务失败则整体失败（默认忽略失败并继续汇合）")

	ui.Divider(c)
	for i := range p.Tasks {
		task := &p.Tasks[i]
		idx := i
		ui.Column(c.Key(fmt.Sprintf("parallel-task-%d", idx))).Padding(10).Gap(6).Background(t.Surface).Radius(8).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, fmt.Sprintf("任务 %d", idx+1)).FontSize(11).Bold().TextColor(t.Accent)
				ui.TextInput(c, &task.Name).Placeholder("任务名（object 模式作为 key）").Grow(1)
				if ui.Button(c.Key("del"), "移除").Clicked() {
					p.Tasks = append(p.Tasks[:idx], p.Tasks[idx+1:]...)
				}
			})
			ui.Row(c).Gap(8).Children(func() {
				ui.TextInput(c, &task.Var).Placeholder("产物变量名（如 rows）").Grow(1)
				ui.TextInput(c, &task.Value).Placeholder("或直接产出常量文本").Grow(1)
			})
			ui.Text(c, "子任务步骤（在独立变量沙箱中并行执行）").FontSize(10).TextColor(t.TextMuted)
			a.editorBranchChildren(c, t, step, &task.Steps, fmt.Sprintf("pt%d", idx))
		})
	}
	if ui.Button(c, "+ 添加子任务").Clicked() {
		p.Tasks = append(p.Tasks, model.ParallelTask{Name: fmt.Sprintf("task %d", len(p.Tasks)+1)})
	}
}

// editorMerge 扇入门控编辑器：用高级表达式检查上游并行任务的状态/产物，不成立即中止。

// editorMerge 扇入门控编辑器：用高级表达式检查上游并行任务的状态/产物，不成立即中止。
func (a *App) editorMerge(c *ui.Context, t *ui.Theme, step *model.Step) {
	if step.Merge == nil {
		step.Merge = &model.Merge{When: "task.allSuccess"}
	}
	m := step.Merge
	ui.Text(c, "扇入(门控)：等待上游并行任务全部完成后检查条件；条件不成立则中止整个场景。").FontSize(10).TextColor(t.TextMuted)
	ui.Text(c, "门控表达式（支持 && || 与比较；留空默认 task.allSuccess）").FontSize(11).Bold()
	ui.TextArea(c, &m.When).Placeholder(`task.allSuccess && task.订单查询.output > 0`).Height(70)

	ui.Divider(c)
	ui.Text(c, "可引用变量（点击追加到表达式末尾）").FontSize(11).Bold()
	refs := []struct{ label, token string }{
		{"全部成功", "task.allSuccess"},
		{"成功数", "task.success"},
		{"失败数", "task.failed"},
		{"任务总数", "task.total"},
		{"&&", " && "},
		{"||", " || "},
		{"==", " == "},
		{"!=", " != "},
		{">", " > "},
		{"<", " < "},
	}
	// 从上游 parallel 步骤收集任务名，提供 task.<名>.status / .output 快捷引用
	if a.scenario != nil {
		for _, s := range a.scenario.Steps {
			if s.Parallel != nil {
				for _, tk := range s.Parallel.Tasks {
					if tk.Name == "" {
						continue
					}
					refs = append(refs,
						struct{ label, token string }{tk.Name + ".status", "task." + tk.Name + ".status == \"success\""},
						struct{ label, token string }{tk.Name + ".output", "task." + tk.Name + ".output"},
					)
				}
			}
		}
	}
	ui.Row(c).Gap(6).Wrap().Children(func() {
		for _, r := range refs {
			ref := r
			if ui.Button(c.Key("ref-"+ref.label), ref.label).Clicked() {
				m.When = strings.TrimRight(m.When, " ") + ref.token
			}
		}
	})
	ui.Text(c, "示例：task.allSuccess  ·  task.A.status == \"success\" && task.B.status == \"success\"  ·  task.failed == 0  ·  len(task) >= 3").FontSize(10).TextColor(t.TextMuted)
	ui.Text(c, "判定：表达式求值为真→放行后续步骤；为假→报错中止场景。任一并行任务失败且需门控时，请勿在【扇出】步骤勾选 failFast，让失败进入状态表由本步骤裁决。").FontSize(10).TextColor(t.TextMuted)
}

func defaultModeStr(mode string) string {
	m := strings.ToLower(strings.TrimSpace(mode))
	if m == "" {
		return "array"
	}
	return m
}

func (a *App) editorScript(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "自定义脚本").FontSize(12).Bold()
		langs := []string{"js", "python"}
		lang := step.ScriptLang
		if lang == "" {
			lang = "js"
		}
		if ui.Select(c, &lang, langs).Label("语言").Changed() {
			step.ScriptLang = lang
		}
	})
	if strings.EqualFold(step.ScriptLang, "python") || strings.EqualFold(step.ScriptLang, "py") {
		fieldHint(c, t, "Python（Starlark 子集）：vars[\"k\"] = v 读写变量，last[\"status\"/\"json\"] 取响应，log() 打印。")
		fieldHint(c, t, "支持 if/for/函数/列表推导；顶层不能直接写 for（放进函数里调用），无 while/try/class。")
		fieldHint(c, t, "示例：vars[\"token\"] = last[\"json\"][\"data\"][\"token\"];  log(\"取到 token\")")
	} else {
		fieldHint(c, t, "JavaScript（goja）：vars（读写变量）、last（最近响应 status/body/json）、log()、JSON.stringify/parse。")
		fieldHint(c, t, "示例：vars.token = last.json.data.token;  log('取到 token');")
	}
	// 内容格式化：脚本通常不是结构化数据，但常被用来粘贴 JSON 样例，提供一键美化。
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if ui.Button(c, "✨ 格式化").Tooltip("若内容是 JSON/XML/键值对则一键美化").Clicked() {
			out, kind, changed := FormatContent(step.Code)
			if changed {
				step.Code = out
				a.scriptFmtMsg = "已按 " + kind + " 格式化"
			} else {
				a.scriptFmtMsg = "脚本内容无需格式化（仅支持 JSON/XML/键值对）"
			}
		}
		if a.scriptFmtMsg != "" {
			ui.Text(c, a.scriptFmtMsg).FontSize(10).TextColor(t.TextMuted)
		}
	})
	ui.TextArea(c, &step.Code).Height(240)
}

func (a *App) editorAssert(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Text(c, "独立断言（针对最近一次响应）").FontSize(12).Bold()
	a.editorAssertList(c, t, step)
}

func (a *App) editorExport(c *ui.Context, t *ui.Theme, step *model.Step) {
	fieldHint(c, t, "把提取到的数组/表格变量导出成文件；相对路径写到 exe 同目录。")
	formats := []string{"xlsx", "csv", "txt", "json"}
	f := step.Format
	if f == "" {
		f = "xlsx"
	}
	ui.Row(c).Gap(8).Children(func() {
		ui.TextInput(c, &step.From).Placeholder("table / rows").Label("数据源变量 From").Grow(1)
		if ui.Select(c, &f, formats).Label("格式").Width(110).Changed() {
			step.Format = f
		}
	})
	ui.Row(c).Gap(8).Children(func() {
		ui.TextInput(c, &step.Path).Placeholder("result.xlsx").Label("输出路径").Grow(1)
	})
	fieldHint(c, t, "xlsx/csv 生成表格，txt 生成纯文本，json 原样输出；留空路径按格式给默认文件名。")
}

// ---------- 底部：日志 / 结果表 / 变量 三视图 ----------
