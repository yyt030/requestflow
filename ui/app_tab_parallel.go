package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

// parallelTabView 按 Postman 风格标签渲染扇出步骤的单个配置维度。
func (a *App) parallelTabView(c *ui.Context, t *ui.Theme, step *model.Step, tab string) {
	if step.Parallel == nil {
		step.Parallel = &model.Parallel{Mode: "table", Var: "merged"}
	}
	p := step.Parallel
	switch tab {
	case "汇合配置":
		a.parallelMergeCfgView(c, t, p)
	case "子任务":
		a.parallelTasksView(c, t, step, p)
	}
}

func (a *App) parallelMergeCfgView(c *ui.Context, t *ui.Theme, p *model.Parallel) {
	fieldHint(c, t, "扇出：多个子任务并发执行（类似 Go 的 goroutine），全部完成后汇合产物。后续用【扇入合并】步骤做门控。")
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
	fieldHint(c, t, "执行后自动登记：task.<任务名>.status(success/fail) / .output / .error / .duration，以及 task.success/failed/total/allSuccess，供扇入门控表达式引用。")
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
}

func (a *App) parallelTasksView(c *ui.Context, t *ui.Theme, step *model.Step, p *model.Parallel) {
	fieldHint(c, t, "每个子任务在独立变量沙箱中并行执行；产物按「汇合配置」设定的方式合并。")
	removeIdx := -1 // 删除延后到循环结束，避免遍历中修改切片导致索引越界闪退
	for i := range p.Tasks {
		task := &p.Tasks[i]
		idx := i
		ui.Column(c.Key(fmt.Sprintf("parallel-task-%d", idx))).Padding(10).Gap(6).Background(t.Surface).Radius(8).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, fmt.Sprintf("任务 %d", idx+1)).FontSize(11).Bold().TextColor(t.Accent)
				ui.TextInput(c, &task.Name).Placeholder("任务名（object 模式作为 key）").Grow(1)
				if ui.Button(c.Key("del"), "移除").Clicked() {
					removeIdx = idx
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
	if removeIdx >= 0 && removeIdx < len(p.Tasks) {
		p.Tasks = append(p.Tasks[:removeIdx], p.Tasks[removeIdx+1:]...)
	}
	if ui.Button(c, "+ 添加子任务").Clicked() {
		p.Tasks = append(p.Tasks, model.ParallelTask{Name: fmt.Sprintf("task %d", len(p.Tasks)+1)})
	}
}

// mergeTabView 按 Postman 风格标签渲染扇入门控步骤的单个配置维度。
func (a *App) mergeTabView(c *ui.Context, t *ui.Theme, step *model.Step, tab string) {
	if step.Merge == nil {
		step.Merge = &model.Merge{When: "task.allSuccess"}
	}
	m := step.Merge
	switch tab {
	case "门控":
		a.mergeGateView(c, t, m)
	case "引用":
		a.mergeRefsView(c, t, m)
	}
}

func (a *App) mergeGateView(c *ui.Context, t *ui.Theme, m *model.Merge) {
	fieldHint(c, t, "扇入(门控)：等待上游并行任务全部完成后检查条件；条件不成立则中止整个场景。")
	ui.Text(c, "门控表达式（支持 && || 与比较；留空默认 task.allSuccess）").FontSize(11).Bold()
	ui.TextArea(c, &m.When).Placeholder(`task.allSuccess && task.订单查询.output > 0`).Height(90)
	fieldHint(c, t, "判定：表达式求值为真→放行后续步骤；为假→报错中止场景。任一并行任务失败且需门控时，请勿在【扇出】步骤勾选 failFast，让失败进入状态表由本步骤裁决。")
}

func (a *App) mergeRefsView(c *ui.Context, t *ui.Theme, m *model.Merge) {
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
	fieldHint(c, t, "示例：task.allSuccess  ·  task.A.status == \"success\" && task.B.status == \"success\"  ·  task.failed == 0  ·  len(task) >= 3")
}
