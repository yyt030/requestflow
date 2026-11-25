// Package ui 基于 mygo 原生 UI 实现场景可视化配置与一键执行。
package ui

import (
	"fmt"
	"os"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"requestflow/engine"
	"requestflow/history"
	"requestflow/model"
	"requestflow/version"
)

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
	listState       ui.ListState
	splitSide       float32
	splitSidePrev   float32 // 侧栏折叠前宽度
	splitLog        float32
	splitEditorResp float32 // 编排区「配置 / 响应」垂直分屏百分比（默认 50/50）

	// "新建步骤"菜单
	adding     bool
	sidebarTab int    // 左侧栏标签：0=工作区 1=变量（Postman 风格）
	stepSearch string // 左侧文件树步骤搜索过滤
	showSteps  bool   // 文件树"步骤"文件夹节点是否展开

	// 底部控制台视图：0=日志 1=结果表 2=变量
	consoleView int
	// 结果表：选中的数组变量名与表格状态
	resultVar     string
	resultState   ui.ListState
	resultCols    []ui.TableColumn
	resultCopyCol int // 结果表「复制列」选中的列序号，-1=全部
	// 变量预览（P1）：数组/对象变量的可展开状态，按变量名记录。
	varExpanded map[string]bool

	// cURL 导入面板
	showImport bool
	importText string
	importMsg  string

	// 场景模板/新建向导
	showTemplateWizard bool
	templateIndex      int

	// 主工作区标签：0=步骤编排 1=接口调试 2=运行结果（Postman 风格标签式工作区）
	mainTab int
	stepTab int // 步骤编排子标签（接口调试风格），切换步骤时按类型 clamp

	tabDirty      map[string]string // 步骤标签修改标记：key=步骤ID|标签名, value=已确认内容指纹（详见 app_tab_dirty.go）
	debugTabDirty map[string]string // 调试请求子标签修改标记：key=标签名, value=已确认内容指纹（详见 app_debug_tabs.go）

	// 接口调试面板（调试已合并进步骤编排，debug* 仅存一次发送的快照）
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
	debugView         int // 0=响应体 1=响应头 2=Cookie 3=代码（代码内含 curl 片段）
	debugReqTab       int // 0=Params 1=Headers 2=Body 3=Auth（Postman 风格子标签）
	debugResult       *engine.DebugResult
	debugSending      bool
	splitDebug        float32
	tcpDebug          tcpDebugState // TCP 步骤单步调试状态（详见 app_debug_tcp.go）

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

	// process 结构化表单：「原始参数」折叠区展开状态（按步骤 ID/名称）。
	processRawOpen map[string]*bool

	// VSCode 风格侧边栏：编辑弹窗的开关与缓冲。
	// 场景名 / 环境名走 Modal 重命名，变量 key=value 走 Modal 编辑，
	// 取代原先常驻的大文本框与铺开的行内输入框。
	nameEditOpen bool
	nameEditBuf  string
	envEditOpen  bool
	envEditBuf   string
	// 变量编辑弹窗：target 为 "global" 或环境名，origKey 为空表示新建。
	varEditOpen    bool
	varEditTarget  string
	varEditOrigKey string
	varEditKey     string
	varEditValue   string

	// 场景工作区：一个目录下的多个场景 JSON，点击列表即可切换编辑（详见 app_workspace.go）。
	workspace workspaceState

	// ── 布局重构（Postman 风格五区）新增状态 ──
	// 全局搜索：顶栏输入，过滤侧栏列表。
	globalSearch string
	// 侧栏折叠与三块导航：0=Collections 1=Flows 2=History。
	sidebarCollapsed bool
	sidebarSection   int
	// 侧栏右键菜单：当前右键目标类型与索引（"-1" 表示空白区），以及菜单开关。
	sideMenuOpen   bool
	sideMenuKind   string           // "collection" | "flow" | "history" | "blank"
	sideMenuTarget int              // 步骤/场景索引
	ctxMenuOpens   map[string]*bool // 每行独立的右键菜单开关（按 key 持久）
	// 右侧抽屉：默认收起；section 0=环境变量 1=代码片段 2=历史详情 3=流程变量。
	drawerOpen    bool
	drawerSection int
	drawerWidth   float32
	// 中央多标签：每个标签绑定一个步骤/场景/结果视图，可关闭、可拖动排序。
	workTabs    []workTab
	activeWork  int // 当前激活标签在 workTabs 中的索引
	nextTabSeq  int
	tabDragFrom int // 标签拖动源索引，-1 表示无
	// 顶栏环境选择弹层开关。
	headerEnvOpen bool
	// 响应区：JSON/Text 切换、过滤、JSONPath 过滤。
	respJSONMode int // Segmented 索引：0=Text 原文，1=JSON 美化
	respFilter   string
	respJSONPath string
}

// workTab 中央工作区的一个标签页。
type workTab struct {
	id    string
	label string
	kind  string // "step" | "flow" | "result"
	step  int    // kind=step 时对应的步骤索引
}

// newWorkTab 生成一个带自增 ID 的标签。
func (a *App) newWorkTab(label, kind string, step int) workTab {
	a.nextTabSeq++
	return workTab{id: fmt.Sprintf("tab-%d", a.nextTabSeq), label: label, kind: kind, step: step}
}

// Run 启动 GUI。
func Run() {
	app := &App{
		scenario:         demoScenario(),
		selectedStep:     -1,
		splitSide:        280,
		splitSidePrev:    280,
		splitLog:         220,
		splitDebug:       50,
		splitEditorResp:  50,
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
		sidebarTab:       0,
		showSteps:        true,
		varExpanded:      map[string]bool{},
		tabDirty:         map[string]string{},
		debugTabDirty:    map[string]string{},
		sidebarSection:   0,
		drawerWidth:      300,
		tabDragFrom:      -1,
		ctxMenuOpens:     map[string]*bool{},
		respJSONMode:     1,
	}
	app.refreshDebugTabBaseline() // 记录初始调试请求配置为基线
	app.logs = append(app.logs, "就绪。打开或新建场景，配置步骤后点击【一键运行】。")
	mygo.App.WhenReady(func() {
		app.win = mygo.NewWindow(mygo.WindowOptions{
			Title:   "RequestFlow " + version.Short() + " · request流程自动化",
			Width:   1440,
			Height:  900,
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
		// 左侧栏（可折叠）+ 中央工作区 + 右侧抽屉。
		// 抽屉收起时只留 44px 窄条，展开时 320px，避免挤占中央工作区。
		if a.drawerOpen {
			a.drawerWidth = 320
		} else {
			a.drawerWidth = 44
		}
		// 侧栏折叠：记住宽度并压到 44；展开时恢复到合理值。
		if a.sidebarCollapsed {
			if a.splitSide > 100 {
				a.splitSidePrev = a.splitSide
			}
			a.splitSide = 44
		} else if a.splitSide < 100 && a.splitSidePrev > 100 {
			a.splitSide = a.splitSidePrev
		}
		ui.Split(c, &a.splitSide, func() { a.sidebar(c, t) }, func() {
			// 抽屉作为第一个区（宽度由 drawerWidth 控制），主区占满剩余。
			ui.Split(c, &a.drawerWidth, func() { a.rightDrawer(c, t) }, func() {
				a.mainArea(c, t)
			}).Grow(1)
		}).Grow(1)
		// 新建场景向导：以 Modal 覆盖窗口，选择模板或空白场景。
		ui.Modal(c, &a.showTemplateWizard, func() {
			a.templateWizard(c, t)
		})
		a.sidebarModals(c, t)
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
