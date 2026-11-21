// Package ui 基于 mygo 原生 UI 实现场景可视化配置与一键执行。
package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"requestflow/engine"
	"requestflow/history"
	"requestflow/importer"
	"requestflow/logger"
	"requestflow/model"
	"requestflow/report"
	"requestflow/version"
)

// resizeGripIcon 是标准 textarea 右下角的缩放手柄（两条斜线的小三角）。
// 用 currentColor 绘制，随主题文字色变化；比底部整条横线更通用、克制，
// 与浏览器/主流编辑器的 textarea resize handle 视觉一致。
var resizeGripIcon = ui.MustParseSVG([]byte(
	`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"><path d="M14 5L5 14"/><path d="M14 10l-4 4"/></svg>`))

// 步骤列表右侧的工具按钮图标：细线箭头与垃圾桶，用 currentColor 随文字色变化。
var (
	iconMoveUp = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M8 13V3.5"/><path d="M4.5 7L8 3.5 11.5 7"/></svg>`))
	iconMoveDown = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M8 3v9.5"/><path d="M4.5 9L8 12.5 11.5 9"/></svg>`))
	iconDelete = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"><path d="M2.7 4.2h10.6"/><path d="M6.2 4.2V2.7h3.6v1.5"/><path d="M4.2 4.2l.6 8.9h6.4l.6-8.9"/><path d="M6.6 6.6v4.1M9.4 6.6v4.1"/></svg>`))
	// 顶栏与 Collection 图标。
	iconSearch = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><circle cx="7" cy="7" r="4.2"/><path d="M10.2 10.2L14 14"/></svg>`))
	iconFolder = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"><path d="M2 4.5h4l1.4 1.6H14v6.4H2z"/></svg>`))
	iconSend = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M2 8l12-5-4 12-2.2-5.2L2 8z"/><path d="M7.8 9.8L14 3"/></svg>`))
	iconPlus = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><path d="M8 3v10M3 8h10"/></svg>`))
	iconNew = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"><path d="M4 2h5l3 3v9H4z"/><path d="M9 2v3h3"/></svg>`))
	iconOpen = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"><path d="M2 5V3.5h4l1.3 1.5H14V13H2z"/></svg>`))
	iconSave = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"><path d="M3 2.5h8.5L13.5 5v8.5h-11z"/><path d="M5 2.5v4h6v-4M5 13.5v-4h6v4"/></svg>`))
	iconImport = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"><path d="M8 2v8M5 7l3 3 3-3"/><path d="M3 11.5V14h10v-2.5"/></svg>`))
	// 文件树：通用文件图标（步骤作为文件节点，用类型色着色）。
	iconFile = ui.MustParseSVG([]byte(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"><path d="M4 1.8h5l3 3v9.4H4z"/><path d="M9 1.8v3h3"/></svg>`))
)

// Postman 风格配色：品牌橙与 HTTP 方法的语义色。
var (
	brandOrange  = ui.RGB(255, 108, 55) // Postman 标志橙
	methodColors = map[string]ui.Color{
		"GET": ui.RGB(97, 175, 254), "POST": ui.RGB(73, 204, 144),
		"PUT": ui.RGB(252, 161, 48), "PATCH": ui.RGB(80, 227, 194),
		"DELETE": ui.RGB(249, 62, 62), "HEAD": ui.RGB(144, 18, 254),
	}
	// 步骤类型图标色，让左侧 Collection 树一眼区分组件类型。
	stepColors = map[model.StepType]ui.Color{
		model.StepRequest: ui.RGB(97, 175, 254), model.StepTCP: ui.RGB(34, 197, 94), model.StepLoop: ui.RGB(144, 18, 254),
		model.StepBranch: ui.RGB(252, 161, 48), model.StepSwitch: ui.RGB(80, 227, 194),
		model.StepParallel: ui.RGB(59, 130, 246), model.StepMerge: ui.RGB(59, 130, 246),
		model.StepProcess: ui.RGB(148, 163, 184), model.StepScript: ui.RGB(234, 179, 8),
		model.StepAssert: ui.RGB(73, 204, 144), model.StepExport: ui.RGB(249, 115, 22),
	}
)

// methodColor 返回 HTTP 方法的语义色，未知方法用强调色。
func methodColor(m string) ui.Color {
	if c, ok := methodColors[strings.ToUpper(strings.TrimSpace(m))]; ok {
		return c
	}
	return ui.RGB(148, 163, 184)
}

// iconButton 是通用图标按钮：透明底、hover 高亮，用于顶栏与侧栏。
func iconButton(c *ui.Context, t *ui.Theme, key string, icon *ui.SVG, tip string, size float32, col ui.Color) ui.Element {
	b := ui.ButtonBase(c.Key(key)).Size(size, size).Radius(5).Label(tip).Tooltip(tip).Cursor(ui.CursorPointer)
	b.Children(func() {
		ui.Icon(c, icon).Size(size-8, size-8).TextColor(col)
	})
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	return b
}

// stepToolButton 渲染步骤列表右侧的轻量图标按钮：默认透明无边框，仅 hover 时淡色高亮，
// 比标准小方块按钮更克制；danger 用于删除按钮，图标着危险色。
func stepToolButton(c *ui.Context, t *ui.Theme, key string, icon *ui.SVG, tip string, disabled, danger bool) ui.Element {
	b := ui.ButtonBase(c.Key(key)).Size(22, 22).Radius(4).Label(tip).Tooltip(tip).
		Cursor(ui.CursorPointer).Disabled(disabled)
	col := t.TextMuted
	if danger {
		col = t.Danger
	}
	b.Children(func() {
		ui.Icon(c, icon).Size(13, 13).TextColor(col)
	})
	if b.Hovered() && !disabled {
		b.Background(t.SurfaceHover)
	}
	return b
}

// App 保存界面所有可变状态。view 是纯函数：从状态构建界面。
type App struct {
	win      *mygo.Window
	scenario *model.Scenario
	filePath string

	selectedStep int
	logs         []string
	running      bool
	runOK        bool
	lastResult   *engine.Result
	currentEng   *engine.Engine
	engMu        sync.RWMutex // 保护 currentEng 的跨 goroutine 读写

	// 数据驱动运行
	dataFile      string                 // 数据文件路径（CSV/JSON）
	dataResults   []engine.DatasetResult // 迭代结果
	dataRunning   bool
	showDataPanel bool

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
	adding     bool
	stepSearch string // 左侧文件树步骤搜索过滤
	showSteps  bool   // 文件树"步骤"文件夹节点是否展开

	// 底部控制台视图：0=日志 1=结果表 2=变量
	consoleView int
	// 结果表：选中的数组变量名与表格状态
	resultVar   string
	resultState ui.ListState
	resultCols  []ui.TableColumn
	// 变量预览（P1）：数组/对象变量的可展开状态，按变量名记录。
	varExpanded map[string]bool
	// 全局变量编辑
	showVars    bool
	newVarKey   string
	newVarValue string

	// cURL 导入面板
	showImport bool
	importText string
	importMsg  string

	// 场景模板/新建向导
	showTemplateWizard bool
	templateIndex      int

	// 主工作区标签：0=步骤编排 1=接口调试 2=运行结果（Postman 风格标签式工作区）
	mainTab int

	// 接口调试面板
	showDebug         bool // 兼容旧动作：请求双击时置 true；view 里同步到 mainTab=1
	debugMethod       string
	debugURL          string
	debugHeaders      string // 每行 key: value
	debugQuery        string // 每行 key: value
	debugBody         string
	debugBodyType     string            // raw | form-data | urlencoded | binary | graphql | none
	debugFormData     []model.FormField // 表单字段（key/type/value）
	debugGQLQuery     string
	debugGQLVars      string
	debugAuthType     string
	debugAuthToken    string
	debugUser         string
	debugPass         string
	debugKeyName      string // apikey 参数名
	debugKeyValue     string // apikey 参数值
	debugKeyIn        string // header | query
	debugTokenURL     string // oauth2 token url
	debugClientID     string
	debugClientSecret string
	debugScope        string
	debugGrantType    string // client_credentials | password
	debugTimeout      string
	debugInsecure     bool
	debugView         int // 0=美化响应 1=响应头 2=Cookie 3=cURL
	debugReqTab       int // 0=Params 1=Headers 2=Body 3=Auth（Postman 风格子标签）
	debugResult       *engine.DebugResult
	debugSending      bool
	splitDebug        float32

	// 调试面板文本框高度（可拖拽调整）
	headersHeight float32
	queryHeight   float32
	bodyHeight    float32
	importHeight  float32
	gqlVarsHeight float32

	// 环境变量分层：当前选中的环境名 + 新增环境变量缓冲
	activeEnvBuf string
	newEnvName   string

	// 代码生成 Code Snippet：语言选择 + 生成结果缓存
	snippetLang string
	snippetText string

	// 请求历史：显示开关 + 缓存列表
	showHistory      bool
	historyList      []history.Entry
	historyMsg       string
	preScriptHeight  float32
	postScriptHeight float32
	tcpSendHeight    float32

	// 内容格式化按钮的临时状态提示
	scriptFmtMsg    string
	bodyFmtMsg      string
	gqlFmtMsg       string
	debugBodyFmtMsg string
}

// Run 启动 GUI。
func Run() {
	app := &App{
		scenario:         demoScenario(),
		selectedStep:     -1,
		splitSide:        280,
		splitLog:         220,
		splitDebug:       360,
		debugMethod:      "GET",
		debugAuthType:    "none",
		debugBodyType:    "raw",
		debugKeyIn:       "header",
		debugGrantType:   "client_credentials",
		debugTimeout:     "30",
		headersHeight:    56,
		queryHeight:      56,
		bodyHeight:       72,
		importHeight:     200,
		gqlVarsHeight:    90,
		preScriptHeight:  72,
		postScriptHeight: 72,
		tcpSendHeight:    56,
		snippetLang:      "python",
		showSteps:        true,
		showVars:         true,
		varExpanded:      map[string]bool{},
	}
	app.logs = append(app.logs, "就绪。打开或新建场景，配置步骤后点击【一键运行】。")
	mygo.App.WhenReady(func() {
		app.win = mygo.NewWindow(mygo.WindowOptions{
			Title:   "RequestFlow " + version.Short() + " · request流程自动化",
			Width:   1180,
			Height:  780,
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
	// 兼容旧动作：双击请求步骤会置 showDebug=true，这里同步到调试标签并复位，
	// 让所有入口（工具栏/双击/原 showDebug 调用）统一由 mainTab 驱动主工作区。
	if a.showDebug {
		a.mainTab = 1
		a.showDebug = false
	}
	ui.Column(c).Fill().Children(func() {
		a.toolbar(c, t)
		ui.Split(c, &a.splitSide, func() { a.sidebar(c, t) }, func() {
			ui.Column(c).Fill().Children(func() {
				// Postman 风格：主工作区用标签切换「编排 / 调试 / 运行结果」。
				ui.Row(c).Padding(6, 12, 0, 12).Children(func() {
					ui.Tabs(c, &a.mainTab, "步骤编排", "接口调试", "运行结果")
				})
				ui.Divider(c)
				switch a.mainTab {
				case 1:
					a.debugPanel(c, t)
				case 2:
					a.consoleOnly(c, t)
				default:
					a.editor(c, t)
				}
			})
		}).Grow(1)
		a.statusBar(c, t)
		// 新建场景向导：以 Modal 覆盖窗口，选择模板或空白场景。
		ui.Modal(c, &a.showTemplateWizard, func() {
			a.templateWizard(c, t)
		})
	})
}

// templateWizard 渲染新建场景的模板选择向导。
func (a *App) templateWizard(c *ui.Context, t *ui.Theme) {
	templates := scenarioTemplates()
	if a.templateIndex >= len(templates) {
		a.templateIndex = 0
	}
	ui.Column(c).Width(560).Padding(20).Gap(10).Background(t.Surface).Radius(12).Children(func() {
		ui.Text(c, "新建场景").FontSize(18).Bold()
		ui.Text(c, "选择模板快速开始，之后可自由修改步骤与参数。").FontSize(11).TextColor(t.TextMuted)
		ui.Divider(c)
		ui.Scroll(c).Height(320).Gap(6).Children(func() {
			for i, tpl := range templates {
				i, tpl := i, tpl
				selected := i == a.templateIndex
				row := ui.ButtonBase(c.Key(fmt.Sprintf("tpl-%d", i))).Padding(10).Radius(8).
					AlignItems(ui.Center).Cursor(ui.CursorPointer)
				if selected {
					row.Background(t.Accent.Mix(t.Surface, 0.88))
				} else if row.Hovered() {
					row.Background(t.SurfaceHover)
				}
				row.Children(func() {
					ui.Column(c).Gap(3).Children(func() {
						ui.Text(c, tpl.Name).FontSize(13).Bold().TextColor(func() ui.Color {
							if selected {
								return t.Accent
							}
							return t.Text
						}())
						ui.Text(c, tpl.Desc).FontSize(10).TextColor(t.TextMuted)
					})
				})
				if row.Clicked() {
					a.templateIndex = i
				}
				if row.DoubleClicked() {
					a.applyTemplate(tpl)
				}
			}
		})
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Spacer(c)
			if ui.Button(c, "取消").Clicked() {
				a.showTemplateWizard = false
			}
			if ui.PrimaryButton(c, "使用模板").Clicked() {
				a.applyTemplate(templates[a.templateIndex])
			}
		})
	})
}

// consoleOnly 把运行结果（日志/结果表/变量）作为独立标签页铺满主工作区。
func (a *App) consoleOnly(c *ui.Context, t *ui.Theme) {
	a.console(c, t)
}

// ---------- 顶部工具栏（Postman 风格：品牌 + 场景 + 文件/数据/运行分组 + 溢出菜单） ----------
func (a *App) toolbar(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Gap(8).Padding(10, 14).AlignItems(ui.Center).Children(func() {
		// 品牌标识：橙色圆角块 + 名称。
		ui.Box(c).Width(26).Height(26).Radius(7).Background(brandOrange).Center().Children(func() {
			ui.Text(c, "R").FontSize(14).Bold().TextColor(ui.RGB(255, 255, 255))
		})
		ui.Column(c).Gap(0).Children(func() {
			ui.Text(c, "RequestFlow").FontSize(14).Bold()
			ui.Text(c, "request流程自动化").FontSize(9).TextColor(t.TextMuted)
		})
		ui.Divider(c).Height(24)
		// 当前场景名（工作区概念）。
		ui.Icon(c, iconFolder).Size(14, 14).TextColor(t.TextMuted)
		ui.Text(c, a.scenario.Name).FontSize(12).Bold()
		if a.filePath != "" {
			ui.Badge(c, "已保存").TextColor(t.TextMuted)
		} else {
			ui.Badge(c, "未保存").TextColor(t.TextMuted)
		}
		ui.Spacer(c)

		// ── 文件组：新建 / 打开 / 保存 ──
		ui.Text(c, "文件").FontSize(9).TextColor(t.TextMuted)
		if iconButton(c, t, "new", iconNew, "新建场景", 28, t.Text).Clicked() {
			a.newScenario()
		}
		if iconButton(c, t, "open", iconOpen, "打开场景", 28, t.Text).Clicked() {
			a.openScenario()
		}
		if iconButton(c, t, "save", iconSave, "保存场景", 28, t.Text).Clicked() {
			a.saveScenario()
		}
		ui.Divider(c).Height(20)

		// ── 数据组：导入（统一入口）/ 数据驱动 ──
		ui.Text(c, "数据").FontSize(9).TextColor(t.TextMuted)
		// 导入入口统一到一个菜单，避免 cURL 与 Postman/OpenAPI 分散两处让用户找错。
		ui.PopoverMenuButton(c, "⇩ 导入", func(m *ui.Menu) {
			if m.Item("⇥ 从 cURL 粘贴").Chosen() {
				a.openImport("curl")
			}
			if m.Item("⇩ 导入 Postman / OpenAPI 文件").Chosen() {
				a.openImport("file")
			}
		})
		if ui.Button(c, "⊞ 数据驱动").Tooltip("用 CSV/JSON 数据逐行运行场景").Clicked() {
			a.pickDataFile()
		}
		ui.Divider(c).Height(20)

		// ── 运行组：停止 / 运行 ──
		ui.Text(c, "运行").FontSize(9).TextColor(t.TextMuted)
		if ui.Button(c, "■").Tooltip("停止运行").Disabled(!a.running).Clicked() {
			a.stop()
		}
		if ui.PrimaryButton(c, "▶ 运行").Disabled(a.running).Clicked() {
			a.run()
		}
		ui.Divider(c).Height(20)

		// ── 溢出菜单：次要功能（历史 / 报告 / 导入 API / 关于） ──
		ui.PopoverMenuButton(c, "⋯ 更多", func(m *ui.Menu) {
			if m.Item("🕘 请求历史").Chosen() {
				a.openHistory()
			}
			if m.Item("⤓ 导出测试报告").Disabled(a.lastResult == nil && len(a.dataResults) == 0).Chosen() {
				a.exportReport()
			}
			m.Separator()
			if m.Item("ⓘ 关于 RequestFlow " + version.Short()).Chosen() {
				a.showAbout()
			}
		})
	})
}

// confirmDeleteEnv 删除当前环境前二次确认，避免误删整组环境变量。
func (a *App) confirmDeleteEnv() {
	env := a.scenario.ActiveEnv
	if env == "" {
		return
	}
	go func() {
		res, _ := mygo.Dialog.Message(mygo.MessageOptions{
			Parent:  a.win,
			Type:    mygo.MessageWarning,
			Title:   "删除环境",
			Message: "确定删除环境「" + env + "」及其全部变量吗？此操作不可撤销。",
			Buttons: []string{"取消", "删除"},
		})
		if res.Button == 1 {
			delete(a.scenario.Environments, env)
			a.scenario.ActiveEnv = ""
			a.activeEnvBuf = ""
			a.log("已删除环境 " + env)
		}
	}()
}

// openImport 统一导入入口：curl=打开粘贴面板，file=选择 Postman/OpenAPI 文件。
func (a *App) openImport(mode string) {
	a.showImport = true
	a.showHistory = false
	a.importMsg = ""
	a.mainTab = 0
	if mode == "file" {
		a.importAPIFile()
	}
}

// showAbout 弹出"关于"对话框展示完整版本信息。
// Dialog.Message 会阻塞，因此放到独立 goroutine 中调用。
func (a *App) showAbout() {
	go func() {
		detail := version.Detail()
		if p := logger.Path(); p != "" {
			detail += "\n\n日志文件:\n" + p
		}
		if err := logger.Err(); err != nil {
			detail += "\n\n⚠ 日志未启用:\n" + err.Error()
		}
		_, _ = mygo.Dialog.Message(mygo.MessageOptions{
			Parent:  a.win,
			Type:    mygo.MessageInfo,
			Title:   "关于 RequestFlow",
			Message: version.String(),
			Detail:  detail,
			Buttons: []string{"确定"},
		})
	}()
}

// ---------- 左侧：文件树风格场景导航 ----------
func (a *App) sidebar(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Children(func() {
		// 面板标题栏。
		ui.Row(c).Padding(12, 12, 8, 12).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconFolder).Size(16, 16).TextColor(brandOrange)
			ui.Text(c, "资源管理器").FontSize(13).Bold()
			ui.Spacer(c)
			ui.Text(c, fmt.Sprintf("%d 项", len(a.scenario.Steps))).FontSize(10).TextColor(t.TextMuted)
		})
		// 搜索框：过滤步骤。
		ui.Row(c).Padding(0, 12, 10, 12).Children(func() {
			ui.SearchField(c, &a.stepSearch).Placeholder("搜索步骤…")
		})
		// 文件树根节点区。
		ui.Scroll(c).Grow(1).Padding(2, 8, 10, 8).Children(func() {
			a.fileTree(c, t)
		})
	})
}

// fileTree 文件树：场景（步骤）与环境变量两棵可折叠文件夹节点。
func (a *App) fileTree(c *ui.Context, t *ui.Theme) {
	// 根文件夹：场景 —— 子节点是各个步骤。
	a.treeFolderRow(c, t, "folder-scene", &a.showSteps, iconFolder, brandOrange, func() {
		ui.TextInput(c, &a.scenario.Name).FontSize(13).Bold().Grow(1)
	}, nil)
	if a.showSteps {
		treeGuide(c, t, func() { a.stepList(c, t) })
	}

	ui.Box(c).Height(8)

	// 根文件夹：环境变量 —— 子节点是环境选择器 + 全局变量 + 当前环境变量。
	varCount := len(a.scenario.Variables)
	if env := a.scenario.EnvVars(a.scenario.ActiveEnv); env != nil {
		varCount += len(env)
	}
	a.treeFolderRow(c, t, "folder-vars", &a.showVars, iconFolder, t.TextMuted, func() {
		ui.Text(c, "变量").FontSize(12).Bold()
		ui.Text(c, fmt.Sprintf("(%d)", varCount)).FontSize(10).TextColor(t.TextMuted)
	}, nil)
	if a.showVars {
		treeGuide(c, t, func() { a.environmentEditor(c, t) })
	}
}

// environmentEditor 环境变量分层编辑：环境选择器 + 全局变量 + 当前环境变量。
// 运行时"当前环境变量"覆盖同名"全局变量"，可在多套环境间快速切换。
func (a *App) environmentEditor(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Gap(6).Children(func() {
		// 环境选择器：无 / 各环境名。切换时同步到场景与本地缓冲。
		envs := append([]string{"(无)"}, a.scenario.EnvNames()...)
		cur := "(无)"
		if a.scenario.ActiveEnv != "" {
			cur = a.scenario.ActiveEnv
		}
		sel := cur
		if ui.Select(c, &sel, envs).Label("当前环境").Changed() {
			if sel == "(无)" {
				a.scenario.ActiveEnv = ""
			} else {
				a.scenario.ActiveEnv = sel
			}
			a.activeEnvBuf = a.scenario.ActiveEnv
		}
		fieldHint(c, t, "当前环境变量覆盖同名全局变量；运行时优先级：环境变量 > 全局变量。")

		// 新建环境
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			ui.TextInput(c, &a.newEnvName).Placeholder("新环境名").Width(90)
			if ui.Button(c, "+").Width(26).Tooltip("新建环境").Clicked() {
				n := strings.TrimSpace(a.newEnvName)
				if n != "" {
					if a.scenario.Environments == nil {
						a.scenario.Environments = map[string]map[string]string{}
					}
					if _, exists := a.scenario.Environments[n]; !exists {
						a.scenario.Environments[n] = map[string]string{}
						a.scenario.ActiveEnv = n
						a.activeEnvBuf = n
					}
					a.newEnvName = ""
				}
			}
			if a.scenario.ActiveEnv != "" {
				if ui.Button(c, "✕").Width(26).Tooltip("删除当前环境").Clicked() {
					a.confirmDeleteEnv()
				}
			}
		})

		// 全局变量区
		ui.Text(c, "全局变量").FontSize(11).Bold().TextColor(t.TextMuted)
		a.variablesEditor(c, t)

		// 当前环境变量区
		if a.scenario.ActiveEnv != "" {
			ui.Divider(c)
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "环境 ["+a.scenario.ActiveEnv+"] 变量").FontSize(11).Bold().TextColor(t.Accent)
			})
			envVars := a.scenario.Environments[a.scenario.ActiveEnv]
			a.envVariablesEditor(c, t, envVars)
		}
	})
}

// envVariablesEditor 编辑指定环境的变量集（key=value），结构同全局变量编辑器。
func (a *App) envVariablesEditor(c *ui.Context, t *ui.Theme, vars map[string]string) {
	if vars == nil {
		return
	}
	keyValueRows(c, t, vars, &a.newVarKey, &a.newVarValue,
		"如 baseUrl: https://api.prod.com；切换环境即整组生效", 78)
}

// treeFolderRow 文件夹节点行：展开箭头 + 文件夹图标 + 标题内容 + 尾部操作。
func (a *App) treeFolderRow(c *ui.Context, t *ui.Theme, key string, open *bool, icon *ui.SVG, iconColor ui.Color, label func(), tail func()) {
	ui.Row(c.Key(key)).Gap(4).Padding(5, 4).AlignItems(ui.Center).Children(func() {
		arrow := map[bool]string{true: "▾", false: "▸"}[*open]
		if ui.Button(c.Key("tog"), arrow).Width(16).FontSize(10).Clicked() {
			*open = !*open
		}
		ui.Icon(c, icon).Size(14, 14).TextColor(iconColor)
		label()
		ui.Spacer(c)
		if tail != nil {
			tail()
		}
	})
}

// treeGuide 在子节点左侧画一条贯穿高度的竖线，形成文件树的层级引导线。
func treeGuide(c *ui.Context, t *ui.Theme, fn func()) {
	color := t.Border.Mix(t.Surface, 0.2)
	ui.Row(c).AlignItems(ui.Stretch).Children(func() {
		ui.Box(c).Width(14)                  // 缩进占位（透明）
		ui.Box(c).Width(1).Background(color) // 竖线：被 Stretch 拉满整棵子树高度
		ui.Column(c).Grow(1).Children(fn)
	})
}

// treeBranch 子节点行首的横折线（与 guide 竖线相接，形成 ├─ 效果）。
func treeBranch(c *ui.Context, t *ui.Theme) {
	ui.Box(c).Width(10).Height(1).Background(t.Border.Mix(t.Surface, 0.2))
}

// keyValueRows 渲染一个通用的 key=value 映射编辑器：逐行展示已有键值对（可编辑、删除），
// 底部提供新增行。newKey/newValue 为「新行」输入缓冲，由调用方持有以跨帧保留。
// 供全局变量、环境变量等复用，消除重复的增删改样板。
func keyValueRows(c *ui.Context, t *ui.Theme, vars map[string]string,
	newKey, newValue *string, topHint string, keyWidth float32) {
	if keyWidth <= 0 {
		keyWidth = 110
	}
	ui.Column(c).Gap(4).Children(func() {
		if topHint != "" {
			fieldHint(c, t, topHint)
		}
		keys := make([]string, 0, len(vars))
		for k := range vars {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			key := k
			val := vars[key]
			ui.Row(c.Key("kv-" + key)).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Text(c, key).FontSize(11).Width(keyWidth).SingleLine()
				input := val
				if ui.TextInput(c, &input).Grow(1).Changed() {
					vars[key] = input
				}
				if ui.Button(c.Key("del"), "×").Width(26).Tooltip("删除此变量").Clicked() {
					delete(vars, key)
				}
			})
		}
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			ui.TextInput(c, newKey).Placeholder("变量名").Width(keyWidth - 26)
			ui.TextInput(c, newValue).Placeholder("值").Grow(1)
			if ui.Button(c, "+").Width(26).Tooltip("添加变量").Clicked() {
				k := strings.TrimSpace(*newKey)
				if k != "" {
					vars[k] = *newValue
					*newKey, *newValue = "", ""
				}
			}
		})
	})
}

// variablesEditor 可视化编辑全局变量（key=value），直接读写 scenario.Variables。
func (a *App) variablesEditor(c *ui.Context, t *ui.Theme) {
	if a.scenario.Variables == nil {
		a.scenario.Variables = map[string]string{}
	}
	keyValueRows(c, t, a.scenario.Variables, &a.newVarKey, &a.newVarValue,
		"用 {{key}} 在步骤中引用", 78)
}

func (a *App) addMenu(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Padding(8, 12).Gap(4).Background(t.Surface).Radius(8).Children(func() {
		ui.Text(c, "选择步骤类型").FontSize(10).TextColor(t.TextMuted)
		types := []struct {
			label string
			typ   model.StepType
		}{
			{"HTTP 请求", model.StepRequest},
			{"TCP Socket", model.StepTCP},
			{"条件分支", model.StepBranch},
			{"多分支 switch", model.StepSwitch},
			{"循环 / 翻页", model.StepLoop},
			{"并行扇出", model.StepParallel},
			{"扇入合并(门控)", model.StepMerge},
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
	q := strings.ToLower(strings.TrimSpace(a.stepSearch))
	// 搜索过滤：匹配步骤名或类型名，保持原始索引用于选择/移动。
	visible := make([]int, 0, len(steps))
	for i := range steps {
		if q == "" || strings.Contains(strings.ToLower(steps[i].Name), q) ||
			strings.Contains(strings.ToLower(string(steps[i].Type)), q) {
			visible = append(visible, i)
		}
	}
	if len(visible) == 0 {
		if q != "" {
			ui.Text(c, "没有匹配的步骤").FontSize(11).TextColor(t.TextMuted).Padding(8)
		} else {
			ui.Text(c, "暂无步骤，点击下方 + 新建。").FontSize(11).TextColor(t.TextMuted).Padding(8)
		}
	} else {
		for _, idx := range visible {
			step := &steps[idx]
			selected := idx == a.selectedStep
			sc := stepColors[step.Type]
			ui.Row(c.Key(fmt.Sprintf("step-%d", idx))).Gap(4).Padding(2, 6).Radius(6).
				AlignItems(ui.Center).Children(func() {
				// 文件树横折线 ├─，与上层 guide 竖线相接。
				treeBranch(c, t)
				// 类型色文件图标本身兼作启停开关：彩色=启用、淡化=禁用，点击切换。
				iconColor := sc
				enableTip := "点击禁用此步骤"
				if !step.Enabled {
					iconColor = t.TextMuted
					enableTip = "点击启用此步骤"
				}
				fileIconBtn := ui.ButtonBase(c.Key("en")).Size(16, 16).Radius(4).
					Tooltip(enableTip).Cursor(ui.CursorPointer)
				fileIconBtn.Children(func() {
					ui.Icon(c, iconFile).Size(13, 13).TextColor(iconColor)
				})
				if fileIconBtn.Hovered() {
					fileIconBtn.Background(t.SurfaceHover)
				}
				if fileIconBtn.Clicked() {
					step.Enabled = !step.Enabled
				}
				name := step.Name
				if name == "" {
					name = string(step.Type)
				}
				b := ui.Button(c, name).Grow(1).
					Tooltip("点击选中并配置；双击 HTTP 请求进入调试")
				if selected {
					b = b.Background(t.Accent.Mix(t.Surface, 0.86)).TextColor(t.Accent).Bold()
				}
				if !step.Enabled {
					b = b.TextColor(t.TextMuted)
				}
				if b.Clicked() {
					a.selectStep(idx)
				}
				if b.DoubleClicked() {
					a.selectStep(idx)
					a.openStepDebug(idx)
				}
				if stepToolButton(c, t, "up", iconMoveUp, "上移步骤", idx == 0, false).Clicked() {
					a.moveStep(idx, -1)
				}
				if stepToolButton(c, t, "dn", iconMoveDown, "下移步骤", idx == len(steps)-1, false).Clicked() {
					a.moveStep(idx, 1)
				}
				if stepToolButton(c, t, "del", iconDelete, "删除步骤", false, true).Clicked() {
					a.deleteStep(idx)
				}
			})
		}
	}
	// 文件树底部：新建步骤入口（展开类型菜单），同样带横折线。
	ui.Row(c).Gap(4).Padding(2, 6).AlignItems(ui.Center).Children(func() {
		treeBranch(c, t)
		label := "＋ 新建步骤"
		if a.adding {
			label = "− 收起"
		}
		if ui.Button(c, label).Grow(1).TextColor(t.TextMuted).Clicked() {
			a.adding = !a.adding
		}
	})
	if a.adding {
		a.addMenu(c, t)
	}
}

func (a *App) stepLabel(s model.Step) string {
	icon := map[model.StepType]string{
		model.StepRequest: "⇄", model.StepTCP: "TCP", model.StepLoop: "↻", model.StepBranch: "分", model.StepSwitch: "选",
		model.StepParallel: "扇出", model.StepMerge: "扇入", model.StepProcess: "⚙",
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
	// cURL / API 定义导入模式：覆盖编辑器区域
	if a.showImport {
		a.importPanel(c, t)
		return
	}
	// 请求历史模式
	if a.showHistory {
		a.historyPanel(c, t)
		return
	}
	ui.Column(c).Fill().Children(func() {
		if a.selectedStep < 0 || a.selectedStep >= len(a.scenario.Steps) {
			emptyState(c, t, "选择一个步骤进行配置",
				"单击左侧步骤查看配置，双击请求步骤进入调试；内置登录提取、HTTP、循环翻页、数据处理、脚本、断言、导出")
			return
		}
		step := &a.scenario.Steps[a.selectedStep]
		ui.Scroll(c).Grow(1).Padding(14).Gap(10).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "步骤配置").FontSize(14).Bold()
				ui.Text(c, string(step.Type)).FontSize(11).TextColor(t.Accent)
				ui.Spacer(c)
				ui.Checkbox(c, &step.Enabled, "启用")
				ui.Checkbox(c, &step.ContinueOnFail, "失败后继续")
			})
			ui.TextInput(c, &step.Name).Placeholder("给步骤起个名字，便于识别").Label("步骤名称")
			ui.Divider(c)
			switch step.Type {
			case model.StepRequest:
				a.editorRequest(c, t, step)
			case model.StepTCP:
				a.editorTCP(c, t, step)
			case model.StepLoop:
				a.editorLoop(c, t, step)
			case model.StepBranch:
				a.editorBranch(c, t, step)
			case model.StepSwitch:
				a.editorSwitch(c, t, step)
			case model.StepParallel:
				a.editorParallel(c, t, step)
			case model.StepMerge:
				a.editorMerge(c, t, step)
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

// debugPanel 接口调试面板：上半请求配置，下半响应展示。
func (a *App) debugPanel(c *ui.Context, t *ui.Theme) {
	ui.SplitVertical(c, &a.splitDebug, func() {
		a.debugRequestArea(c, t)
	}, func() {
		a.debugResponseArea(c, t)
	}).Fill()
}

// debugRequestArea Postman 风格请求构建区：请求行(方法徽章+URL+Send) + 配置子标签。
func (a *App) debugRequestArea(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Children(func() {
		ui.Column(c).Gap(8).Padding(10, 12, 6, 12).Children(func() {
			// 标题行。
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "接口调试").FontSize(13).Bold()
				ui.Text(c, "单独发送请求并查看响应").FontSize(10).TextColor(t.TextMuted)
				ui.Spacer(c)
				if a.debugResult != nil && a.debugResult.Curl != "" {
					if ui.Button(c, "⇥ 导入此 cURL").Clicked() {
						a.importText = a.debugResult.Curl
						a.showImport = true
						a.mainTab = 0
					}
				}
			})
			// 请求行：方法色标 + 方法选择 + URL + 醒目 Send（Postman 标志性布局）。
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				method := a.debugMethod
				if method == "" {
					method = "GET"
				}
				ui.Box(c).Width(6).Height(30).Radius(3).Background(methodColor(method))
				ui.Select(c, &a.debugMethod, []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"}).Width(96)
				ui.TextInput(c, &a.debugURL).Placeholder("{{baseUrl}}/path 或完整 URL").Grow(1).Height(30)
				if ui.PrimaryButton(c, "▶ 发送").Disabled(a.debugSending).Clicked() {
					a.doDebugSend()
				}
			})
			// 超时与证书：收起为一行小字，默认不占视觉重心。
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "超时(秒)").FontSize(10).TextColor(t.TextMuted)
				ui.TextInput(c, &a.debugTimeout).Width(60)
				ui.Checkbox(c, &a.debugInsecure, "跳过 TLS 校验")
			})
		})
		// 配置子标签（Postman：Params / Headers / Body / Authorization）。
		ui.Row(c).Padding(0, 12).Children(func() {
			ui.Tabs(c, &a.debugReqTab, "Params", "Headers", "Body", "Authorization")
			ui.Spacer(c)
			ui.Text(c, "支持 {{变量}}").FontSize(10).TextColor(t.TextMuted)
		})
		ui.Divider(c)
		ui.Scroll(c).Grow(1).Padding(10, 12, 8, 12).Children(func() {
			switch a.debugReqTab {
			case 0: // Params
				fieldHint(c, t, "查询参数，每行 key: value；发送时拼接到 URL。")
				resizableTextArea(c, t, &a.debugQuery, &a.queryHeight, "page: 1", 40, 600)
			case 1: // Headers
				fieldHint(c, t, "请求头，每行 key: value；可直接粘贴 Chrome 复制的请求头。")
				resizableTextArea(c, t, &a.debugHeaders, &a.headersHeight, "Authorization: Bearer {{token}}", 40, 600)
			case 2: // Body
				bodyEditor(c, t, &a.debugBodyType, &a.debugBody, &a.debugFormData,
					&a.debugGQLQuery, &a.debugGQLVars, &a.bodyHeight, `{"key": "{{value}}"}`, &a.debugBodyFmtMsg)
			case 3: // Authorization
				authEditor(c, t, &a.debugAuthType, &a.debugAuthToken, &a.debugUser, &a.debugPass,
					&a.debugKeyName, &a.debugKeyValue, &a.debugKeyIn,
					&a.debugTokenURL, &a.debugClientID, &a.debugClientSecret, &a.debugScope, &a.debugGrantType)
			}
		})
	})
}

// debugResponseArea Postman 风格响应区：状态/耗时/大小概览 + 分段标签。
func (a *App) debugResponseArea(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Padding(8, 12).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Segmented(c, &a.debugView, "Body", "Headers", "Cookies", "cURL", "代码")
			ui.Spacer(c)
			if a.debugSending {
				ui.Text(c, "发送中…").FontSize(11).TextColor(t.Accent)
			} else if a.debugResult != nil {
				res := a.debugResult
				if res.OK {
					statusColor := t.Success
					if res.StatusCode >= 400 {
						statusColor = t.Danger
					}
					ui.Badge(c, fmt.Sprintf("%d %s", res.StatusCode, res.StatusText)).TextColor(statusColor)
					ui.Text(c, fmt.Sprintf("%d ms", res.DurationMS)).FontSize(11).TextColor(t.TextMuted)
					ui.Text(c, fmt.Sprintf("%d B", res.Size)).FontSize(11).TextColor(t.TextMuted)
				} else {
					ui.Text(c, "✗ 请求失败").FontSize(11).Bold().TextColor(t.Danger)
				}
			}
		})
		ui.Divider(c)
		ui.Scroll(c).Grow(1).Padding(10, 12, 10, 12).Children(func() {
			// 代码生成标签：基于最近一次调试请求生成，无需请求成功即可查看。
			if a.debugView == 4 {
				a.codeSnippetView(c, t)
				return
			}
			if a.debugSending || a.debugResult == nil {
				ui.Column(c).Fill().Center().Gap(6).Children(func() {
					ui.Icon(c, iconSend).Size(24, 24).TextColor(t.TextMuted)
					ui.Text(c, "配置好请求后点击【发送】，响应将显示在这里。").FontSize(11).TextColor(t.TextMuted)
				})
				return
			}
			res := a.debugResult
			if !res.OK {
				ui.Text(c, "错误: "+res.Err).FontSize(11).TextColor(t.Danger)
				return
			}
			switch a.debugView {
			case 1: // 响应头
				if len(res.Headers) == 0 {
					ui.Text(c, "（无响应头）").FontSize(11).TextColor(t.TextMuted)
				}
				for _, kv := range res.Headers {
					ui.Row(c).Gap(8).Children(func() {
						ui.Text(c, kv.Key+":").FontSize(11).Bold().Width(180)
						ui.Text(c, kv.Value).FontSize(11)
					})
				}
			case 2: // Cookie
				if len(res.Cookies) == 0 {
					ui.Text(c, "（无 Cookie）").FontSize(11).TextColor(t.TextMuted)
				}
				for _, kv := range res.Cookies {
					ui.Text(c, kv.Key+" = "+kv.Value).FontSize(11)
				}
			case 3: // cURL
				ui.Text(c, res.Curl).FontSize(11)
			default: // 响应体
				ui.Text(c, res.Formatted).FontSize(11)
			}
		})
	})
}

// codeSnippetView 代码生成面板：把当前请求转成 Python/Go/JS/cURL 代码片段。
func (a *App) codeSnippetView(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Gap(8).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "代码生成").FontSize(12).Bold()
			langs := []string{"python", "go", "javascript", "curl"}
			lang := a.snippetLang
			if ui.Select(c, &lang, langs).Label("语言").Width(150).Changed() {
				a.snippetLang = lang
			}
			ui.Spacer(c)
			if ui.Button(c, "⧉ 复制代码").Clicked() {
				if a.snippetText != "" {
					_ = mygo.Clipboard.Write(transfer.New(transfer.NewItem(
						transfer.Bytes(transfer.Text, []byte(a.snippetText)),
					)))
					a.log("代码片段已复制到剪贴板")
				}
			}
		})
		fieldHint(c, t, "基于最近一次调试的请求（已解析 {{变量}}）生成；先发送一次即可得到对应代码。")
		if a.debugResult == nil || len(a.debugResult.Snippets) == 0 {
			ui.Column(c).Fill().Center().Gap(6).Children(func() {
				ui.Text(c, "先点击【发送】，再切换到此标签查看生成的代码。").FontSize(11).TextColor(t.TextMuted)
			})
			return
		}
		code := a.debugResult.Snippets[a.snippetLang]
		a.snippetText = code
		resizableTextArea(c, t, &a.snippetText, &a.importHeight, "", 120, 800)
	})
}

// doDebugSend 异步发送调试请求，避免阻塞 UI 线程。
func (a *App) doDebugSend() {
	if strings.TrimSpace(a.debugURL) == "" {
		a.debugResult = &engine.DebugResult{OK: false, Err: "URL 不能为空"}
		return
	}
	timeout, _ := strconv.Atoi(strings.TrimSpace(a.debugTimeout))
	vars := map[string]string{"baseUrl": a.scenario.BaseURL}
	for k, v := range a.scenario.Variables {
		vars[k] = v
	}
	req := engine.DebugRequest{
		Method:           a.debugMethod,
		URL:              a.debugURL,
		Headers:          stringToMap(a.debugHeaders, ":"),
		Query:            stringToMap(a.debugQuery, ":"),
		Body:             a.debugBody,
		BodyType:         a.debugBodyType,
		FormData:         a.debugFormData,
		GraphQLQuery:     a.debugGQLQuery,
		GraphQLVariables: a.debugGQLVars,
		AuthType:         a.debugAuthType,
		AuthToken:        a.debugAuthToken,
		AuthUser:         a.debugUser,
		AuthPass:         a.debugPass,
		AuthKeyName:      a.debugKeyName,
		AuthKeyValue:     a.debugKeyValue,
		AuthKeyIn:        a.debugKeyIn,
		AuthTokenURL:     a.debugTokenURL,
		AuthClientID:     a.debugClientID,
		AuthClientSecret: a.debugClientSecret,
		AuthScope:        a.debugScope,
		AuthGrantType:    a.debugGrantType,
		TimeoutSec:       timeout,
		InsecureTLS:      a.debugInsecure,
	}
	a.debugSending = true
	a.debugResult = nil
	go func() {
		res := engine.Debug(req, vars)
		// 记录历史（含请求快照），写文件失败仅在日志提示，不影响调试结果。
		histEntry := history.Entry{
			Method: res.Method, URL: res.URL, Status: res.StatusCode,
			DurationMS: res.DurationMS, Error: res.Err, Source: "debug",
			Headers: req.Headers, BodyType: req.BodyType, Body: req.Body,
		}
		if err := history.Append(histEntry); err != nil {
			logger.Printf("记录请求历史失败: %v", err)
		}
		mygo.RunOnMain(func() {
			a.debugSending = false
			a.debugResult = res
			if res.OK {
				a.log(fmt.Sprintf("[调试] %s %s → %d (%d ms)", res.Method, previewURL(res.URL), res.StatusCode, res.DurationMS))
			} else {
				a.log("[调试] 失败: " + res.Err)
			}
		})
	}()
}

// loadStepToDebug 把某个 request 步骤载入调试面板（仅填充，不发送），供双击步骤时查看与编辑。
func (a *App) loadStepToDebug(step *model.Step) {
	a.debugMethod = step.Method
	if a.debugMethod == "" {
		a.debugMethod = "GET"
	}
	a.debugURL = step.URL
	a.debugHeaders = mapToString(step.Headers)
	a.debugQuery = mapToString(step.Query)
	a.debugBody = step.Body
	a.debugBodyType = step.BodyType
	if a.debugBodyType == "" {
		a.debugBodyType = "raw"
	}
	a.debugFormData = append([]model.FormField(nil), step.FormData...)
	a.debugGQLQuery = step.GraphQLQuery
	a.debugGQLVars = step.GraphQLVariables
	a.debugAuthType = step.AuthType
	if a.debugAuthType == "" {
		a.debugAuthType = "none"
	}
	a.debugAuthToken = step.AuthToken
	a.debugUser = step.AuthUser
	a.debugPass = step.AuthPass
	a.debugKeyName = step.AuthKeyName
	a.debugKeyValue = step.AuthKeyValue
	a.debugKeyIn = step.AuthKeyIn
	if a.debugKeyIn == "" {
		a.debugKeyIn = "header"
	}
	a.debugTokenURL = step.AuthTokenURL
	a.debugClientID = step.AuthClientID
	a.debugClientSecret = step.AuthClientSecret
	a.debugScope = step.AuthScope
	a.debugGrantType = step.AuthGrantType
	if a.debugGrantType == "" {
		a.debugGrantType = "client_credentials"
	}
	a.debugTimeout = "30"
	a.showDebug = true
	a.debugResult = nil
	a.debugView = 0
}

// debugStep 把某个 request 步骤载入调试面板并立即发送。
func (a *App) debugStep(step *model.Step) {
	a.loadStepToDebug(step)
	a.doDebugSend()
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
			ui.Divider(c)
			ui.Text(c, "或导入整个 API 定义").FontSize(12).Bold()
			fieldHint(c, t, "支持 Postman Collection v2.1 / OpenAPI 3.x / Swagger 2.0；自动转成多步骤场景（文件夹/路径展开为步骤，变量/认证/Body 保留）。")
			if ui.Button(c, "📂 导入 Postman / OpenAPI 文件").Clicked() {
				a.importAPIFile()
			}
			ui.Divider(c)
			resizableTextArea(c, t, &a.importText, &a.importHeight,
				"curl 'https://api.example.com/orders?page=1' \\\n  -H 'authorization: Bearer xxx' \\\n  -H 'cookie: session=abc'", 80, 900)
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

// openHistory 加载历史并切换到历史面板。
func (a *App) openHistory() {
	entries, err := history.Load()
	if err != nil {
		a.historyMsg = "✗ 读取历史失败: " + err.Error()
	} else {
		a.historyMsg = ""
	}
	a.historyList = entries
	a.showHistory = true
	a.showImport = false
	a.mainTab = 0
}

// historyPanel 请求历史面板：列表回看、重新载入调试、清空。
func (a *App) historyPanel(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Gap(10).Padding(14, 16).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "请求历史").FontSize(15).Bold()
			ui.Text(c, fmt.Sprintf("共 %d 条（exe 同目录 requestflow-history.jsonl）", len(a.historyList))).FontSize(10).TextColor(t.TextMuted)
			ui.Spacer(c)
			if ui.Button(c, "↻ 刷新").Clicked() {
				a.openHistory()
			}
			if ui.Button(c, "🗑 清空").Clicked() {
				if err := history.Clear(); err != nil {
					a.historyMsg = "✗ 清空失败: " + err.Error()
				} else {
					a.historyList = []history.Entry{}
					a.historyMsg = "已清空历史"
				}
			}
			if ui.Button(c, "✕ 关闭").Clicked() {
				a.showHistory = false
				a.historyMsg = ""
			}
		})
		if a.historyMsg != "" {
			ui.Row(c).Padding(0, 16, 6, 16).Children(func() {
				ui.Text(c, a.historyMsg).FontSize(11).TextColor(t.Accent)
			})
		}
		ui.Scroll(c).Grow(1).Padding(6, 16, 16, 16).Gap(4).Children(func() {
			if len(a.historyList) == 0 {
				ui.Text(c, "暂无历史。发送一次调试请求后会自动记录。").FontSize(11).TextColor(t.TextMuted)
				return
			}
			for _, e := range a.historyList {
				ent := e
				ui.Row(c).Gap(8).Padding(8, 10).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Width(5).Height(24).Radius(2).Background(methodColor(ent.Method))
					ui.Column(c).Grow(1).Gap(2).Children(func() {
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Text(c, ent.Method).FontSize(11).Bold().Width(52)
							ui.Text(c, previewURL(ent.URL)).FontSize(11)
						})
						status := ""
						if ent.Error != "" {
							status = "失败: " + ent.Error
						} else {
							status = fmt.Sprintf("%d · %d ms · %s", ent.Status, ent.DurationMS, ent.Source)
						}
						ui.Text(c, ent.Time+"  "+status).FontSize(10).TextColor(t.TextMuted)
					})
					if ui.Button(c, "载入调试").Clicked() {
						a.loadHistoryToDebug(ent)
					}
				})
			}
		})
	})
}

// loadHistoryToDebug 把一条历史记录的请求快照载回调试面板。
func (a *App) loadHistoryToDebug(e history.Entry) {
	a.debugMethod = e.Method
	a.debugURL = e.URL
	a.debugHeaders = mapToString(e.Headers)
	a.debugBodyType = e.BodyType
	if e.BodyType == "" {
		a.debugBodyType = "raw"
	}
	a.debugBody = e.Body
	a.showHistory = false
	a.mainTab = 1
	a.debugResult = nil
	a.log("已载入历史请求，可编辑后重新发送")
}

// importAPIFile 选择 Postman Collection / OpenAPI / Swagger 文件并导入为场景。
func (a *App) importAPIFile() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Title: "选择 API 定义文件（Postman / OpenAPI / Swagger）",
			Filters: []mygo.FileFilter{
				{Name: "API 定义", Extensions: []string{"json"}},
				{Name: "所有文件", Extensions: []string{"*"}},
			},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		data, err := os.ReadFile(paths[0])
		if err != nil {
			mygo.RunOnMain(func() { a.importMsg = "✗ 读取文件失败: " + err.Error() })
			return
		}
		sc, impErr := importer.Import(data)
		if impErr != nil {
			mygo.RunOnMain(func() { a.importMsg = "✗ 导入失败: " + impErr.Error() })
			return
		}
		base := filepath.Base(paths[0])
		mygo.RunOnMain(func() {
			a.applyLoadedScenario(sc, "")
			a.showImport = false
			a.importMsg = ""
			a.log(fmt.Sprintf("已导入 %s：%d 个步骤，%d 个全局变量", base, len(sc.Steps), len(sc.Variables)))
		})
	}()
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

// clampHeight 把拖拽后的高度限制在 [min, max] 内。
// emptyState 渲染居中的空状态占位：主提示 + 可选副提示，统一各面板无数据时的视觉。
func emptyState(c *ui.Context, t *ui.Theme, title, subtitle string) {
	ui.Column(c).Fill().Center().Gap(6).Children(func() {
		ui.Text(c, title).FontSize(13).Bold().TextColor(t.TextMuted)
		if subtitle != "" {
			ui.Text(c, subtitle).FontSize(11).TextColor(t.TextMuted)
		}
	})
}

// intInput 渲染一个整数输入框，把字符串编辑值同步回 *int；解析失败时保留原值。
// 统一处理 fmt.Sprint/parseInt 样板，供循环/翻页等数字字段复用。
func intInput(c *ui.Context, target *int, label, placeholder string, width float32) {
	s := fmt.Sprint(*target)
	if placeholder != "" && *target == 0 {
		s = ""
	}
	input := ui.TextInput(c, &s).Label(label).Placeholder(placeholder)
	if width > 0 {
		input.Width(width)
	} else {
		input.Grow(1)
	}
	if s != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			*target = n
		}
	}
}

func clampHeight(v, min, max float32) float32 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// fieldHint 在字段下方渲染一行浅灰色注释，说明该字段的用途或格式。
// 组件配置统一用它给出填写指引，降低界面配置的学习成本。
func fieldHint(c *ui.Context, t *ui.Theme, text string) {
	// 提示文字略提亮，避免 10px 灰字在浅色主题下对比度不足难以阅读。
	ui.Text(c, "  ⓘ "+text).FontSize(10).TextColor(t.TextMuted.Mix(t.Text, 0.3))
}

// resizableTextArea 渲染一个可拖拽调整高度的多行文本框。
// 手柄是文本框右下角的标准斜线小三角（同浏览器 textarea 的 resize handle），
// 鼠标悬停显示上下调整光标，上下拖动即改变高度。
// height 指向 App 中保存该文本框高度的变量；min/max 限制避免缩到不可用或撑满屏幕。
func resizableTextArea(c *ui.Context, t *ui.Theme, value *string, height *float32, placeholder string, min, max float32) {
	*height = clampHeight(*height, min, max)
	h := *height
	// 外层 Box 作为定位参照，手柄绝对定位到右下角、悬浮在文本框之上。
	ui.Box(c).Children(func() {
		ui.TextArea(c, value).Placeholder(placeholder).Height(*height)
		// 手柄用 currentColor 跟随文字色；命中区稍大于图标本身，便于拖动。
		grip := ui.Box(c).Absolute().Right(1).Bottom(1).Width(20).Height(20).
			Cursor(ui.CursorResizeNS).Center().Tooltip("拖动右下角调整高度")
		if _, dy, held := grip.Dragged(); held && dy != 0 {
			*height = clampHeight(h+dy, min, max)
		}
		grip.Children(func() {
			ui.Icon(c, resizeGripIcon).Size(16, 16).TextColor(t.Text.Mix(t.TextMuted, 0.35))
		})
	})
}

// formattedTextArea 在可拖拽文本框上方加一个「格式化」按钮，点击后自动美化内容
// （JSON/XML/HTML/键值对），并在按钮旁短暂显示识别到的类型。
// status 是每次渲染重置的临时提示缓冲，由 App 各编辑区持有。
func formattedTextArea(c *ui.Context, t *ui.Theme, value *string, height *float32,
	placeholder string, min, max float32, status *string) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if ui.Button(c, "✨ 格式化").Tooltip("自动美化内容：JSON / XML / HTML / 键值对").Clicked() {
			out, kind, changed := FormatContent(*value)
			if changed {
				*value = out
				*status = "已按 " + kind + " 格式化"
			} else if kind != "" {
				*status = kind + " 已是规范格式"
			} else {
				*status = "未识别可格式化的内容"
			}
		}
		if *status != "" {
			ui.Text(c, *status).FontSize(10).TextColor(t.TextMuted)
		}
	})
	resizableTextArea(c, t, value, height, placeholder, min, max)
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
	fieldHint(c, t, "发送一个 HTTP 请求；响应可提取为变量、供后续步骤使用。URL 与字段均支持 {{变量名}}。")
	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE"}
	method := step.Method
	if method == "" {
		method = "GET"
	}
	// 方法与 URL 同一行：方法短，不必各占一列。
	ui.Row(c).Gap(8).Children(func() {
		if ui.Select(c, &method, methods).Label("方法").Width(110).Changed() {
			step.Method = method
		}
		ui.TextInput(c, &step.URL).Placeholder("{{baseUrl}}/path 或完整 URL").Label("URL").Grow(1)
	})
	fieldHint(c, t, "相对路径自动拼接场景 Base URL；写完整 http(s):// 则忽略 Base URL。")

	// Headers 与 Query 并排两列：都是短小的 key=value 列表。
	ui.Row(c).Gap(10).Children(func() {
		ui.Column(c).Grow(1).Children(func() {
			ui.Text(c, "请求头 Headers").FontSize(11).Bold()
			fieldHint(c, t, "每行 key: value，如 Authorization: Bearer {{token}}")
			headerText := mapToString(step.Headers)
			if ui.TextArea(c, &headerText).Height(64).Changed() {
				step.Headers = stringToMap(headerText, ":")
			}
		})
		ui.Column(c).Grow(1).Children(func() {
			ui.Text(c, "查询参数 Query").FontSize(11).Bold()
			fieldHint(c, t, "每行 key: value，如 page: {{page}}，自动拼到 URL")
			queryText := mapToString(step.Query)
			if ui.TextArea(c, &queryText).Height(64).Changed() {
				step.Query = stringToMap(queryText, ":")
			}
		})
	})

	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "请求体 Body").FontSize(11).Bold()
	})
	bodyEditor(c, t, &step.BodyType, &step.Body, &step.FormData,
		&step.GraphQLQuery, &step.GraphQLVariables, &a.bodyHeight, `{"key": "{{value}}"}`, &a.bodyFmtMsg)

	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if ui.Button(c, "⚡ 试发送此请求").Clicked() {
			a.debugStep(step)
		}
		ui.Text(c, "用当前步骤配置单独发起调试，不影响场景").FontSize(10).TextColor(t.TextMuted)
	})

	ui.Divider(c)
	ui.Text(c, "认证").FontSize(12).Bold()
	authEditor(c, t, &step.AuthType, &step.AuthToken, &step.AuthUser, &step.AuthPass,
		&step.AuthKeyName, &step.AuthKeyValue, &step.AuthKeyIn,
		&step.AuthTokenURL, &step.AuthClientID, &step.AuthClientSecret, &step.AuthScope, &step.AuthGrantType)

	ui.Divider(c)
	a.editorRequestScripts(c, t, step)
	a.editorExtract(c, t, step)
	a.editorAssertList(c, t, step)
}

// editorRequestScripts 请求级脚本（Postman 风格 Pre-request / Tests）。
// Pre-request 在发请求前执行、可改写本次请求变量；Tests 在响应后执行、可读写变量。
func (a *App) editorRequestScripts(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "请求脚本").FontSize(12).Bold()
		langs := []string{"js", "python"}
		lang := step.ScriptLang
		if lang == "" {
			lang = "js"
		}
		if ui.Select(c, &lang, langs).Label("语言").Width(130).Changed() {
			step.ScriptLang = lang
		}
	})
	fieldHint(c, t, "Pre-request 在请求前执行（vars 读写变量，可影响本次 URL/头/体）；Tests 在响应后执行（last 取响应）。")
	ui.Row(c).Gap(10).Children(func() {
		ui.Column(c).Grow(1).Children(func() {
			ui.Text(c, "Pre-request Script").FontSize(11).Bold()
			fieldHint(c, t, "示例：vars.token = 'abc'（JS）")
			resizableTextArea(c, t, &step.PreScript, &a.preScriptHeight,
				"// vars.foo = 'bar'", 48, 400)
		})
		ui.Column(c).Grow(1).Children(func() {
			ui.Text(c, "Tests (PostScript)").FontSize(11).Bold()
			fieldHint(c, t, "示例：vars.saved = last.json.data.id（JS）")
			resizableTextArea(c, t, &step.PostScript, &a.postScriptHeight,
				"// vars.saved = last.json.data.id", 48, 400)
		})
	})
}

// tcpFrameTypeLabels 是帧头类型下拉的中文展示项；存盘值由 tcpFrameTypeValue 映射为英文。
var tcpFrameTypeLabels = []string{"不启用", "字符串头(位数自定义,左补零)", "1字节二进制头(≤255)", "2字节二进制头(64K)"}

func tcpFrameTypeLabel(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "ascii", "ascii8", "string", "string8":
		return "字符串头(位数自定义,左补零)"
	case "bin1", "binary1", "byte":
		return "1字节二进制头(≤255)"
	case "bin2", "binary2", "binary":
		return "2字节二进制头(64K)"
	default:
		return "不启用"
	}
}

func tcpFrameTypeValue(label string) string {
	switch label {
	case "字符串头(位数自定义,左补零)":
		return "ascii"
	case "1字节二进制头(≤255)":
		return "bin1"
	case "2字节二进制头(64K)":
		return "bin2"
	default:
		return ""
	}
}

// editorTCP TCP Socket 步骤配置：连接参数、长短连接、帧头类型、发送数据、读取策略、提取与断言。
func (a *App) editorTCP(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Text(c, "连接目标").FontSize(12).Bold()
	fieldHint(c, t, "连接 TCP 服务端。主机与端口支持 {{变量}}；如需 TLS 加密勾选 TLS。keepalive 填探测周期秒数（如 30），0=关闭（默认）。")
	ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
		ui.TextInput(c, &step.TCPHost).Placeholder("127.0.0.1 或 {{host}}").Grow(2).Label("主机 Host")
		ui.TextInput(c, &step.TCPPort).Placeholder("8080").Grow(1).Label("端口 Port")
		ui.TextInput(c, &step.TCPKeepAlive).Placeholder("0=关闭, 如 30").Grow(1).Label("Keepalive(秒)")
		ui.Checkbox(c, &step.TCPTLS, "TLS")
	})

	ui.Divider(c)
	ui.Text(c, "连接模式（长连接 / 短连接）").FontSize(12).Bold()
	fieldHint(c, t, "short=短连接，每步独立建立、发完即关；long=长连接，按「连接名」跨步骤复用同一 socket，场景结束自动关闭。勾选「关闭连接」可显式释放指定长连接。")
	ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
		connModes := []string{"short", "long"}
		cmode := step.TCPConnMode
		if cmode == "" {
			cmode = "short"
		}
		if ui.Select(c, &cmode, connModes).Label("连接模式").Width(140).Changed() {
			step.TCPConnMode = cmode
		}
		ui.TextInput(c, &step.TCPConnKey).Placeholder("留空为默认连接").Grow(1).Label("连接名 (long 复用键)")
		ui.Checkbox(c, &step.TCPClose, "关闭连接")
	})

	ui.Divider(c)
	ui.Text(c, "帧头类型（发送/读取选 frame 时生效）").FontSize(12).Bold()
	fieldHint(c, t, "字符串头：位数可自定义（如 3/5/8），十进制长度字面值不足左补零，如 8 位头+8 字节报文→00000008。1 字节二进制头：最大 255 字节。2 字节二进制头：最大 65535 字节（64K-1），可切换字节序。")
	ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
		ftype := step.TCPFrameType
		if ftype == "" && strings.TrimSpace(step.TCPFrameHead) != "" {
			ftype = "ascii" // 兼容旧场景：仅配置了位数时按 ascii 展示
		}
		ftypeLabel := tcpFrameTypeLabel(ftype)
		if ui.Select(c, &ftypeLabel, tcpFrameTypeLabels).Label("帧头类型").Width(230).Changed() {
			step.TCPFrameType = tcpFrameTypeValue(ftypeLabel)
			if step.TCPFrameType == "" {
				step.TCPFrameHead = "" // 防止旧位数触发兼容推断，令“不启用”失效
			}
		}
		switch ftype {
		case "ascii":
			ui.TextInput(c, &step.TCPFrameHead).Placeholder("8").Grow(1).Label("位数 (如 3/5/8)")
		case "bin2":
			endianLabel := "大端 (网络序)"
			if strings.EqualFold(strings.TrimSpace(step.TCPFrameEndian), "little") {
				endianLabel = "小端"
			}
			if ui.Select(c, &endianLabel, []string{"大端 (网络序)", "小端"}).Label("字节序").Width(140).Changed() {
				if endianLabel == "小端" {
					step.TCPFrameEndian = "little"
				} else {
					step.TCPFrameEndian = "big"
				}
			}
		}
	})

	ui.Divider(c)
	ui.Text(c, "发送数据").FontSize(12).Bold()
	fieldHint(c, t, "支持 {{变量}} 与转义 \\n \\r\\n \\t \\xNN。line=末尾补换行；raw=原样；hex=十六进制；frame=按上方所选帧头类型（字符串头位数自定义 / 1 字节 / 2 字节二进制）自动加长度头。")
	ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
		sendModes := []string{"line", "raw", "hex", "frame"}
		mode := step.TCPSendMode
		if mode == "" {
			mode = "line"
		}
		if ui.Select(c, &mode, sendModes).Label("发送模式").Width(150).Changed() {
			step.TCPSendMode = mode
		}
		ui.Spacer(c)
	})
	resizableTextArea(c, t, &step.TCPSend, &a.tcpSendHeight,
		"例如：{\"cmd\":\"ping\"} 或 HELLO", 56, 400)

	ui.Divider(c)
	ui.Text(c, "读取响应").FontSize(12).Bold()
	fieldHint(c, t, "line=读一行；all=读到连接关闭/超时；fixed=读固定字节；until=读到结束标记；frame=按上方所选帧头类型先读长度头再读定长报文。响应以文本供提取/断言使用。")
	ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
		readModes := []string{"line", "all", "fixed", "until", "frame"}
		rmode := step.TCPReadMode
		if rmode == "" {
			rmode = "line"
		}
		if ui.Select(c, &rmode, readModes).Label("读取模式").Width(150).Changed() {
			step.TCPReadMode = rmode
		}
		ui.TextInput(c, &step.TCPReadNum).Placeholder("如 1024").Grow(1).Label("字节数 (fixed)")
		ui.TextInput(c, &step.TCPUntil).Placeholder("如 \\n 或 >").Grow(1).Label("结束标记 (until)")
		ui.TextInput(c, &step.TCPTimeout).Placeholder("5000").Grow(1).Label("超时 ms")
	})

	ui.Divider(c)
	a.editorRequestScripts(c, t, step)
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
	fieldHint(c, t, "从响应取值存入变量：来源选 body/header/cookie；jsonpath 填 data.token，regex 填正则表达式；css 用 HTML 选择器（如 li.item a），多元素自动提取为数组，可指定取 text/html/attr。")
	for i := range step.Extract {
		ex := &step.Extract[i]
		idx := i
		ui.Column(c.Key(fmt.Sprintf("ex-%d", idx))).Padding(8).Gap(6).Background(t.Surface).Radius(8).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
				ui.TextInput(c, &ex.Name).Placeholder("变量名，如 token").Grow(1).Label("变量名")
				if ui.Button(c.Key("del"), "移除").Clicked() {
					step.Extract = append(step.Extract[:idx], step.Extract[idx+1:]...)
				}
			})
			// 来源 / 方式 / 默认值 都是短控件，同一行排列。
			ui.Row(c).Gap(8).Children(func() {
				sources := []string{"body", "header", "cookie", "status"}
				ui.Select(c, &ex.Source, sources).Label("来源").Grow(1)
				kinds := []string{"jsonpath", "regex", "whole", "css"}
				ui.Select(c, &ex.Kind, kinds).Label("方式").Grow(1)
				ui.TextInput(c, &ex.Default).Placeholder("可选").Label("默认值").Grow(1)
			})
			placeholder := "data.token 或正则"
			if strings.EqualFold(strings.TrimSpace(ex.Kind), "css") {
				placeholder = "CSS 选择器，如 li.item a.title"
			}
			ui.TextInput(c, &ex.Path).Placeholder(placeholder).Label("路径 / 表达式")
			if strings.EqualFold(strings.TrimSpace(ex.Kind), "css") {
				ui.Row(c).Gap(8).Children(func() {
					fields := []string{"text", "html", "attr"}
					if strings.TrimSpace(ex.CssField) == "" {
						ex.CssField = "text"
					}
					ui.Select(c, &ex.CssField, fields).Label("取值").Grow(1)
					if strings.EqualFold(strings.TrimSpace(ex.CssField), "attr") {
						ui.TextInput(c, &ex.CssAttr).Placeholder("href / content / ...").Label("属性名").Grow(2)
					}
				})
			}
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
	fieldHint(c, t, "校验响应是否符合预期：如 status eq 200、jsonpath 变量 contains 文本；失败即中止（除非勾选失败后继续）。")
	for i := range step.Assert {
		as := &step.Assert[i]
		idx := i
		ui.Column(c.Key(fmt.Sprintf("as-%d", idx))).Padding(8).Gap(6).Background(t.Surface).Radius(8).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
				ui.TextInput(c, &as.Name).Placeholder("断言说明").Grow(1).Label("名称")
				if ui.Button(c.Key("del"), "移除").Clicked() {
					step.Assert = append(step.Assert[:idx], step.Assert[idx+1:]...)
				}
			})
			// 来源 / 操作 同一行。
			ui.Row(c).Gap(8).Children(func() {
				sources := []string{"status", "jsonpath", "body", "header", "variable"}
				ui.Select(c, &as.Source, sources).Label("来源").Grow(1)
				ops := []string{"eq", "ne", "contains", "exists", "gt", "gte", "lt", "lte"}
				ui.Select(c, &as.Op, ops).Label("操作").Grow(1)
			})
			// 路径 / 期望值 同一行。
			ui.Row(c).Gap(8).Children(func() {
				ui.TextInput(c, &as.Path).Placeholder("jsonpath / header 名 / 变量名").Label("路径").Grow(1)
				ui.TextInput(c, &as.Value).Placeholder("期望值").Label("期望值").Grow(1)
			})
		})
	}
}

// editorBranch 条件分支编辑器：可视化拼装条件 + then/else 子步骤。
func (a *App) editorBranch(c *ui.Context, t *ui.Theme, step *model.Step) {
	if step.Branch == nil {
		step.Branch = &model.Branch{Logic: "and", Conditions: []model.Condition{{Op: "大于"}}}
	}
	br := step.Branch

	fieldHint(c, t, "根据变量条件决定走 then 还是 else：例如 amount 大于 0 请求页面1，否则请求页面2。")
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "条件").FontSize(12).Bold()
		ui.Spacer(c)
		logics := []string{"and", "or"}
		if br.Logic == "" {
			br.Logic = "and"
		}
		ui.Select(c, &br.Logic, logics).Width(80)
		ui.Text(c, map[string]string{"and": "全部满足", "or": "任一满足"}[br.Logic]).FontSize(10).TextColor(t.TextMuted)
	})

	// 条件列表：每行 = 变量名 + 操作符 + 比较值
	ops := []string{"大于", "大于等于", "小于", "小于等于", "等于", "不等于", "包含", "不包含", "存在", "为空", "为真"}
	for i := range br.Conditions {
		cond := &br.Conditions[i]
		idx := i
		ui.Column(c.Key(fmt.Sprintf("cond-%d", idx))).Padding(8).Gap(5).Background(t.Surface).Radius(8).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, fmt.Sprintf("条件 %d", idx+1)).FontSize(10).TextColor(t.TextMuted).Width(48)
				ui.Spacer(c)
				if ui.Button(c.Key("del"), "移除").Clicked() {
					br.Conditions = append(br.Conditions[:idx], br.Conditions[idx+1:]...)
				}
			})
			ui.Row(c).Gap(6).Children(func() {
				ui.TextInput(c, &cond.Var).Placeholder("变量名，如 amount").Grow(1)
				if cond.Op == "" {
					cond.Op = "大于"
				}
				ui.Select(c, &cond.Op, ops).Width(90)
				// 存在/为空/为真 无需比较值
				needValue := cond.Op != "存在" && cond.Op != "为空" && cond.Op != "为真"
				if needValue {
					ui.TextInput(c, &cond.Value).Placeholder("值，如 0").Width(90)
				}
			})
		})
	}
	ui.Row(c).Gap(8).Children(func() {
		if ui.Button(c, "+ 添加条件").Clicked() {
			br.Conditions = append(br.Conditions, model.Condition{Op: "等于"})
		}
	})

	ui.Divider(c)
	ui.Text(c, "高级：直接写条件表达式（留空则用上方条件）").FontSize(10).TextColor(t.TextMuted)
	ui.TextInput(c, &br.When).Placeholder("{{amount}} > 0  或  {{len(rows)}} == 0")

	ui.Divider(c)
	ui.Text(c, "条件成立 → then").FontSize(12).Bold().TextColor(t.Accent)
	a.editorBranchChildren(c, t, step, &br.Then, "then")
	ui.Text(c, "条件不成立 → else").FontSize(12).Bold().TextColor(t.TextMuted)
	a.editorBranchChildren(c, t, step, &br.Else, "else")
}

// editorBranchChildren 编辑分支的 then/else 子步骤（复用简化的子步骤表单）。
func (a *App) editorBranchChildren(c *ui.Context, t *ui.Theme, step *model.Step, children *[]model.Step, slot string) {
	ui.Row(c).Gap(8).Children(func() {
		if ui.Button(c.Key("http"), "+ 请求").Clicked() {
			*children = append(*children, model.Step{ID: newID(), Name: "请求", Type: model.StepRequest, Enabled: true, Method: "GET"})
		}
		if ui.Button(c.Key("tcp"), "+ TCP").Clicked() {
			*children = append(*children, model.Step{ID: newID(), Name: "TCP", Type: model.StepTCP, Enabled: true, TCPPort: "8080", TCPSendMode: "line", TCPReadMode: "line", TCPTimeout: "5000"})
		}
		if ui.Button(c.Key("script"), "+ 脚本").Clicked() {
			*children = append(*children, model.Step{ID: newID(), Name: "脚本", Type: model.StepScript, Enabled: true})
		}
		if ui.Button(c.Key("process"), "+ 处理").Clicked() {
			*children = append(*children, model.Step{ID: newID(), Name: "处理", Type: model.StepProcess, Enabled: true, Op: "set"})
		}
	})
	if len(*children) == 0 {
		ui.Text(c, "（无子步骤）").FontSize(10).TextColor(t.TextMuted)
	}
	for i := range *children {
		child := &(*children)[i]
		idx := i
		ui.Column(c.Key(fmt.Sprintf("%s-child-%d", slot, idx))).Padding(8).Gap(5).Background(t.Surface).Radius(8).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, a.stepLabel(*child)).Grow(1)
				if ui.Button(c.Key("del"), "移除").Clicked() {
					*children = append((*children)[:idx], (*children)[idx+1:]...)
				}
			})
			ui.TextInput(c, &child.Name).Label("名称")
			switch child.Type {
			case model.StepRequest:
				ui.Row(c).Gap(6).Children(func() {
					methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE"}
					ui.Select(c, &child.Method, methods).Width(90)
					ui.TextInput(c, &child.URL).Placeholder("{{baseUrl}}/path").Grow(1)
				})
			case model.StepTCP:
				ui.Row(c).Gap(6).Children(func() {
					ui.TextInput(c, &child.TCPHost).Placeholder("主机").Grow(2)
					ui.TextInput(c, &child.TCPPort).Placeholder("端口").Grow(1)
				})
				ui.TextInput(c, &child.TCPSend).Placeholder("发送数据（支持 {{变量}}）")
				ui.TextInput(c, &child.Body).Placeholder("Body（可选）")
				a.editorExtract(c, t, child)
			case model.StepScript:
				ui.TextArea(c, &child.Code).Placeholder("vars.result = 'ok';").Height(60)
			case model.StepProcess:
				ops := []string{"set", "append", "json_to_table", "filter", "sort", "pick", "flatten"}
				op := child.Op
				if op == "" {
					op = "set"
				}
				if ui.Select(c, &op, ops).Label("处理").Changed() {
					child.Op = op
				}
				if child.Args == nil {
					child.Args = map[string]string{}
				}
				argsText := mapToString(child.Args)
				if ui.TextArea(c, &argsText).Height(50).Changed() {
					child.Args = stringToMap(argsText, "=")
				}
			}
		})
	}
}

// editorSwitch 多分支编辑器：多个 case 自上而下匹配，命中即执行；支持 default。
func (a *App) editorSwitch(c *ui.Context, t *ui.Theme, step *model.Step) {
	if step.Switch == nil {
		step.Switch = &model.Switch{}
	}
	sw := step.Switch
	fieldHint(c, t, "多分支：自上而下匹配，命中第一个 case 后执行并跳出；都不满足则执行 default。")

	ops := []string{"大于", "大于等于", "小于", "小于等于", "等于", "不等于", "包含", "不包含", "存在", "为空", "为真"}
	for i := range sw.Cases {
		cs := &sw.Cases[i]
		idx := i
		ui.Column(c.Key(fmt.Sprintf("switch-case-%d", idx))).Padding(10).Gap(6).Background(t.Surface).Radius(8).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, fmt.Sprintf("case %d", idx+1)).FontSize(11).Bold().TextColor(t.Accent)
				ui.TextInput(c, &cs.Name).Placeholder("分支名称（可选）").Grow(1)
				if ui.Button(c.Key("up"), "↑").Width(28).Disabled(idx == 0).Clicked() {
					sw.Cases[idx], sw.Cases[idx-1] = sw.Cases[idx-1], sw.Cases[idx]
				}
				if ui.Button(c.Key("dn"), "↓").Width(28).Disabled(idx == len(sw.Cases)-1).Clicked() {
					sw.Cases[idx], sw.Cases[idx+1] = sw.Cases[idx+1], sw.Cases[idx]
				}
				if ui.Button(c.Key("del"), "移除").Clicked() {
					sw.Cases = append(sw.Cases[:idx], sw.Cases[idx+1:]...)
				}
			})
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if cs.Logic == "" {
					cs.Logic = "and"
				}
				ui.Select(c, &cs.Logic, []string{"and", "or"}).Width(80)
				ui.Text(c, map[string]string{"and": "全部满足", "or": "任一满足"}[cs.Logic]).FontSize(10).TextColor(t.TextMuted)
			})
			for j := range cs.Conditions {
				cond := &cs.Conditions[j]
				ui.Row(c).Gap(6).Children(func() {
					ui.TextInput(c, &cond.Var).Placeholder("变量名").Grow(1)
					if cond.Op == "" {
						cond.Op = "等于"
					}
					ui.Select(c, &cond.Op, ops).Width(90)
					if cond.Op != "存在" && cond.Op != "为空" && cond.Op != "为真" {
						ui.TextInput(c, &cond.Value).Placeholder("值").Width(90)
					}
				})
			}
			if ui.Button(c.Key("addcond"), "+ 条件").Clicked() {
				cs.Conditions = append(cs.Conditions, model.Condition{Op: "等于"})
			}
			ui.Text(c, "高级表达式（留空则用上方条件）").FontSize(10).TextColor(t.TextMuted)
			ui.TextInput(c, &cs.When).Placeholder("{{status}} == 200")
			ui.Text(c, "命中后执行").FontSize(10).TextColor(t.TextMuted)
			a.editorBranchChildren(c, t, step, &cs.Steps, fmt.Sprintf("sw%d", idx))
		})
	}
	if ui.Button(c, "+ 添加 case").Clicked() {
		sw.Cases = append(sw.Cases, model.SwitchCase{Name: fmt.Sprintf("case %d", len(sw.Cases)+1), Logic: "and"})
	}
	ui.Divider(c)
	ui.Text(c, "default（都不命中时执行）").FontSize(11).Bold().TextColor(t.TextMuted)
	a.editorBranchChildren(c, t, step, &sw.Default, "swdefault")
}

// editorParallel 扇出编辑器：配置多个并发子任务及其产物汇合方式。
func (a *App) editorParallel(c *ui.Context, t *ui.Theme, step *model.Step) {
	if step.Parallel == nil {
		step.Parallel = &model.Parallel{Mode: "table", Var: "merged"}
	}
	p := step.Parallel
	ui.Text(c, "扇出：多个子任务并发执行（类似 Go 的 goroutine），全部完成后汇合产物。后续用【扇入合并】步骤做门控。").FontSize(10).TextColor(t.TextMuted)
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
	ui.Text(c, "执行后自动登记：task.<任务名>.status(success/fail) / .output / .error / .duration，以及 task.success/failed/total/allSuccess，供扇入门控表达式引用。").FontSize(10).TextColor(t.TextMuted)
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

	ui.Divider(c)
	for i := range p.Tasks {
		task := &p.Tasks[i]
		idx := i
		ui.Column(c.Key(fmt.Sprintf("parallel-task-%d", idx))).Padding(10).Gap(6).Background(t.Surface).Radius(8).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, fmt.Sprintf("任务 %d", idx+1)).FontSize(11).Bold().TextColor(t.Accent)
				ui.TextInput(c, &task.Name).Placeholder("任务名（object 模式作为 key）").Grow(1)
				if ui.Button(c.Key("del"), "移除").Clicked() {
					p.Tasks = append(p.Tasks[:idx], p.Tasks[idx+1:]...)
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
	if ui.Button(c, "+ 添加子任务").Clicked() {
		p.Tasks = append(p.Tasks, model.ParallelTask{Name: fmt.Sprintf("task %d", len(p.Tasks)+1)})
	}
}

// editorMerge 扇入门控编辑器：用高级表达式检查上游并行任务的状态/产物，不成立即中止。
func (a *App) editorMerge(c *ui.Context, t *ui.Theme, step *model.Step) {
	if step.Merge == nil {
		step.Merge = &model.Merge{When: "task.allSuccess"}
	}
	m := step.Merge
	ui.Text(c, "扇入(门控)：等待上游并行任务全部完成后检查条件；条件不成立则中止整个场景。").FontSize(10).TextColor(t.TextMuted)
	ui.Text(c, "门控表达式（支持 && || 与比较；留空默认 task.allSuccess）").FontSize(11).Bold()
	ui.TextArea(c, &m.When).Placeholder(`task.allSuccess && task.订单查询.output > 0`).Height(70)

	ui.Divider(c)
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
	// 从上游 parallel 步骤收集任务名，提供 task.<名>.status / .output 快捷引用
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
	ui.Text(c, "示例：task.allSuccess  ·  task.A.status == \"success\" && task.B.status == \"success\"  ·  task.failed == 0  ·  len(task) >= 3").FontSize(10).TextColor(t.TextMuted)
	ui.Text(c, "判定：表达式求值为真→放行后续步骤；为假→报错中止场景。任一并行任务失败且需门控时，请勿在【扇出】步骤勾选 failFast，让失败进入状态表由本步骤裁决。").FontSize(10).TextColor(t.TextMuted)
}

func defaultModeStr(mode string) string {
	m := strings.ToLower(strings.TrimSpace(mode))
	if m == "" {
		return "array"
	}
	return m
}

func (a *App) editorLoop(c *ui.Context, t *ui.Theme, step *model.Step) {
	if step.Loop == nil {
		step.Loop = &model.Loop{Mode: "until", Max: 20, PageVar: "page", PageStart: 1, PageStep: 1}
	}
	lp := step.Loop
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "循环模式").FontSize(12).Bold()
		ui.Spacer(c)
		modes := []string{"until", "count", "foreach"}
		ui.Select(c, &lp.Mode, modes).Width(110)
	})
	fieldHint(c, t, map[string]string{
		"until":   "翻页场景常用：每轮执行循环体，直到终止条件成立（如返回空数组）退出。",
		"count":   "固定执行 N 次，适合已知页数的场景。",
		"foreach": "遍历一个数组变量，逐个元素执行循环体。",
	}[lp.Mode])

	switch lp.Mode {
	case "count":
		intInput(c, &lp.Count, "次数 Count", "", 140)
	case "foreach":
		// 三个短变量一行排列。
		ui.Row(c).Gap(8).Children(func() {
			ui.TextInput(c, &lp.ListVar).Placeholder("rows").Label("数组变量 ListVar").Grow(1)
			ui.TextInput(c, &lp.ItemVar).Placeholder("item").Label("元素变量 ItemVar").Grow(1)
			ui.TextInput(c, &lp.IndexVar).Placeholder("index").Label("索引变量 IndexVar").Grow(1)
		})
		fieldHint(c, t, "逐项取出数组元素：当前元素存入 ItemVar，下标存入 IndexVar。")
	case "until":
		ui.TextInput(c, &lp.BreakWhen).Placeholder("{{len(rows)}} == 0").Label("终止条件 BreakWhen")
		fieldHint(c, t, "表达式为真时退出循环，例如响应数组为空：{{len(rows)}} == 0。")
	}
	// 页码相关字段短，一行排布：页码变量 / 起始页 / 步长 / 最大轮数。
	ui.Row(c).Gap(8).Children(func() {
		pageVar := lp.PageVar
		if ui.TextInput(c, &pageVar).Label("页码变量").Grow(1).Changed() {
			lp.PageVar = pageVar
		}
		intInput(c, &lp.PageStart, "起始页", "", 80)
		intInput(c, &lp.PageStep, "步长", "", 70)
		intInput(c, &lp.Max, "最大轮数", "", 88)
	})
	fieldHint(c, t, "翻页时在 URL 用 {{页码变量}}（默认 page）；最大轮数用于兜底防止死循环。")

	// 累积去重（P1）：循环结束后对累积数组按字段去重并提示重复数。
	ui.Divider(c)
	ui.Text(c, "累积去重（可选）").FontSize(11).Bold()
	ui.Row(c).Gap(8).Children(func() {
		ui.TextInput(c, &lp.DedupVar).Placeholder("allRows").Label("去重数组变量").Grow(1)
		ui.TextInput(c, &lp.DedupField).Placeholder("link").Label("去重字段").Grow(1)
		ui.TextInput(c, &lp.DedupStat).Placeholder("dupCount").Label("重复数变量").Grow(1)
	})
	fieldHint(c, t, "循环结束后对「去重数组变量」按「去重字段」去重（字段为空按整行），去掉的重复数写入日志；填了「重复数变量」也会写入该变量。留空去重数组变量则不启用。")

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
				ops := []string{"set", "append", "json_to_table", "filter", "sort", "pick", "flatten", "dedup", "format_time"}
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
	ops := []string{"set", "append", "json_to_table", "filter", "sort", "pick", "flatten", "dedup", "format_time", "join", "base64encode", "base64decode", "urlencode", "urldecode"}
	op := step.Op
	if op == "" {
		op = "json_to_table"
	}
	if ui.Select(c, &op, ops).Label("处理操作").Width(180).Changed() {
		step.Op = op
	}
	hints := map[string]string{
		"set":           "设置变量：name=变量名, value=值",
		"append":        "循环内累积：from=源变量, to=数组变量（每轮追加一项）",
		"json_to_table": "JSON 数组转表格：from=源变量, to=目标变量",
		"filter":        "过滤：from, field, op(eq/ne/contains/gt...), value, to",
		"sort":          "排序：from, field, order(asc/desc), to",
		"pick":          "取字段：from, fields(逗号分隔), to",
		"flatten":       "合并数组：from(逗号分隔的多个变量), to",
		"dedup":         "去重：from=源数组变量, to=目标变量；field 按对象字段去重，stat 写入去掉的重复数",
		"format_time":   "时间格式化：from, in(unix/unixms), layout, to",
		"join":          "拼接：values(a,b,c), sep, to",
		"base64encode":  "Base64 编码：from=源变量, to=目标变量",
		"base64decode":  "Base64 解码：from=源变量, to=目标变量",
		"urlencode":     "URL 编码：from=源变量, to=目标变量",
		"urldecode":     "URL 解码：from=源变量, to=目标变量",
	}
	fieldHint(c, t, "参数（每行 key=value，支持 {{变量}}）")
	if h, ok := hints[op]; ok {
		fieldHint(c, t, "「"+op+"」："+h)
	}
	if step.Args == nil {
		step.Args = map[string]string{}
	}
	argsText := mapToString(step.Args)
	if ui.TextArea(c, &argsText).Height(120).Changed() {
		step.Args = stringToMap(argsText, "=")
	}
}

func (a *App) editorScript(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "自定义脚本").FontSize(12).Bold()
		langs := []string{"js", "python"}
		lang := step.ScriptLang
		if lang == "" {
			lang = "js"
		}
		if ui.Select(c, &lang, langs).Label("语言").Changed() {
			step.ScriptLang = lang
		}
	})
	if strings.EqualFold(step.ScriptLang, "python") || strings.EqualFold(step.ScriptLang, "py") {
		fieldHint(c, t, "Python（Starlark 子集）：vars[\"k\"] = v 读写变量，last[\"status\"/\"json\"] 取响应，log() 打印。")
		fieldHint(c, t, "支持 if/for/函数/列表推导；顶层不能直接写 for（放进函数里调用），无 while/try/class。")
		fieldHint(c, t, "示例：vars[\"token\"] = last[\"json\"][\"data\"][\"token\"];  log(\"取到 token\")")
	} else {
		fieldHint(c, t, "JavaScript（goja）：vars（读写变量）、last（最近响应 status/body/json）、log()、JSON.stringify/parse。")
		fieldHint(c, t, "示例：vars.token = last.json.data.token;  log('取到 token');")
	}
	// 内容格式化：脚本通常不是结构化数据，但常被用来粘贴 JSON 样例，提供一键美化。
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if ui.Button(c, "✨ 格式化").Tooltip("若内容是 JSON/XML/键值对则一键美化").Clicked() {
			out, kind, changed := FormatContent(step.Code)
			if changed {
				step.Code = out
				a.scriptFmtMsg = "已按 " + kind + " 格式化"
			} else {
				a.scriptFmtMsg = "脚本内容无需格式化（仅支持 JSON/XML/键值对）"
			}
		}
		if a.scriptFmtMsg != "" {
			ui.Text(c, a.scriptFmtMsg).FontSize(10).TextColor(t.TextMuted)
		}
	})
	ui.TextArea(c, &step.Code).Height(240)
}

func (a *App) editorAssert(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Text(c, "独立断言（针对最近一次响应）").FontSize(12).Bold()
	a.editorAssertList(c, t, step)
}

func (a *App) editorExport(c *ui.Context, t *ui.Theme, step *model.Step) {
	fieldHint(c, t, "把提取到的数组/表格变量导出成文件；相对路径写到 exe 同目录。")
	formats := []string{"xlsx", "csv", "txt", "json"}
	f := step.Format
	if f == "" {
		f = "xlsx"
	}
	ui.Row(c).Gap(8).Children(func() {
		ui.TextInput(c, &step.From).Placeholder("table / rows").Label("数据源变量 From").Grow(1)
		if ui.Select(c, &f, formats).Label("格式").Width(110).Changed() {
			step.Format = f
		}
	})
	ui.Row(c).Gap(8).Children(func() {
		ui.TextInput(c, &step.Path).Placeholder("result.xlsx").Label("输出路径").Grow(1)
	})
	fieldHint(c, t, "xlsx/csv 生成表格，txt 生成纯文本，json 原样输出；留空路径按格式给默认文件名。")
}

// ---------- 底部：日志 / 结果表 / 变量 三视图 ----------
func (a *App) console(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Padding(8, 10).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Segmented(c, &a.consoleView, "日志", "结果表", "变量")
			ui.Spacer(c)
			if len(a.dataResults) > 0 {
				passed, failed := 0, 0
				for _, r := range a.dataResults {
					if r.Result != nil && r.Result.OK {
						passed++
					} else {
						failed++
					}
				}
				ui.Text(c, fmt.Sprintf("数据驱动 %d 轮 · %d 通过 / %d 失败", len(a.dataResults), passed, failed)).FontSize(11).TextColor(t.Accent)
			} else if a.lastResult != nil {
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
		emptyState(c, t, "暂无可展示的表格数据", "运行场景后，从响应提取的数组/JSON 数据会在此以表格显示")
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

// variableKind 返回变量的展示类型与元素/字段数量，供变量预览表格显示。
func variableKind(v interface{}) (string, int) {
	switch x := v.(type) {
	case []interface{}:
		return "数组", len(x)
	case []string:
		return "数组", len(x)
	case []map[string]interface{}:
		return "数组", len(x)
	case map[string]interface{}:
		return "对象", len(x)
	case map[string]string:
		return "对象", len(x)
	case string:
		var parsed interface{}
		t := strings.TrimSpace(x)
		if len(t) > 0 && (t[0] == '[' || t[0] == '{') && json.Unmarshal([]byte(t), &parsed) == nil {
			return variableKind(parsed)
		}
		return "文本", 0
	case nil:
		return "空", 0
	default:
		return "标量", 0
	}
}

// variableCell 把单个变量值渲染为可复制展示文本；数组/对象返回简短摘要。
func variableCell(v interface{}) string {
	b, err := json.Marshal(v)
	if err == nil && len(b) > 0 && b[0] == '"' {
		return string(b[1 : len(b)-1])
	}
	s := string(b)
	if len(s) > 160 {
		return s[:160] + "…"
	}
	return s
}

// variableElements 把数组/对象展开为 (键, 值) 列表，供变量预览展开区展示。
func variableElements(v interface{}) [][2]string {
	list, ok := toAnySlice(v)
	if ok {
		out := make([][2]string, 0, len(list))
		for i, item := range list {
			out = append(out, [2]string{fmt.Sprintf("[%d]", i), variableCell(item)})
		}
		return out
	}
	if m, ok := toAnyMap(v); ok {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make([][2]string, 0, len(keys))
		for _, k := range keys {
			out = append(out, [2]string{k, variableCell(m[k])})
		}
		return out
	}
	return nil
}

func toAnySlice(v interface{}) ([]interface{}, bool) {
	switch x := v.(type) {
	case []interface{}:
		return x, true
	case []string:
		out := make([]interface{}, 0, len(x))
		for _, s := range x {
			out = append(out, s)
		}
		return out, true
	case []map[string]interface{}:
		out := make([]interface{}, 0, len(x))
		for _, m := range x {
			out = append(out, m)
		}
		return out, true
	case string:
		var parsed interface{}
		t := strings.TrimSpace(x)
		if len(t) > 0 && t[0] == '[' && json.Unmarshal([]byte(t), &parsed) == nil {
			if list, ok := parsed.([]interface{}); ok {
				return list, true
			}
		}
	}
	return nil, false
}

func toAnyMap(v interface{}) (map[string]interface{}, bool) {
	switch x := v.(type) {
	case map[string]interface{}:
		return x, true
	case map[string]string:
		out := make(map[string]interface{}, len(x))
		for k, val := range x {
			out[k] = val
		}
		return out, true
	case string:
		var parsed interface{}
		t := strings.TrimSpace(x)
		if len(t) > 0 && t[0] == '{' && json.Unmarshal([]byte(t), &parsed) == nil {
			if m, ok := parsed.(map[string]interface{}); ok {
				return m, true
			}
		}
	}
	return nil, false
}

func (a *App) variablesView(c *ui.Context, t *ui.Theme) {
	if a.lastResult == nil {
		ui.Scroll(c).Grow(1).Padding(0, 12, 8, 12).Children(func() {
			ui.Text(c, "运行后在此查看全部变量（含步骤间传递的数据）").FontSize(11).TextColor(t.TextMuted)
		})
		return
	}
	keys := make([]string, 0, len(a.lastResult.Vars))
	for k := range a.lastResult.Vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	ui.Scroll(c).Grow(1).Padding(0, 12, 8, 12).Gap(2).Children(func() {
		// 表头
		ui.Row(c).Gap(8).Padding(2, 6).Children(func() {
			ui.Text(c, "变量名").FontSize(10).Bold().TextColor(t.TextMuted).Width(180)
			ui.Text(c, "类型/数量").FontSize(10).Bold().TextColor(t.TextMuted).Width(90)
			ui.Text(c, "值").FontSize(10).Bold().TextColor(t.TextMuted).Grow(1)
		})
		ui.Divider(c)
		for _, k := range keys {
			v := a.lastResult.Vars[k]
			kind, n := variableKind(v)
			cell := variableCell(v)
			elements := variableElements(v)
			if len(elements) == 0 || len(elements) == 1 && kind == "文本" {
				// 标量：一行展示完整值
				ui.Row(c.Key(k)).Gap(8).Padding(2, 6).Children(func() {
					ui.Text(c, k).FontSize(11).Bold().Width(180)
					ui.Text(c, kind).FontSize(10).TextColor(t.TextMuted).Width(90)
					ui.Text(c, cell).FontSize(11).TextColor(t.TextMuted).Grow(1)
				})
				continue
			}
			// 数组/对象：点击变量名展开/收起，展开区逐元素成表
			key := k
			open := a.varExpanded[k]
			arrow := "▸ "
			if open {
				arrow = "▾ "
			}
			ui.Row(c.Key(k)).Gap(8).Padding(2, 6).Children(func() {
				if ui.Button(c, arrow+key).Width(180).Clicked() {
					a.varExpanded[key] = !open
				}
				ui.Text(c, fmt.Sprintf("%s · %d", kind, n)).FontSize(10).TextColor(t.Accent).Width(90)
				ui.Text(c, cell).FontSize(11).TextColor(t.TextMuted).Grow(1).SingleLine()
			})
			if open {
				ui.Column(c.Key(k+"-items")).Padding(0, 0, 4, 28).Gap(1).Children(func() {
					for _, el := range elements {
						ui.Row(c).Gap(8).Padding(1, 4).Children(func() {
							ui.Text(c, el[0]).FontSize(10).TextColor(t.Accent).Width(120)
							ui.Text(c, el[1]).FontSize(10).SingleLine().Grow(1)
						})
					}
				})
			}
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

// truncateRunes 按 rune 截断字符串（避免中文按字节截断乱码）。
func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// showErrorDetail 弹框显示完整错误，供状态栏失败信息点击查看。
func (a *App) showErrorDetail(detail string) {
	go func() {
		_, _ = mygo.Dialog.Message(mygo.MessageOptions{
			Parent:  a.win,
			Type:    mygo.MessageError,
			Title:   "运行失败详情",
			Message: detail,
			Buttons: []string{"确定"},
		})
	}()
}

func (a *App) statusBar(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Padding(6, 12).AlignItems(ui.Center).Children(func() {
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
func (a *App) newScenario() {
	a.showTemplateWizard = true
	a.templateIndex = 0
}

// applyTemplate 载入选中的场景模板并关闭向导。
func (a *App) applyTemplate(tpl ScenarioTemplate) {
	a.scenario = tpl.Build()
	a.filePath = ""
	a.selectedStep = -1
	a.mainTab = 0
	a.adding = false
	a.resetResult()
	a.showTemplateWizard = false
	a.log("已从模板新建场景：" + tpl.Name)
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
		mygo.RunOnMain(func() { a.applyLoadedScenario(&sc, path) })
	}()
}

// applyLoadedScenario 统一应用外部载入的场景，保证各项状态一致、无半初始化。
func (a *App) applyLoadedScenario(sc *model.Scenario, path string) {
	if sc == nil {
		sc = model.NewScenario("空场景")
	}
	if sc.Variables == nil {
		sc.Variables = map[string]string{}
	}
	if sc.Steps == nil {
		sc.Steps = []model.Step{}
	}
	a.scenario = sc
	a.filePath = path
	a.selectedStep = -1
	a.adding = false
	a.showImport = false
	a.resetResult()
	a.log("已打开: " + path)
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

// pickDataFile 选择数据文件后立即启动数据驱动运行。
func (a *App) pickDataFile() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Title:   "选择数据文件 (CSV/JSON)",
			Filters: []mygo.FileFilter{{Name: "数据文件", Extensions: []string{"csv", "json", "txt"}}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		mygo.RunOnMain(func() {
			a.dataFile = paths[0]
			a.runDataDriven()
		})
	}()
}

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
	case model.StepTCP:
		return "TCP Socket"
	case model.StepLoop:
		return "循环/翻页"
	case model.StepBranch:
		return "条件分支"
	case model.StepSwitch:
		return "多分支 switch"
	case model.StepParallel:
		return "并行扇出"
	case model.StepMerge:
		return "扇入合并(门控)"
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

// stringToMap 把多行文本解析为 key→value，兼容两种写法：
//   - "key: value"（界面提示格式，也是 Chrome DevTools「Copy request headers」的格式）
//   - "key = value"（mapToString 的输出格式）
//
// 以行内最先出现的 ":" 或 "=" 作为分隔符且只切一次，确保值里的 "=" 不会被误当分隔符
// （Cookie、Bearer token 等经常含 "="）。以 ":" 开头的 HTTP/2 伪头（:authority、:method 等）
// 会被跳过——Go 的 HTTP 客户端不接受这类头。
// bodyTypes 请求体类型下拉选项（Postman 风格）。
var bodyTypes = []string{"none", "raw", "form-data", "urlencoded", "binary", "graphql"}

// authTypes 认证方式下拉选项。
var authTypes = []string{"none", "bearer", "basic", "apikey", "oauth2"}

// bodyEditor 渲染请求体类型选择 + 对应编辑控件。 pointers 让步骤编辑器与调试面板复用。
func bodyEditor(c *ui.Context, t *ui.Theme, bodyType *string, body *string, form *[]model.FormField,
	gqlQuery *string, gqlVars *string, height *float32, placeholder string, fmtMsg *string) {
	bt := *bodyType
	if bt == "" {
		bt = "raw"
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "Body 类型").FontSize(11).Bold()
		if ui.Select(c, &bt, bodyTypes).Width(130).Changed() {
			*bodyType = bt
		}
	})
	switch strings.ToLower(bt) {
	case "none", "raw", "binary":
		hint := "支持 {{变量}}；JSON 会自动设 Content-Type。"
		if bt == "binary" {
			hint = "填写要上传的文件路径，以 application/octet-stream 发送。"
		}
		fieldHint(c, t, hint)
		if fmtMsg != nil {
			formattedTextArea(c, t, body, height, placeholder, 40, 800, fmtMsg)
		} else {
			resizableTextArea(c, t, body, height, placeholder, 40, 800)
		}
	case "form-data", "urlencoded":
		fieldHint(c, t, "逐行添加字段；类型选 file 时值为本地文件路径（form-data 支持上传）。")
		fields := *form
		for i := range fields {
			f := &fields[i]
			idx := i
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.TextInput(c, &f.Key).Placeholder("字段名").Width(130)
				ft := f.Type
				if ft == "" {
					ft = "text"
				}
				if bt == "urlencoded" {
					ft = "text" // urlencoded 无文件类型
				}
				if ui.Select(c, &ft, []string{"text", "file"}).Width(76).Changed() {
					f.Type = ft
				}
				ui.TextInput(c, &f.Value).Placeholder("值或文件路径").Grow(1)
				if ui.Button(c.Key("delf"), "×").Width(26).Clicked() {
					*form = append(fields[:idx], fields[idx+1:]...)
				}
			})
		}
		if ui.Button(c, "+ 添加字段").Clicked() {
			*form = append(fields, model.FormField{Type: "text"})
		}
	case "graphql":
		fieldHint(c, t, "Query 填写 GraphQL 查询；Variables 为 JSON 对象（可留空 {}）。")
		ui.Text(c, "Query").FontSize(10).TextColor(t.TextMuted)
		resizableTextArea(c, t, gqlQuery, height, "query { me { id } }", 40, 600)
		ui.Text(c, "Variables (JSON)").FontSize(10).TextColor(t.TextMuted)
		ui.TextArea(c, gqlVars).Placeholder("{}").Height(80)
	}
}

// authEditor 渲染认证方式选择与对应凭据输入，复用指针以兼容步骤编辑器与调试面板。
func authEditor(c *ui.Context, t *ui.Theme, authType *string, token, user, pass *string,
	keyName, keyValue, keyIn *string, tokenURL, clientID, clientSecret, scope, grantType *string) {
	at := *authType
	if at == "" {
		at = "none"
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "认证方式").FontSize(11).Bold()
		if ui.Select(c, &at, authTypes).Width(130).Changed() {
			*authType = at
		}
	})
	fieldHint(c, t, "认证头/参数自动加入请求；none 表示不认证。")
	switch strings.ToLower(at) {
	case "bearer":
		ui.TextInput(c, token).Placeholder("{{token}}").Label("Bearer Token")
	case "basic":
		ui.Row(c).Gap(8).Children(func() {
			ui.TextInput(c, user).Placeholder("用户名").Label("用户名").Grow(1)
			ui.TextInput(c, pass).Password().Placeholder("密码").Label("密码").Grow(1)
		})
	case "apikey":
		fieldHint(c, t, "在请求头或 URL 查询参数中加入指定的 Key。")
		ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
			ui.TextInput(c, keyName).Placeholder("如 X-API-Key").Label("参数名").Grow(1)
			ki := *keyIn
			if ki == "" {
				ki = "header"
			}
			if ui.Select(c, &ki, []string{"header", "query"}).Width(96).Label("位置").Changed() {
				*keyIn = ki
			}
		})
		ui.TextInput(c, keyValue).Placeholder("{{apiKey}}").Label("参数值")
	case "oauth2":
		fieldHint(c, t, "自动向 Token URL 换取 access_token 并以 Bearer 注入；同一凭据会缓存复用。")
		ui.TextInput(c, tokenURL).Placeholder("https://api.example.com/oauth/token").Label("Token URL")
		ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
			gt := *grantType
			if gt == "" {
				gt = "client_credentials"
			}
			if ui.Select(c, &gt, []string{"client_credentials", "password"}).Width(170).Label("授权类型").Changed() {
				*grantType = gt
			}
			ui.TextInput(c, scope).Placeholder("可选，如 read write").Label("Scope").Grow(1)
		})
		ui.Row(c).Gap(8).Children(func() {
			ui.TextInput(c, clientID).Placeholder("Client ID").Label("Client ID").Grow(1)
			ui.TextInput(c, clientSecret).Password().Placeholder("Client Secret").Label("Client Secret").Grow(1)
		})
		if strings.EqualFold(*grantType, "password") {
			ui.Row(c).Gap(8).Children(func() {
				ui.TextInput(c, user).Placeholder("用户名").Label("用户名").Grow(1)
				ui.TextInput(c, pass).Password().Placeholder("密码").Label("密码").Grow(1)
			})
		}
	}
}

// formDataToString 把表单字段序列化为多行文本，值以 @ 开头表示文件字段。
func formDataToString(fields []model.FormField) string {
	lines := make([]string, 0, len(fields))
	for _, f := range fields {
		v := f.Value
		if strings.EqualFold(f.Type, "file") && !strings.HasPrefix(v, "@") {
			v = "@" + v
		}
		lines = append(lines, f.Key+": "+v)
	}
	return strings.Join(lines, "\n")
}

// stringToFormData 解析表单文本：每行 key: value；值以 @ 开头视为文件路径（Type=file）。
func stringToFormData(s string) []model.FormField {
	out := []model.FormField{}
	for _, raw := range strings.Split(s, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		i := strings.IndexAny(line, ":=")
		if i <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:i])
		val := strings.TrimSpace(line[i+1:])
		if key == "" {
			continue
		}
		f := model.FormField{Key: key, Type: "text", Value: val}
		if strings.HasPrefix(val, "@") && len(val) > 1 {
			f.Type = "file"
			f.Value = strings.TrimSpace(val[1:])
		}
		out = append(out, f)
	}
	return out
}

func stringToMap(s, _ string) map[string]string {
	out := map[string]string{}
	for _, raw := range strings.Split(s, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		i := strings.IndexAny(line, ":=")
		if i <= 0 { // 无分隔符，或以 ":" 开头（HTTP/2 伪头 / 空 key）
			continue
		}
		key := strings.TrimSpace(line[:i])
		if key == "" {
			continue
		}
		out[key] = strings.TrimSpace(line[i+1:])
	}
	return out
}

var _ = filepath.Join
