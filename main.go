package main

import (
	"flag"
	"fmt"
	"os"

	"requestflow/engine"
	"requestflow/model"
	"requestflow/ui"
	"requestflow/version"
	"encoding/json"
)

func main() {
	runFile := flag.String("run", "", "以无界面模式运行指定场景 JSON")
	showVersion := flag.Bool("version", false, "显示版本号并退出")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.String())
		return
	}

	if *runFile != "" {
		runHeadless(*runFile)
		return
	}
	ui.Run()
}

// runHeadless 无界面运行，便于 CI / 命令行自动化。
func runHeadless(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "读取场景失败:", err)
		os.Exit(1)
	}
	var sc model.Scenario
	if err := json.Unmarshal(data, &sc); err != nil {
		fmt.Fprintln(os.Stderr, "解析场景失败:", err)
		os.Exit(1)
	}
	res := engine.NewEngine(&sc, func(ev engine.Event) {
		if ev.Message != "" {
			fmt.Printf("[%s] %s: %s\n", ev.Type, ev.StepName, ev.Message)
		}
	}).Run()
	if !res.OK {
		fmt.Fprintln(os.Stderr, "执行失败:", res.Err)
		os.Exit(1)
	}
	b, _ := json.MarshalIndent(res.Vars, "", "  ")
	fmt.Println("执行成功，最终变量:")
	fmt.Println(string(b))
}
