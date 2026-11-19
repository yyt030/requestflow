package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"requestflow/engine"
	"requestflow/logger"
	"requestflow/model"
	"requestflow/report"
	"requestflow/ui"
	"requestflow/version"
)

func main() {
	runFile := flag.String("run", "", "以无界面模式运行指定场景 JSON")
	dataFile := flag.String("data", "", "数据驱动：CSV/JSON 数据文件，每行运行一次场景")
	reportPath := flag.String("report", "", "导出测试报告（.html 或 .json），可与 -run/-data 同用")
	showVersion := flag.Bool("version", false, "显示版本号并退出")
	logDir := flag.String("logdir", "", "指定日志目录（默认写可执行文件同目录）")
	software := flag.Bool("software", false, "强制软件渲染（Linux 图形异常/卡死时尝试）")
	flag.Parse()

	// 日常运行默认记录日志（GUI 与无界面模式共用）。
	logger.Init(*logDir)
	defer logger.Close()
	logger.Printf("RequestFlow 启动 %s (commit %s)", version.Short(), version.Commit)

	if *showVersion {
		fmt.Println(version.String())
		return
	}

	if *runFile != "" {
		runHeadless(*runFile, *dataFile, *reportPath)
		return
	}
	// 指定 -data/-report 但没给 -run 时，给出明确提示。
	if *dataFile != "" || *reportPath != "" {
		fmt.Fprintln(os.Stderr, "-data / -report 需与 -run 一起使用，例如: requestflow -run scenario.json -data users.csv -report report.html")
		os.Exit(1)
	}

	// 进入图形界面前：Linux 下做图形环境预检 + 软件渲染兜底，避免无 display
	// 时静默卡住，或 WebKitGPU 在特定驱动上拖死整个桌面。
	// CLI 模式（-version/-run）不调用，保证终端输出可见。
	hideConsole()
	prepareGUI(*software)
	if err := guiAvailable(); err != nil {
		fmt.Fprintln(os.Stderr, "无法启动图形界面:", err)
		logger.Error("图形环境检测失败: %v", err)
		os.Exit(1)
	}
	ui.Run()
}

// runHeadless 无界面运行，便于 CI / 命令行自动化。
// dataFile 非空时启用数据驱动（每行跑一遍）；reportPath 非空时导出测试报告。
func runHeadless(path, dataFile, reportPath string) {
	logger.Printf("无界面运行场景: %s", path)
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
	emit := func(ev engine.Event) {
		if ev.Message != "" {
			logger.Printf("[%s] %s: %s", ev.Type, ev.StepName, ev.Message)
		}
	}

	var singleRes *engine.Result
	var datasetRes []engine.DatasetResult
	allOK := true
	if dataFile != "" {
		rows, err := engine.LoadDataset(dataFile)
		if err != nil {
			logger.Error("加载数据文件失败: %v", err)
			fmt.Fprintln(os.Stderr, "加载数据文件失败:", err)
			os.Exit(1)
		}
		logger.Printf("数据驱动: 从 %s 加载 %d 行", dataFile, len(rows))
		datasetRes = engine.RunDataset(&sc, rows, func(ie engine.IterationEvent) {
			if ie.Event.Message != "" {
				logger.Printf("[迭代%d][%s] %s: %s", ie.Iteration, ie.Event.Type, ie.Event.StepName, ie.Event.Message)
			}
		})
		for _, r := range datasetRes {
			if r.Result == nil || !r.Result.OK {
				allOK = false
			}
			status := "成功"
			if r.Result == nil || !r.Result.OK {
				status = "失败"
			}
			fmt.Printf("迭代 %d/%d: %s\n", r.Index, len(datasetRes), status)
		}
	} else {
		singleRes = engine.NewEngine(&sc, emit).Run()
		allOK = singleRes.OK
		b, _ := json.MarshalIndent(singleRes.Vars, "", "  ")
		fmt.Println("执行成功，最终变量:")
		fmt.Println(string(b))
	}

	// 导出报告
	if reportPath != "" {
		var rd *report.Data
		if dataFile != "" {
			rd = report.NewFromDataset(datasetRes)
		} else {
			rd = report.NewFromResult(singleRes)
		}
		if err := report.Save(rd, reportPath); err != nil {
			logger.Error("导出报告失败: %v", err)
			fmt.Fprintln(os.Stderr, "导出报告失败:", err)
			os.Exit(1)
		}
		fmt.Printf("测试报告已导出: %s (通过 %d / 失败 %d)\n", reportPath, rd.Passed, rd.Failed)
		logger.Printf("报告已导出 %s: 通过%d 失败%d", reportPath, rd.Passed, rd.Failed)
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
