package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"requestflow/engine"
	"requestflow/logger"
	"requestflow/model"
	"requestflow/report"
)

// runHeadless 无界面运行场景，便于 CI / 命令行自动化。
// dataFile 非空时启用数据驱动（每行跑一遍）；reportPath 非空时导出测试报告。
func runHeadless(path, dataFile, reportPath string) {
	logger.Printf("无界面运行场景: %s", path)
	sc := loadScenario(path)
	// 请求字段内联在步骤中，是唯一真相源，直接运行即可。
	emit := func(ev engine.Event) {
		if ev.Message != "" {
			logger.Printf("[%s] %s: %s", ev.Type, ev.StepName, ev.Message)
		}
	}

	var singleRes *engine.Result
	var datasetRes []engine.DatasetResult
	allOK := true
	if dataFile != "" {
		datasetRes, allOK = runDataset(&sc, dataFile)
	} else {
		singleRes = engine.NewEngine(&sc, emit).Run()
		allOK = singleRes.OK
		b, _ := json.MarshalIndent(singleRes.Vars, "", "  ")
		fmt.Println("执行成功，最终变量:")
		fmt.Println(string(b))
	}

	if reportPath != "" {
		exportHeadlessReport(reportPath, dataFile != "", singleRes, datasetRes)
	}

	if !allOK {
		if singleRes != nil && singleRes.Err != nil {
			logger.Error("执行失败: %v", singleRes.Err)
		} else {
			logger.Error("执行失败: 存在失败的迭代")
		}
		os.Exit(1)
	}
	logger.Printf("场景 %q 执行完成", sc.Name)
}

// loadScenario 读取并解析场景 JSON，失败时记录日志并退出。
func loadScenario(path string) model.Scenario {
	data, err := os.ReadFile(path)
	if err != nil {
		logger.Error("读取场景失败: %v", err)
		os.Exit(1)
	}
	var sc model.Scenario
	if err := json.Unmarshal(data, &sc); err != nil {
		logger.Error("解析场景失败: %v", err)
		os.Exit(1)
	}
	return sc
}

// runDataset 数据驱动运行：每行一次，返回迭代结果与是否全部成功。
func runDataset(sc *model.Scenario, dataFile string) ([]engine.DatasetResult, bool) {
	rows, err := engine.LoadDataset(dataFile)
	if err != nil {
		logger.Error("加载数据文件失败: %v", err)
		fmt.Fprintln(os.Stderr, "加载数据文件失败:", err)
		os.Exit(1)
	}
	logger.Printf("数据驱动: 从 %s 加载 %d 行", dataFile, len(rows))
	res := engine.RunDataset(sc, rows, func(ie engine.IterationEvent) {
		if ie.Event.Message != "" {
			logger.Printf("[迭代%d][%s] %s: %s", ie.Iteration, ie.Event.Type, ie.Event.StepName, ie.Event.Message)
		}
	})
	allOK := true
	for _, r := range res {
		if r.Result == nil || !r.Result.OK {
			allOK = false
		}
		status := "成功"
		if r.Result == nil || !r.Result.OK {
			status = "失败"
		}
		fmt.Printf("迭代 %d/%d: %s\n", r.Index, len(res), status)
	}
	return res, allOK
}

// exportHeadlessReport 根据单次/数据驱动模式导出报告。
func exportHeadlessReport(path string, dataset bool, singleRes *engine.Result, datasetRes []engine.DatasetResult) {
	var rd *report.Data
	if dataset {
		rd = report.NewFromDataset(datasetRes)
	} else {
		rd = report.NewFromResult(singleRes)
	}
	if err := report.Save(rd, path); err != nil {
		logger.Error("导出报告失败: %v", err)
		fmt.Fprintln(os.Stderr, "导出报告失败:", err)
		os.Exit(1)
	}
	fmt.Printf("测试报告已导出: %s (通过 %d / 失败 %d)\n", path, rd.Passed, rd.Failed)
	logger.Printf("报告已导出 %s: 通过%d 失败%d", path, rd.Passed, rd.Failed)
}
