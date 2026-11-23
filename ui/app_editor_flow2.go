package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

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
