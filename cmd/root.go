// Package cmd 聚合命令行入口：参数解析、无界面运行、图形界面启动分发。
package cmd

import (
	"fmt"
	"os"

	"requestflow/logger"
	"requestflow/version"
)

// Execute 解析命令行参数并分发到对应执行路径，是 main 的唯一入口。
func Execute() {
	opts := parseFlags()

	logger.Init(opts.logDir)
	defer logger.Close()
	logger.Printf("RequestFlow 启动 %s (commit %s)", version.Short(), version.Commit)

	switch {
	case opts.version:
		fmt.Println(version.String())
	case opts.runFile != "":
		runHeadless(opts.runFile, opts.dataFile, opts.reportPath)
	case opts.dataFile != "" || opts.reportPath != "":
		fmt.Fprintln(os.Stderr, "-data / -report 需与 -run 一起使用，例如: requestflow -run scenario.json -data users.csv -report report.html")
		os.Exit(1)
	default:
		launchGUI(opts.software)
	}
}

// options 保存解析后的命令行参数。
type options struct {
	runFile    string
	dataFile   string
	reportPath string
	version    bool
	logDir     string
	software   bool
}
