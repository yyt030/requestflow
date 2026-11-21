package cmd

import "flag"

// parseFlags 定义并解析全部命令行参数。
func parseFlags() options {
	runFile := flag.String("run", "", "以无界面模式运行指定场景 JSON")
	dataFile := flag.String("data", "", "数据驱动：CSV/JSON 数据文件，每行运行一次场景")
	reportPath := flag.String("report", "", "导出测试报告（.html 或 .json），可与 -run/-data 同用")
	showVersion := flag.Bool("version", false, "显示版本号并退出")
	logDir := flag.String("logdir", "", "指定日志目录（默认写可执行文件同目录）")
	software := flag.Bool("software", false, "强制软件渲染（Linux 图形异常/卡死时尝试）")
	flag.Parse()
	return options{
		runFile:    *runFile,
		dataFile:   *dataFile,
		reportPath: *reportPath,
		version:    *showVersion,
		logDir:     *logDir,
		software:   *software,
	}
}
