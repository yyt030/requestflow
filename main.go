package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"requestflow/engine"
	"requestflow/logger"
	"requestflow/model"
	"requestflow/ui"
	"requestflow/version"
)

func main() {
	runFile := flag.String("run", "", "以无界面模式运行指定场景 JSON")
	showVersion := flag.Bool("version", false, "显示版本号并退出")
	logDir := flag.String("logdir", "", "指定日志目录（默认写可执行文件同目录）")
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
		runHeadless(*runFile)
		return
	}

	// 进入图形界面：Windows 下隐藏双击 exe 时自动弹出的控制台黑框。
	// CLI 模式（-version/-run）不调用，保证终端输出可见。
	hideConsole()
	ui.Run()
}

// runHeadless 无界面运行，便于 CI / 命令行自动化。
func runHeadless(path string) {
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
	res := engine.NewEngine(&sc, func(ev engine.Event) {
		if ev.Message != "" {
			line := fmt.Sprintf("[%s] %s: %s", ev.Type, ev.StepName, ev.Message)
			logger.Printf("%s", line)
		}
	}).Run()
	if !res.OK {
		logger.Error("执行失败: %v", res.Err)
		os.Exit(1)
	}
	b, _ := json.MarshalIndent(res.Vars, "", "  ")
	fmt.Println("执行成功，最终变量:")
	fmt.Println(string(b))
	logger.Printf("场景 %q 执行成功", sc.Name)
}
