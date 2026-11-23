package ui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"requestflow/logger"
	"requestflow/model"
)

func (a *App) statusBar(c *ui.Context, t *ui.Theme) {
	// 固定高度 + 不透明背景 + 分隔线：确保状态栏占据明确的一行，
	// 主工作区（尤其接口调试响应/结果表这类滚动区）不会延伸进这一行，
	// 避免返回内容的最后几行被状态栏遮挡。
	ui.Divider(c)
	ui.Row(c).Height(30).Padding(0, 12).AlignItems(ui.Center).Background(t.Surface).Children(func() {
		status := "就绪"
		failDetail := ""
		if a.running {
			status = "运行中…"
		} else if a.lastResult != nil {
			if a.lastResult.OK {
				status = fmt.Sprintf("完成 · %d 变量", len(a.lastResult.Vars))
			} else {
				failDetail = resultErrText(a.lastResult)
				status = "失败：" + failDetail
			}
		}
		if failDetail != "" {
			// 失败信息可能很长，点击状态栏弹出完整错误详情。
			if ui.Button(c, truncateRunes(status, 100)).FontSize(10).TextColor(t.Danger).
				Tooltip("点击查看完整错误").Clicked() {
				a.showErrorDetail(failDetail)
			}
		} else {
			ui.Text(c, status).FontSize(10).TextColor(t.TextMuted)
		}
		ui.Spacer(c)
		if a.filePath != "" {
			ui.Text(c, a.filePath).FontSize(10).TextColor(t.TextMuted)
		}
	})
}

// ---------- 动作 ----------

func (a *App) log(s string) {
	a.logs = append(a.logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), s))
	if len(a.logs) > 500 {
		a.logs = a.logs[len(a.logs)-500:]
	}
	// 同步落盘，日常运行的关键事件（打开/保存/执行/调试等）均可在日志文件中回看。
	logger.Printf("%s", s)
	win := a.win
	if win != nil {
		win.Invalidate()
	}
}

func (a *App) selectStep(i int) {
	a.selectStepNoSync(i)
	if win := a.win; win != nil {
		win.Invalidate()
	}
}

func (a *App) selectStepNoSync(i int) {
	a.selectedStep = i
}

// openStepDebug 双击步骤时进入调试：HTTP 请求步骤载入调试面板供查看/编辑/发送；
// 其它类型保持选中并在右侧显示其配置详情，同时给出说明。

// openStepDebug 双击步骤时进入调试：HTTP 请求步骤载入调试面板供查看/编辑/发送；
// 其它类型保持选中并在右侧显示其配置详情，同时给出说明。
func (a *App) openStepDebug(i int) {
	if i < 0 || i >= len(a.scenario.Steps) {
		return
	}
	step := &a.scenario.Steps[i]
	if step.Type == model.StepRequest {
		a.loadStepToDebug(step)
		a.log("已载入调试: " + a.stepLabel(*step) + "（可编辑后点【发送】，不自动请求）")
	} else {
		a.mainTab = 0
		a.showDebug = false
		a.log("已选中 " + a.stepLabel(*step) + "，配置详情显示在右侧编辑区")
	}
}

func (a *App) appendStep(typ model.StepType) {
	s := model.Step{ID: newID(), Name: defaultStepName(typ), Type: typ, Enabled: true}
	switch typ {
	case model.StepRequest:
		s.Method = "GET"
		s.Extract = []model.Extract{}
		s.Assert = []model.Assert{}
	case model.StepTCP:
		s.TCPPort = "8080"
		s.TCPSendMode = "line"
		s.TCPReadMode = "line"
		s.TCPTimeout = "5000"
		s.Extract = []model.Extract{}
		s.Assert = []model.Assert{}
	case model.StepLoop:
		s.Loop = &model.Loop{Mode: "until", Max: 20, PageVar: "page", PageStart: 1, PageStep: 1}
	case model.StepBranch:
		s.Branch = &model.Branch{Logic: "and", Conditions: []model.Condition{{Op: "大于"}}}
	case model.StepSwitch:
		s.Switch = &model.Switch{
			Cases: []model.SwitchCase{{Name: "case 1", Logic: "and", Conditions: []model.Condition{{Op: "等于"}}}},
		}
	case model.StepParallel:
		s.Parallel = &model.Parallel{
			Mode: "table", Var: "merged",
			Tasks: []model.ParallelTask{{Name: "task 1"}},
		}
	case model.StepMerge:
		s.Merge = &model.Merge{When: "task.allSuccess"}
	case model.StepExport:
		s.Format = "xlsx"
		s.Path = "result.xlsx"
	}
	a.scenario.Steps = append(a.scenario.Steps, s)
	a.selectedStep = len(a.scenario.Steps) - 1
}

func (a *App) moveStep(i, dir int) {
	// 边界防御：源索引与目录都必须有效。
	if i < 0 || i >= len(a.scenario.Steps) {
		return
	}
	j := i + dir
	if j < 0 || j >= len(a.scenario.Steps) {
		return
	}
	a.scenario.Steps[i], a.scenario.Steps[j] = a.scenario.Steps[j], a.scenario.Steps[i]
	if a.selectedStep == i {
		a.selectedStep = j
	} else if a.selectedStep == j {
		a.selectedStep = i
	}
}

func (a *App) deleteStep(i int) {
	// 边界防御：无效索引直接忽略，避免切片越界 panic。
	if i < 0 || i >= len(a.scenario.Steps) {
		return
	}
	a.scenario.Steps = append(a.scenario.Steps[:i], a.scenario.Steps[i+1:]...)
	if len(a.scenario.Steps) == 0 {
		a.selectedStep = -1
	} else if a.selectedStep >= len(a.scenario.Steps) {
		a.selectedStep = len(a.scenario.Steps) - 1
	}
}

// newScenario 打开模板向导，让用户从模板或空白场景开始，降低手写 JSON 门槛。
