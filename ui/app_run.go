package ui

import (
	"fmt"
	"path/filepath"

	"github.com/egoist/mygo"

	"requestflow/engine"
	"requestflow/report"
)

// runDataDriven 用选中的数据集逐行运行场景，结果展示在运行结果标签。
func (a *App) runDataDriven() {
	if a.dataFile == "" || a.running || a.dataRunning {
		return
	}
	rows, err := engine.LoadDataset(a.dataFile)
	if err != nil {
		a.log("[数据驱动] 加载失败: " + err.Error())
		mygo.Dialog.Error("数据文件加载失败", err.Error())
		return
	}
	a.dataRunning = true
	a.dataResults = nil
	a.resetResult()
	a.mainTab = 2
	a.log(fmt.Sprintf("=== 数据驱动开始: %s (%d 行) ===", filepath.Base(a.dataFile), len(rows)))
	sc := cloneScenario(a.scenario)
	go func() {
		results := engine.RunDataset(sc, rows, func(ie engine.IterationEvent) {
			msg := ie.Event.Message
			if msg == "" {
				return
			}
			mygo.RunOnMain(func() {
				a.log(fmt.Sprintf("[迭代%d] %s", ie.Iteration, msg))
			})
		})
		mygo.RunOnMain(func() {
			a.dataRunning = false
			a.dataResults = results
			passed, failed := 0, 0
			for _, r := range results {
				if r.Result != nil && r.Result.OK {
					passed++
				} else {
					failed++
				}
				status := "成功"
				if r.Result == nil || !r.Result.OK {
					status = "失败"
				}
				a.log(fmt.Sprintf("迭代 %d/%d: %s", r.Index, len(results), status))
			}
			a.log(fmt.Sprintf("=== 数据驱动完成: 通过 %d / 失败 %d ===", passed, failed))
			// 保留最后一次迭代结果，供结果表/变量视图与报告使用
			if len(results) > 0 {
				a.lastResult = results[len(results)-1].Result
				a.runOK = a.lastResult != nil && a.lastResult.OK
			}
		})
	}()
}

// exportReport 导出测试报告（HTML）。数据驱动时导出全部迭代汇总。

// exportReport 导出测试报告（HTML）。数据驱动时导出全部迭代汇总。
func (a *App) exportReport() {
	var rd *report.Data
	if len(a.dataResults) > 0 {
		rd = report.NewFromDataset(a.dataResults)
	} else if a.lastResult != nil {
		rd = report.NewFromResult(a.lastResult)
	} else {
		return
	}
	go func() {
		def := "requestflow-report.html"
		if rd.Scenario != "" {
			def = rd.Scenario + "-报告.html"
		}
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Title:       "导出测试报告",
			DefaultPath: def,
			ButtonLabel: "保存",
			Filters:     []mygo.FileFilter{{Name: "HTML 报告", Extensions: []string{"html"}}},
		})
		if err != nil || path == "" {
			return
		}
		if err := report.Save(rd, path); err != nil {
			mygo.RunOnMain(func() {
				mygo.Dialog.Error("导出失败", err.Error())
				a.log("[报告] 导出失败: " + err.Error())
			})
			return
		}
		mygo.RunOnMain(func() {
			a.log("[报告] 已导出: " + path)
		})
	}()
}

func (a *App) run() {
	a.running = true
	a.runOK = false
	a.dataResults = nil
	a.resetResult()
	a.log("=== 开始执行: " + a.scenario.Name + " ===")
	sc := cloneScenario(a.scenario)
	go func() {
		eng := engine.NewEngine(sc, func(ev engine.Event) {
			msg := ev.Message
			if msg == "" {
				return
			}
			switch ev.Type {
			case "step-start":
				msg = "▶ " + ev.StepName
			case "step-done":
				msg = "  ✓ " + ev.StepName
			case "step-error":
				msg = "  ✗ " + ev.StepName + ": " + ev.Message
			case "loop-iter":
				msg = "  ↻ " + msg
			}
			mygo.RunOnMain(func() { a.log(msg) })
		})
		a.setEngine(eng)
		res := eng.Run()
		mygo.RunOnMain(func() {
			a.running = false
			a.setEngine(nil)
			a.lastResult = res
			a.runOK = res.OK
			if res.OK {
				a.log("=== 执行成功 ===")
			} else {
				a.log("=== 执行失败: " + resultErrText(res) + " ===")
			}
		})
	}()
}

// setEngine / getEngine 并发安全地访问正在运行的引擎。

// setEngine / getEngine 并发安全地访问正在运行的引擎。
func (a *App) setEngine(eng *engine.Engine) {
	a.engMu.Lock()
	a.currentEng = eng
	a.engMu.Unlock()
}

func (a *App) getEngine() *engine.Engine {
	a.engMu.RLock()
	defer a.engMu.RUnlock()
	return a.currentEng
}

// resultErrText 从执行结果安全地取错误文本；Err 为 nil 或结果为 nil 时给出兜底。

// resultErrText 从执行结果安全地取错误文本；Err 为 nil 或结果为 nil 时给出兜底。
func (a *App) resultErrText(res *engine.Result) string { return resultErrText(res) }

func resultErrText(res *engine.Result) string {
	if res == nil {
		return ""
	}
	if res.Err != nil {
		return res.Err.Error()
	}
	if !res.OK {
		return "未知错误"
	}
	return ""
}

func (a *App) stop() {
	if eng := a.getEngine(); eng != nil {
		eng.Stop()
		a.log("已请求停止（将在当前步骤后中断）")
	}
}
