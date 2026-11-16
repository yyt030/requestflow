// Package ui 基于 mygo 原生 UI 实现场景可视化配置与一键执行。
package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"requestflow/engine"
	"requestflow/model"
	"requestflow/version"
)

// App 保存界面所有可变状态。view 是纯函数：从状态构建界面。
type App struct {
	win     *mygo.Window
	scenario *model.Scenario
	filePath string

	selectedStep int
	logs         []string
	running      bool
	runOK        bool
	lastResult   *engine.Result
	currentEng   *engine.Engine

	// 编辑器临时缓冲（避免每帧写入模型）
	editName   string
	editMethod string
	editURL    string
	editBody   string

	// 步骤列表拖拽
	listState ui.ListState
	splitSide float32
	splitLog  float32

	// "新建步骤"菜单
	adding bool

	// 底部控制台视图：0=日志 1=结果表 2=变量
	consoleView int
	// 结果表：选中的数组变量名与表格状态
	resultVar    string
	resultState  ui.ListState
	resultCols   []ui.TableColumn
	// 全局变量编辑
	showVars    bool
	newVarKey   string
	newVarValue string

	// cURL 导入面板
	showImport bool
	importText string
	importMsg  string
}

// Run 启动 GUI。
func Run() {
	app := &App{
		scenario:     demoScenario(),
		selectedStep: -1,
		splitSide:    280,
		splitLog:     220,
	}
	app.logs = append(app.logs, "就绪。打开或新建场景，配置步骤后点击【一键运行】。")
	mygo.App.WhenReady(func() {
		app.win = mygo.NewWindow(mygo.WindowOptions{
			Title:  "RequestFlow " + version.Short() + " · HTTP 流程自动化",
			Width:  1180,
			Height: 780,
			Content: ui.View(app.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func (a *App) view(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Children(func() {
		a.toolbar(c, t)
		ui.Divider(c)
		ui.Split(c, &a.splitSide, func() { a.sidebar(c, t) }, func() {
			ui.SplitVertical(c, &a.splitLog, func() { a.editor(c, t) }, func() { a.console(c, t) }).Fill()
		}).Grow(1)
		a.statusBar(c, t)
	})
}

// ---------- 顶部工具栏 ----------
func (a *App) toolbar(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Gap(8).Padding(10, 12).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "RequestFlow").FontSize(16).Bold()
		ui.Text(c, "HTTP 流程自动化").FontSize(11).TextColor(t.TextMuted)
		ui.Spacer(c)
		if ui.Button(c, "新建").Clicked() {
			a.newScenario()
		}
		if ui.Button(c, "打开").Clicked() {
			a.openScenario()
		}
		if ui.Button(c, "保存").Clicked() {
			a.saveScenario()
		}
		ui.Divider(c).Height(20)
		if ui.Button(c, "⇥ 导入 cURL").Clicked() {
			a.showImport = !a.showImport
			a.importMsg = ""
		}
		if ui.PrimaryButton(c, "▶ 一键运行").Disabled(a.running).Clicked() {
			a.run()
		}
		if ui.Button(c, "■ 停止").Disabled(!a.running).Clicked() {
			a.stop()
		}
		ui.Divider(c).Height(20)
		if ui.Button(c, "ⓘ "+version.Short()).Clicked() {
			a.showAbout()
		}
	})
}

// showAbout 弹出"关于"对话框展示完整版本信息。
// Dialog.Message 会阻塞，因此放到独立 goroutine 中调用。
func (a *App) showAbout() {
	go func() {
		_, _ = mygo.Dialog.Message(mygo.MessageOptions{
			Parent:  a.win,
			Type:    mygo.MessageInfo,
			Title:   "关于 RequestFlow",
			Message: version.String(),
			Detail:  version.Detail(),
			Buttons: []string{"确定"},
		})
	}()
}

// ---------- 左侧：场景信息 + 步骤列表 ----------
func (a *App) sidebar(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Children(func() {
		ui.Column(c).Padding(12).Gap(6).Children(func() {
			ui.Text(c, "场景").FontSize(11).Bold().TextColor(t.TextMuted)
			ui.TextInput(c, &a.scenario.Name).Label("名称")
			ui.TextInput(c, &a.scenario.BaseURL).Placeholder("https://api.example.com").Label("Base URL")
		})
		ui.Divider(c)
		// 全局变量区（可折叠）
		ui.Column(c).Padding(12, 8).Gap(6).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				if ui.Button(c, map[bool]string{true: "▾", false: "▸"}[a.showVars]+" 全局变量").Clicked() {
					a.showVars = !a.showVars
				}
				ui.Text(c, fmt.Sprintf("(%d)", len(a.scenario.Variables))).FontSize(10).TextColor(t.TextMuted)
			})
			if a.showVars {
				a.variablesEditor(c, t)
			}
		})
		ui.Divider(c)
		ui.Row(c).Padding(12, 8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmt.Sprintf("步骤 (%d)", len(a.scenario.Steps))).FontSize(11).Bold().TextColor(t.TextMuted)
			ui.Spacer(c)
			if ui.Button(c, "+ 添加").Clicked() {
				a.adding = !a.adding
			}
		})
		if a.adding {
			a.addMenu(c, t)
		}
		ui.Scroll(c).Grow(1).Padding(0, 8, 8, 8).Children(func() {
			a.stepList(c, t)
		})
	})
}

// variablesEditor 可视化编辑全局变量（key=value），直接读写 scenario.Variables。
func (a *App) variablesEditor(c *ui.Context, t *ui.Theme) {
	if a.scenario.Variables == nil {
		a.scenario.Variables = map[string]string{}
	}
	keys := make([]string, 0, len(a.scenario.Variables))
	for k := range a.scenario.Variables {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	ui.Column(c).Gap(4).Children(func() {
		ui.Text(c, "用 {{key}} 在步骤中引用").FontSize(10).TextColor(t.TextMuted)
		for _, k := range keys {
			key := k
			val := a.scenario.Variables[key]
			ui.Row(c.Key("var-"+key)).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Text(c, key).FontSize(11).Width(78).SingleLine()
				input := val
				if ui.TextInput(c, &input).Grow(1).Changed() {
					a.scenario.Variables[key] = input
				}
				if ui.Button(c.Key("del"), "×").Width(26).Clicked() {
					delete(a.scenario.Variables, key)
				}
			})
		}
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			ui.TextInput(c, &a.newVarKey).Placeholder("变量名").Width(84)
			ui.TextInput(c, &a.newVarValue).Placeholder("值").Grow(1)
			if ui.Button(c, "+").Width(26).Clicked() {
				k := strings.TrimSpace(a.newVarKey)
				if k != "" {
					a.scenario.Variables[k] = a.newVarValue
					a.newVarKey = ""
					a.newVarValue = ""
				}
			}
		})
	})
}

func (a *App) addMenu(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Padding(8, 12).Gap(4).Background(t.Surface).Radius(8).Children(func() {
		ui.Text(c, "选择步骤类型").FontSize(10).TextColor(t.TextMuted)
		types := []struct {
			label string
			typ   model.StepType
		}{
			{"HTTP 请求", model.StepRequest},
			{"循环 / 翻页", model.StepLoop},
			{"内置处理", model.StepProcess},
			{"自定义脚本", model.StepScript},
			{"独立断言", model.StepAssert},
			{"导出结果", model.StepExport},
		}
		for _, item := range types {
			it := item
			if ui.Button(c, "+ "+it.label).Clicked() {
				a.appendStep(it.typ)
				a.adding = false
			}
		}
	})
}

func (a *App) stepList(c *ui.Context, t *ui.Theme) {
	steps := a.scenario.Steps
	if len(steps) == 0 {
		ui.Text(c, "暂无步骤，点击上方 + 添加。").FontSize(11).TextColor(t.TextMuted).Padding(8)
		return
	}
	for i := range steps {
		idx := i
		selected := idx == a.selectedStep
		ui.Row(c.Key(fmt.Sprintf("step-%d", idx))).Gap(6).Padding(6, 8).Radius(6).
			Background(map[bool]ui.Color{true: t.Accent, false: ui.Color{}}[selected]).
			Children(func() {
				ui.Checkbox(c, &a.scenario.Steps[idx].Enabled, "")
				b := ui.Button(c, a.stepLabel(steps[idx])).Grow(1)
				if selected {
					b.TextColor(t.AccentText)
				}
				if b.Clicked() {
					a.selectStep(idx)
				}
				if ui.Button(c.Key("up"), "↑").Width(28).Disabled(idx == 0).Clicked() {
					a.moveStep(idx, -1)
				}
				if ui.Button(c.Key("dn"), "↓").Width(28).Disabled(idx == len(steps)-1).Clicked() {
					a.moveStep(idx, 1)
				}
				if ui.Button(c.Key("del"), "×").Width(28).Clicked() {
					a.deleteStep(idx)
				}
			})
	}
}

func (a *App) stepLabel(s model.Step) string {
	icon := map[model.StepType]string{
		model.StepRequest: "⇄", model.StepLoop: "↻", model.StepProcess: "⚙",
		model.StepScript: "JS", model.StepAssert: "✓", model.StepExport: "⇩",
	}[s.Type]
	name := s.Name
	if name == "" {
		name = string(s.Type)
	}
	return fmt.Sprintf("%s  %s", icon, name)
}

// ---------- 中间：步骤编辑器 ----------
func (a *App) editor(c *ui.Context, t *ui.Theme) {
	// cURL 导入模式：覆盖编辑器区域
	if a.showImport {
		a.importPanel(c, t)
		return
	}
	ui.Column(c).Fill().Children(func() {
		if a.selectedStep < 0 || a.selectedStep >= len(a.scenario.Steps) {
			ui.Column(c).Fill().Center().Gap(8).Children(func() {
				ui.Text(c, "选择一个步骤进行配置").FontSize(15).Bold().TextColor(t.TextMuted)
				ui.Text(c, "内置：登录提取、HTTP 请求、循环翻页、数据处理、JS 脚本、断言、导出").FontSize(11).TextColor(t.TextMuted)
			})
			return
		}
		step := &a.scenario.Steps[a.selectedStep]
		ui.Scroll(c).Grow(1).Padding(14).Gap(10).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "步骤配置").FontSize(14).Bold()
				ui.Spacer(c)
				ui.Text(c, string(step.Type)).FontSize(11).TextColor(t.Accent)
			})
			ui.TextInput(c, &step.Name).Label("步骤名称")
			ui.Row(c).Gap(10).Children(func() {
				ui.Checkbox(c, &step.Enabled, "启用此步骤")
				ui.Checkbox(c, &step.ContinueOnFail, "失败后继续")
			})
			ui.Divider(c)
			switch step.Type {
			case model.StepRequest:
				a.editorRequest(c, t, step)
			case model.StepLoop:
				a.editorLoop(c, t, step)
			case model.StepProcess:
				a.editorProcess(c, t, step)
			case model.StepScript:
				a.editorScript(c, t, step)
			case model.StepAssert:
				a.editorAssert(c, t, step)
			case model.StepExport:
				a.editorExport(c, t, step)
			}
		})
	})
}

// importPanel 展示 cURL 粘贴与解析界面。
func (a *App) importPanel(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Children(func() {
		ui.Scroll(c).Grow(1).Padding(16).Gap(10).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "从 Chrome 导入 cURL").FontSize(15).Bold()
				ui.Spacer(c)
				if ui.Button(c, "✕ 关闭").Clicked() {
					a.showImport = false
					a.importMsg = ""
				}
			})
			ui.Text(c, "在 Chrome 开发者工具 Network 面板中右键请求 → Copy → Copy as cURL，粘贴到下方，点击解析即可生成请求步骤。").FontSize(11).TextColor(t.TextMuted)
			ui.Text(c, "支持 bash / cmd / PowerShell 三种复制格式；自动识别 Bearer Token、Cookie、查询参数与请求体。").FontSize(11).TextColor(t.TextMuted)
			ui.TextArea(c, &a.importText).Placeholder("curl 'https://api.example.com/orders?page=1' \\\n  -H 'authorization: Bearer xxx' \\\n  -H 'cookie: session=abc'").Height(200)
			if a.importMsg != "" {
				msgColor := t.Accent
				if strings.HasPrefix(a.importMsg, "✗") {
					msgColor = t.Danger
				}
				ui.Text(c, a.importMsg).FontSize(11).TextColor(msgColor)
			}
		})
		ui.Row(c).Gap(8).Padding(16, 12).AlignItems(ui.Center).Children(func() {
			if ui.PrimaryButton(c, "⇥ 解析并添加为步骤").Clicked() {
				a.doImportCurl()
			}
			if ui.Button(c, "清空").Clicked() {
				a.importText = ""
				a.importMsg = ""
			}
		})
	})
}

// doImportCurl 解析粘贴的 cURL，成功则生成一个 request 步骤追加到场景末尾。
func (a *App) doImportCurl() {
	text := strings.TrimSpace(a.importText)
	if text == "" {
		a.importMsg = "✗ 请先粘贴 cURL 命令"
		return
	}
	req, err := engine.ParseCurl(text)
	if err != nil {
		a.importMsg = "✗ 解析失败: " + err.Error()
		return
	}
	step := req.ToStep("", a.scenario.BaseURL)
	a.scenario.Steps = append(a.scenario.Steps, step)
	a.selectedStep = len(a.scenario.Steps) - 1
	a.showImport = false
	a.importText = ""
	a.importMsg = ""
	a.log("已导入 cURL 请求: " + step.Method + " " + previewURL(step.URL))
}

// previewURL 截断 URL 用于日志显示。
func previewURL(u string) string {
	if len(u) > 48 {
		return u[:48] + "…"
	}
	return u
}

func (a *App) editorRequest(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Text(c, "请求").FontSize(12).Bold()
	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE"}
	method := step.Method
	if method == "" {
		method = "GET"
	}
	if ui.Select(c, &method, methods).Label("方法").Changed() {
		step.Method = method
	}
	ui.TextInput(c, &step.URL).Placeholder("{{baseUrl}}/path 或完整 URL").Label("URL")
	ui.Text(c, "请求头 Headers（每行 key: value，支持 {{变量}}）").FontSize(10).TextColor(t.TextMuted)
	headerText := mapToString(step.Headers)
	if ui.TextArea(c, &headerText).Height(70).Changed() {
		step.Headers = stringToMap(headerText, ":")
	}
	ui.Text(c, "查询参数 Query（每行 key: value）").FontSize(10).TextColor(t.TextMuted)
	queryText := mapToString(step.Query)
	if ui.TextArea(c, &queryText).Height(70).Changed() {
		step.Query = stringToMap(queryText, ":")
	}
	ui.Text(c, "请求体 Body（支持 {{变量}}）").FontSize(10).TextColor(t.TextMuted)
	ui.TextArea(c, &step.Body).Height(100)

	ui.Divider(c)
	ui.Text(c, "认证").FontSize(12).Bold()
	authTypes := []string{"none", "bearer", "basic"}
	at := step.AuthType
	if at == "" {
		at = "none"
	}
	if ui.Select(c, &at, authTypes).Label("类型").Changed() {
		step.AuthType = at
	}
	if at == "bearer" {
		ui.TextInput(c, &step.AuthToken).Placeholder("{{token}}").Label("Bearer Token")
	} else if at == "basic" {
		ui.TextInput(c, &step.AuthUser).Label("用户名")
		ui.TextInput(c, &step.AuthPass).Password().Label("密码")
	}

	ui.Divider(c)
	a.editorExtract(c, t, step)
	a.editorAssertList(c, t, step)
}

func (a *App) editorExtract(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "提取变量").FontSize(12).Bold()
		ui.Spacer(c)
		if ui.Button(c, "+ 提取").Clicked() {
			step.Extract = append(step.Extract, model.Extract{Source: "body", Kind: "jsonpath"})
		}
	})
	for i := range step.Extract {
		ex := &step.Extract[i]
		idx := i
		ui.Column(c.Key(fmt.Sprintf("ex-%d", idx))).Padding(8).Gap(6).Background(t.Surface).Radius(8).Children(func() {
			ui.Row(c).Gap(8).Children(func() {
				ui.TextInput(c, &ex.Name).Placeholder("变量名").Grow(1).Label("变量名")
				if ui.Button(c, "移除").Clicked() {
					step.Extract = append(step.Extract[:idx], step.Extract[idx+1:]...)
				}
			})
			sources := []string{"body", "header", "cookie", "status"}
			ui.Select(c, &ex.Source, sources).Label("来源")
			kinds := []string{"jsonpath", "regex", "whole"}
			ui.Select(c, &ex.Kind, kinds).Label("方式")
			ui.TextInput(c, &ex.Path).Placeholder("data.token 或正则").Label("路径 / 表达式")
			ui.TextInput(c, &ex.Default).Placeholder("可选").Label("默认值")
		})
	}
}

func (a *App) editorAssertList(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "断言").FontSize(12).Bold()
		ui.Spacer(c)
		if ui.Button(c, "+ 断言").Clicked() {
			step.Assert = append(step.Assert, model.Assert{Source: "status", Op: "eq"})
		}
	})
	for i := range step.Assert {
		as := &step.Assert[i]
		idx := i
		ui.Column(c.Key(fmt.Sprintf("as-%d", idx))).Padding(8).Gap(6).Background(t.Surface).Radius(8).Children(func() {
			ui.Row(c).Gap(8).Children(func() {
				ui.TextInput(c, &as.Name).Placeholder("断言说明").Grow(1).Label("名称")
				if ui.Button(c, "移除").Clicked() {
					step.Assert = append(step.Assert[:idx], step.Assert[idx+1:]...)
				}
			})
			sources := []string{"status", "jsonpath", "body", "header", "variable"}
			ui.Select(c, &as.Source, sources).Label("来源")
			ops := []string{"eq", "ne", "contains", "exists", "gt", "gte", "lt", "lte"}
			ui.Select(c, &as.Op, ops).Label("操作")
			ui.TextInput(c, &as.Path).Placeholder("jsonpath / header 名 / 变量名").Label("路径")
			ui.TextInput(c, &as.Value).Placeholder("期望值").Label("期望值")
		})
	}
}

func (a *App) editorLoop(c *ui.Context, t *ui.Theme, step *model.Step) {
	if step.Loop == nil {
		step.Loop = &model.Loop{Mode: "until", Max: 20, PageVar: "page", PageStart: 1, PageStep: 1}
	}
	lp := step.Loop
	ui.Text(c, "循环模式").FontSize(12).Bold()
	modes := []string{"until", "count", "foreach"}
	ui.Select(c, &lp.Mode, modes).Label("模式")
	ui.Text(c, map[string]string{
		"until": "直到 BreakWhen 条件成立退出（翻页到空数据）",
		"count": "固定执行 Count 次",
		"foreach": "遍历 ListVar 数组的每个元素",
	}[lp.Mode]).FontSize(10).TextColor(t.TextMuted)

	switch lp.Mode {
	case "count":
		count := fmt.Sprint(lp.Count)
		if ui.TextInput(c, &count).Label("次数 Count").Changed() {
			lp.Count, _ = parseInt(count)
		}
	case "foreach":
		ui.TextInput(c, &lp.ListVar).Placeholder("rows").Label("数组变量 ListVar")
		ui.TextInput(c, &lp.ItemVar).Placeholder("item").Label("元素变量 ItemVar")
		ui.TextInput(c, &lp.IndexVar).Placeholder("index").Label("索引变量 IndexVar")
	case "until":
		ui.TextInput(c, &lp.BreakWhen).Placeholder("{{len(rows)}} == 0").Label("终止条件 BreakWhen")
	}
	ui.Row(c).Gap(8).Children(func() {
		pageVar := lp.PageVar
		if ui.TextInput(c, &pageVar).Label("页码变量").Changed() {
			lp.PageVar = pageVar
		}
		ps := fmt.Sprint(lp.PageStart)
		if ui.TextInput(c, &ps).Label("起始页").Changed() {
			lp.PageStart, _ = parseInt(ps)
		}
		pstep := fmt.Sprint(lp.PageStep)
		if ui.TextInput(c, &pstep).Label("步长").Changed() {
			lp.PageStep, _ = parseInt(pstep)
		}
	})
	max := fmt.Sprint(lp.Max)
	if ui.TextInput(c, &max).Label("最大轮数（保护）").Changed() {
		lp.Max, _ = parseInt(max)
	}

	ui.Divider(c)
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, fmt.Sprintf("循环体（%d 个子步骤）", len(step.Children))).FontSize(12).Bold()
		ui.Spacer(c)
		if ui.Button(c, "+ HTTP 请求").Clicked() {
			s := model.Step{ID: newID(), Name: "循环内请求", Type: model.StepRequest, Enabled: true, Method: "GET"}
			step.Children = append(step.Children, s)
		}
		if ui.Button(c, "+ 处理").Clicked() {
			s := model.Step{ID: newID(), Name: "循环内处理", Type: model.StepProcess, Enabled: true}
			step.Children = append(step.Children, s)
		}
	})
	for i := range step.Children {
		child := &step.Children[i]
		idx := i
		ui.Column(c.Key(fmt.Sprintf("child-%d", idx))).Padding(8).Gap(6).Background(t.Surface).Radius(8).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, a.stepLabel(*child)).Grow(1)
				if ui.Button(c, "移除").Clicked() {
					step.Children = append(step.Children[:idx], step.Children[idx+1:]...)
				}
			})
			ui.TextInput(c, &child.Name).Label("名称")
			if child.Type == model.StepRequest {
				ui.TextInput(c, &child.Method).Label("方法")
				ui.TextInput(c, &child.URL).Placeholder("{{baseUrl}}/list?page={{page}}").Label("URL")
				ui.TextInput(c, &child.Body).Label("Body")
				a.editorExtract(c, t, child)
			} else {
				ops := []string{"set", "append", "json_to_table", "filter", "sort", "pick", "flatten", "format_time"}
				op := child.Op
				if op == "" {
					op = "set"
				}
				if ui.Select(c, &op, ops).Label("处理操作").Changed() {
					child.Op = op
				}
				if child.Args == nil {
					child.Args = map[string]string{}
				}
				argsText := mapToString(child.Args)
				if ui.TextArea(c, &argsText).Height(60).Changed() {
					child.Args = stringToMap(argsText, "=")
				}
			}
		})
	}
}

func (a *App) editorProcess(c *ui.Context, t *ui.Theme, step *model.Step) {
	ops := []string{"set", "append", "json_to_table", "filter", "sort", "pick", "flatten", "format_time", "join", "base64encode", "base64decode", "urlencode", "urldecode"}
	op := step.Op
	if op == "" {
		op = "json_to_table"
	}
	if ui.Select(c, &op, ops).Label("处理操作").Changed() {
		step.Op = op
	}
	hints := map[string]string{
		"set": "设置变量：name=变量名, value=值",
		"append": "循环内累积：from=源变量, to=数组变量（每轮追加一项）",
		"json_to_table": "JSON 数组转表格：from=源变量, to=目标变量",
		"filter": "过滤：from, field, op(eq/ne/contains/gt...), value, to",
		"sort": "排序：from, field, order(asc/desc), to",
		"pick": "取字段：from, fields(逗号分隔), to",
		"flatten": "合并数组：from(逗号分隔的多个变量), to",
		"format_time": "时间格式化：from, in(unix/unixms), layout, to",
		"join": "拼接：values(a,b,c), sep, to",
	}
	if h, ok := hints[op]; ok {
		ui.Text(c, h).FontSize(10).TextColor(t.TextMuted)
	}
	if step.Args == nil {
		step.Args = map[string]string{}
	}
	argsText := mapToString(step.Args)
	if ui.TextArea(c, &argsText).Height(140).Changed() {
		step.Args = stringToMap(argsText, "=")
	}
	ui.Text(c, "每行一项 key=value，支持 {{变量}}").FontSize(10).TextColor(t.TextMuted)
}

func (a *App) editorScript(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Text(c, "JavaScript（goja）").FontSize(12).Bold()
	ui.Text(c, "可用：vars（读写变量）、last（最近响应：status/body/json）、log()、JSON.stringify/parse").FontSize(10).TextColor(t.TextMuted)
	ui.TextArea(c, &step.Code).Height(260)
}

func (a *App) editorAssert(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Text(c, "独立断言（针对最近一次响应）").FontSize(12).Bold()
	a.editorAssertList(c, t, step)
}

func (a *App) editorExport(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.TextInput(c, &step.From).Placeholder("table / rows").Label("数据源变量 From")
	formats := []string{"xlsx", "csv", "txt", "json"}
	f := step.Format
	if f == "" {
		f = "xlsx"
	}
	if ui.Select(c, &f, formats).Label("格式").Changed() {
		step.Format = f
	}
	ui.TextInput(c, &step.Path).Placeholder("result.xlsx，相对路径或绝对路径").Label("输出路径")
}

// ---------- 底部：日志 / 结果表 / 变量 三视图 ----------
func (a *App) console(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Padding(8, 10).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Segmented(c, &a.consoleView, "日志", "结果表", "变量")
			ui.Spacer(c)
			if a.lastResult != nil {
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
		ui.Column(c).Fill().Center().Gap(6).Children(func() {
			ui.Text(c, "暂无可展示的表格数据").FontSize(13).Bold().TextColor(t.TextMuted)
			ui.Text(c, "运行场景后，从响应提取的数组/JSON 数据会在此以表格显示").FontSize(11).TextColor(t.TextMuted)
		})
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

func (a *App) variablesView(c *ui.Context, t *ui.Theme) {
	ui.Scroll(c).Grow(1).Padding(0, 12, 8, 12).Gap(4).Children(func() {
		if a.lastResult == nil {
			ui.Text(c, "运行后在此查看全部变量（含步骤间传递的数据）").FontSize(11).TextColor(t.TextMuted)
			return
		}
		keys := make([]string, 0, len(a.lastResult.Vars))
		for k := range a.lastResult.Vars {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := a.lastResult.Vars[k]
			val := fmt.Sprint(v)
			if b, err := json.Marshal(v); err == nil && len(b) > 0 && b[0] == '"' {
				val = string(b[1 : len(b)-1])
			}
			if engine.IsTabular(v) && len(val) > 80 {
				val = val[:80] + "…"
			}
			ui.Row(c.Key(k)).Gap(8).Padding(2, 0).Children(func() {
				ui.Text(c, k).FontSize(11).Bold().Width(140)
				ui.Text(c, val).FontSize(11).TextColor(t.TextMuted).SingleLine().Grow(1)
			})
		}
	})
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func (a *App) statusBar(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Padding(6, 12).AlignItems(ui.Center).Children(func() {
		status := "就绪"
		if a.running {
			status = "运行中…"
		} else if a.lastResult != nil {
			if a.lastResult.OK {
				status = fmt.Sprintf("完成 · %d 变量", len(a.lastResult.Vars))
			} else {
				status = "失败：" + a.lastResult.Err.Error()
			}
		}
		ui.Text(c, status).FontSize(10).TextColor(t.TextMuted)
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

func (a *App) appendStep(typ model.StepType) {
	s := model.Step{ID: newID(), Name: defaultStepName(typ), Type: typ, Enabled: true}
	switch typ {
	case model.StepRequest:
		s.Method = "GET"
		s.Extract = []model.Extract{}
		s.Assert = []model.Assert{}
	case model.StepLoop:
		s.Loop = &model.Loop{Mode: "until", Max: 20, PageVar: "page", PageStart: 1, PageStep: 1}
	case model.StepExport:
		s.Format = "xlsx"
		s.Path = "result.xlsx"
	}
	a.scenario.Steps = append(a.scenario.Steps, s)
	a.selectedStep = len(a.scenario.Steps) - 1
}

func (a *App) moveStep(i, dir int) {
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
	a.scenario.Steps = append(a.scenario.Steps[:i], a.scenario.Steps[i+1:]...)
	if a.selectedStep >= len(a.scenario.Steps) {
		a.selectedStep = len(a.scenario.Steps) - 1
	}
}

func (a *App) newScenario() {
	a.scenario = model.NewScenario("新场景")
	a.filePath = ""
	a.selectedStep = -1
	a.resetResult()
	a.log("已新建场景")
}

// resetResult 清空上一次运行的可视化结果状态。
func (a *App) resetResult() {
	a.lastResult = nil
	a.resultVar = ""
	a.resultState = ui.ListState{}
}

func (a *App) openScenario() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Title:   "打开场景 JSON",
			Filters: []mygo.FileFilter{{Name: "JSON", Extensions: []string{"json"}}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		data, err := os.ReadFile(paths[0])
		if err != nil {
			mygo.Dialog.Error("打开失败", err.Error())
			return
		}
		var sc model.Scenario
		if err := json.Unmarshal(data, &sc); err != nil {
			mygo.Dialog.Error("解析失败", err.Error())
			return
		}
		path := paths[0]
		mygo.RunOnMain(func() {
			a.scenario = &sc
			a.filePath = path
			a.selectedStep = -1
			a.resetResult()
			a.log("已打开: " + path)
		})
	}()
}

func (a *App) saveScenario() {
	if a.filePath == "" {
		go func() {
			path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{DefaultPath: "scenario.json"})
			if err != nil || path == "" {
				return
			}
			mygo.RunOnMain(func() { a.filePath = path; a.saveTo(path) })
		}()
		return
	}
	a.saveTo(a.filePath)
}

func (a *App) saveTo(path string) {
	data, err := json.MarshalIndent(a.scenario, "", "  ")
	if err != nil {
		mygo.Dialog.Error("保存失败", err.Error())
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		mygo.Dialog.Error("保存失败", err.Error())
		return
	}
	a.log("已保存: " + path)
}

func (a *App) run() {
	a.running = true
	a.runOK = false
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
		a.currentEng = eng
		res := eng.Run()
		mygo.RunOnMain(func() {
			a.running = false
			a.currentEng = nil
			a.lastResult = res
			a.runOK = res.OK
			if res.OK {
				a.log("=== 执行成功 ===")
			} else {
				a.log("=== 执行失败: " + res.Err.Error() + " ===")
			}
		})
	}()
}

func (a *App) stop() {
	if a.currentEng != nil {
		a.currentEng.Stop()
		a.log("已请求停止（将在当前步骤后中断）")
	}
}

func cloneScenario(sc *model.Scenario) *model.Scenario {
	b, _ := json.Marshal(sc)
	var out model.Scenario
	_ = json.Unmarshal(b, &out)
	return &out
}

func demoScenario() *model.Scenario {
	sc := model.NewScenario("示例：登录 → 翻页查询 → 导出")
	sc.BaseURL = "https://httpbin.org"
	login := model.Step{
		ID: newID(), Name: "登录", Type: model.StepRequest, Enabled: true,
		Method: "POST", URL: "/post", Body: `{"user":"demo","pwd":"123456"}`,
		Extract: []model.Extract{{Name: "token", Source: "body", Kind: "jsonpath", Path: "json.user"}},
	}
	loop := model.Step{
		ID: newID(), Name: "翻页拉取", Type: model.StepLoop, Enabled: true,
		Loop: &model.Loop{Mode: "count", Count: 3, PageVar: "page", PageStart: 1, PageStep: 1, Max: 20},
		Children: []model.Step{
			{
				ID: newID(), Name: "拉取本页", Type: model.StepRequest, Enabled: true,
				Method: "GET", URL: "/get?page={{page}}",
				Extract: []model.Extract{{Name: "page", Source: "body", Kind: "jsonpath", Path: "args.page"}},
			},
			{
				ID: newID(), Name: "累积到 allRows", Type: model.StepProcess, Enabled: true,
				Op: "append", Args: map[string]string{"from": "page", "to": "allRows"},
			},
		},
	}
	exp := model.Step{
		ID: newID(), Name: "导出", Type: model.StepExport, Enabled: true,
		From: "allRows", Format: "txt", Path: "result.txt",
	}
	sc.Steps = []model.Step{login, loop, exp}
	return sc
}

// ---------- 小工具 ----------
func defaultStepName(t model.StepType) string {
	switch t {
	case model.StepRequest:
		return "HTTP 请求"
	case model.StepLoop:
		return "循环/翻页"
	case model.StepProcess:
		return "数据处理"
	case model.StepScript:
		return "自定义脚本"
	case model.StepAssert:
		return "断言"
	case model.StepExport:
		return "导出"
	}
	return string(t)
}

func newID() string { return fmt.Sprintf("%d", time.Now().UnixNano()) }

func parseInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

func mapToString(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(k)
		b.WriteString(" = ")
		b.WriteString(m[k])
	}
	return b.String()
}

func stringToMap(s, _ string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 1 {
			parts = strings.SplitN(line, ":", 2)
		}
		if len(parts) == 2 {
			out[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return out
}

var _ = filepath.Join
