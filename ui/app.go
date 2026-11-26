// Package ui 基于 mygo 原生 UI 实现场景可视化配置与一键执行。
package ui

import (
	"fmt"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"requestflow/engine"
	"requestflow/history"
	"requestflow/model"
)

// App 保存界面所有可变状态。view 是纯函数：从状态构建界面。
type App struct {
	win      *mygo.Window
	scenario *model.Scenario
	filePath string

	// 统一侧栏树：各场景 / History 合为一棵可展开树。
	// treeTopExpanded 记录顶层分组展开态；sceneExpanded 按场景路径记录场景文件夹展开态；
	// sceneStepCache 缓存非当前场景的步骤摘要，避免展开时反复读盘。
	treeTopExpanded map[string]bool
	sceneExpanded   map[string]bool
	sceneStepCache  map[string][]model.Step

	// 流程编排视图：左侧步骤列表 / 右侧步骤配置的宽度分割。
	splitFlowSteps float32

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

	// 请求编辑区大文本框高度（Params / Headers / 脚本步骤 Code，可拖拽调整）
	reqQueryHeight   float32
	reqHeadersHeight float32
	scriptCodeHeight float32

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
	// 步骤名内联编辑：双击中央顶部名称进入编辑态，回车/失焦提交。
	stepNameEditing bool
	stepNameBuf     string
	envEditOpen     bool
	envEditBuf      string
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
	// 侧栏折叠状态。
	sidebarCollapsed bool
	sidebarSection   int // 历史保留字段，当前统一树不再使用分块导航
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
	// 响应区：原始/格式化/Hex 切换、过滤、JSONPath 过滤。
	respBodyMode int // Segmented 索引：0=原始(Raw) 服务端原文，1=格式化(Pretty) 自动美化，2=Hex 十六进制转储
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
