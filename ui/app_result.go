package ui

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"requestflow/engine"
)

// ---------- 底部：日志 / 结果表 / 变量 三视图 ----------
func (a *App) console(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Padding(8, 10).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Segmented(c, &a.consoleView, "日志", "结果表", "变量")
			ui.Spacer(c)
			if len(a.dataResults) > 0 {
				passed, failed := 0, 0
				for _, r := range a.dataResults {
					if r.Result != nil && r.Result.OK {
						passed++
					} else {
						failed++
					}
				}
				ui.Text(c, fmt.Sprintf("数据驱动 %d 轮 · %d 通过 / %d 失败", len(a.dataResults), passed, failed)).FontSize(11).TextColor(t.Accent)
			} else if a.lastResult != nil {
				label := "✓ 成功"
				if !a.lastResult.OK {
					label = "✗ 失败"
				}
				ui.Text(c, label).FontSize(11).TextColor(t.Accent)
			}
		})
		switch a.consoleView {
		case 1:
			a.resultView(c, t)
		case 2:
			a.variablesView(c, t)
		default:
			a.logView(c, t)
		}
	})
}

func (a *App) logView(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Padding(0, 10, 4, 10).Children(func() {
		if ui.Button(c, "清空日志").Clicked() {
			a.logs = nil
		}
	})
	ui.Scroll(c).Grow(1).Padding(0, 12, 8, 12).Children(func() {
		for _, line := range a.logs {
			ui.Text(c, line).FontSize(11).SingleLine()
		}
	})
}

// tabularVars 返回当前结果里可作为表格展示的变量名（有序）。

// tabularVars 返回当前结果里可作为表格展示的变量名（有序）。
func (a *App) tabularVars() []string {
	if a.lastResult == nil {
		return nil
	}
	var names []string
	for k, v := range a.lastResult.Vars {
		if engine.IsTabular(v) {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	return names
}

func (a *App) resultView(c *ui.Context, t *ui.Theme) {
	names := a.tabularVars()
	if len(names) == 0 {
		emptyState(c, t, "暂无可展示的表格数据", "运行场景后，从响应提取的数组/JSON 数据会在此以表格显示")
		return
	}
	// 选中变量有效性检查
	if a.resultVar == "" || !containsStr(names, a.resultVar) {
		a.resultVar = names[0]
	}

	var rows [][]string
	var cols []string
	if a.lastResult != nil {
		cols, rows, _ = engine.BuildTable(a.lastResult.Vars[a.resultVar])
	}

	ui.Column(c).Fill().Padding(0, 10, 6, 10).Gap(6).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "数据源").FontSize(11).TextColor(t.TextMuted)
			sel := a.resultVar
			if ui.Select(c, &sel, names).Width(180).Changed() {
				a.resultVar = sel
				a.resultState = ui.ListState{}
			}
			ui.Spacer(c)
			ui.Text(c, fmt.Sprintf("%d 行 × %d 列", len(rows), len(cols))).FontSize(11).TextColor(t.TextMuted)
			if len(cols) > 0 {
				colOpts := append([]string{"全部列"}, cols...)
				ci := a.resultCopyCol + 1
				if ci < 0 {
					ci = 0
				}
				cs := colOpts[ci]
				if ui.Select(c, &cs, colOpts).Width(110).Tooltip("选择要复制的列").Changed() {
					idx := indexOfStr(colOpts, cs)
					a.resultCopyCol = idx - 1
				}
				if ui.Button(c, "⧉ 复制").Tooltip("把选中列复制到剪贴板（全部列=TSV）").Clicked() {
					text := resultColumnText(rows, cols, a.resultCopyCol)
					_ = mygo.Clipboard.Write(transfer.New(transfer.NewItem(transfer.Bytes(transfer.Text, []byte(text)))))
					a.log("已复制结果列到剪贴板（" + fmt.Sprint(len(rows)) + " 行）")
				}
				if ui.Button(c, "⤓ CSV").Tooltip("导出当前结果为 CSV 文件").Clicked() {
					a.exportResultCSV(cols, rows)
				}
			}
			if ui.Button(c, "刷新").Clicked() {
				a.resultState = ui.ListState{}
			}
		})
		if len(cols) == 0 {
			ui.Text(c, "该变量是标量数组，导出时会生成单列表。").FontSize(11).TextColor(t.TextMuted)
			return
		}
		tableCols := make([]ui.TableColumn, len(cols))
		for i, h := range cols {
			tableCols[i] = ui.TableColumn{Title: h, Width: 140, Sortable: true}
		}
		ui.Table(c, &a.resultState, tableCols, len(rows), func(row, col int) {
			if row >= 0 && row < len(rows) && col >= 0 && col < len(rows[row]) {
				ui.Text(c, rows[row][col]).FontSize(11).SingleLine()
			}
		}).Grow(1)
	})
}

// indexOfStr 返回 s 在 list 中的下标，不存在返回 -1。

// indexOfStr 返回 s 在 list 中的下标，不存在返回 -1。
func indexOfStr(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}

// resultColumnText 把结果表某列拼成多行文本；col=-1 表示所有列以 TSV 输出。

// resultColumnText 把结果表某列拼成多行文本；col=-1 表示所有列以 TSV 输出。
func resultColumnText(rows [][]string, cols []string, col int) string {
	lines := make([]string, 0, len(rows))
	for _, r := range rows {
		if col >= 0 && col < len(r) {
			lines = append(lines, r[col])
		} else if col < 0 {
			lines = append(lines, strings.Join(r, "\t"))
		} else {
			lines = append(lines, "")
		}
	}
	if len(lines) == 0 && len(cols) > 0 && col >= 0 && col < len(cols) {
		// 列存在但没有数据行时，至少复制表头，便于用户确认列含义。
		return ""
	}
	return strings.Join(lines, "\n")
}

// resultCSV 把列名与数据行编码为 CSV 文本（含表头），字段按 CSV 规则加引号转义。

// resultCSV 把列名与数据行编码为 CSV 文本（含表头），字段按 CSV 规则加引号转义。
func resultCSV(cols []string, rows [][]string) string {
	var b strings.Builder
	csvCell := func(s string) string {
		if strings.ContainsAny(s, ",\"\n\r") {
			return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
		}
		return s
	}
	for i, h := range cols {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(csvCell(h))
	}
	b.WriteByte('\n')
	for _, r := range rows {
		for i := range cols {
			if i > 0 {
				b.WriteByte(',')
			}
			if i < len(r) {
				b.WriteString(csvCell(r[i]))
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// exportResultCSV 弹出保存对话框，把当前结果表导出为 CSV。

// exportResultCSV 弹出保存对话框，把当前结果表导出为 CSV。
func (a *App) exportResultCSV(cols []string, rows [][]string) {
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{DefaultPath: "result.csv"})
	if err != nil || path == "" {
		return
	}
	data := resultCSV(cols, rows)
	if err := os.WriteFile(path, []byte("\ufeff"+data), 0644); err != nil {
		a.log("导出 CSV 失败: " + err.Error())
		return
	}
	a.log("已导出 CSV: " + path + fmt.Sprintf("（%d 行）", len(rows)))
}

// variableKind 返回变量的展示类型与元素/字段数量，供变量预览表格显示。
