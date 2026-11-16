// Package model 定义场景配置的数据模型。
// 一个场景 = 一个 JSON 文件，内含有序步骤列表与全局设置。
package model

// Scenario 一个可独立运行的自动化场景，对应磁盘上的一个 JSON 文件。
type Scenario struct {
	Name      string            `json:"name"`
	BaseURL   string            `json:"baseUrl,omitempty"`
	Variables map[string]string `json:"variables,omitempty"`
	Steps     []Step            `json:"steps"`
}

// NewScenario 返回一个带默认值的空场景。
func NewScenario(name string) *Scenario {
	return &Scenario{
		Name:      name,
		Variables: map[string]string{},
		Steps:     []Step{},
	}
}

// StepType 步骤类型。
type StepType string

const (
	StepRequest  StepType = "request"  // 发送 HTTP 请求，可提取变量、断言
	StepLoop     StepType = "loop"     // 循环执行子步骤，用于翻页/遍历
	StepBranch   StepType = "branch"   // 条件分支：when 成立执行 then，否则执行 else
	StepSwitch   StepType = "switch"   // 多分支：自上而下匹配第一个 case，否则 default
	StepParallel StepType = "parallel" // 并行结果汇合：多个子任务并行执行后汇合产物
	StepProcess  StepType = "process"  // 内置数据处理
	StepScript   StepType = "script"   // 自定义 JavaScript 处理响应
	StepExport   StepType = "export"   // 导出到 Excel / 文本 / JSON
	StepAssert   StepType = "assert"   // 独立断言
)

// Step 单个执行步骤。字段按类型取用，未使用的字段留空即可。
type Step struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Type    StepType `json:"type"`
	Enabled bool     `json:"enabled"`

	// --- request 字段 ---
	Method  string            `json:"method,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Query   map[string]string `json:"query,omitempty"`
	Body    string            `json:"body,omitempty"`
	// Auth: bearer token 从变量读取（如 {{token}}）；cookie 由全局 CookieJar 自动管理
	AuthType  string `json:"authType,omitempty"` // none | bearer | basic
	AuthToken string `json:"authToken,omitempty"` // bearer: "{{token}}"
	AuthUser  string `json:"authUser,omitempty"`
	AuthPass  string `json:"authPass,omitempty"`

	Extract []Extract `json:"extract,omitempty"`
	Assert  []Assert  `json:"assert,omitempty"`

	// --- loop 字段 ---
	Loop *Loop `json:"loop,omitempty"`

	// --- branch 字段 ---
	Branch *Branch `json:"branch,omitempty"`

	// --- switch 字段 ---
	Switch *Switch `json:"switch,omitempty"`

	// --- parallel 字段 ---
	Parallel *Parallel `json:"parallel,omitempty"`

	// --- process 字段 ---
	Op   string            `json:"op,omitempty"`
	Args map[string]string `json:"args,omitempty"`

	// --- script 字段 ---
	Code string `json:"code,omitempty"`

	// --- export 字段 ---
	From   string `json:"from,omitempty"`   // 变量名，引用表格数据
	Format string `json:"format,omitempty"` // xlsx | csv | txt | json
	Path   string `json:"path,omitempty"`

	// --- 通用 ---
	Children []Step `json:"children,omitempty"` // loop 的子步骤
	ContinueOnFail bool `json:"continueOnFail,omitempty"`
}

// Extract 从响应中提取一个变量。
type Extract struct {
	Name    string `json:"name"`             // 变量名
	Source  string `json:"source,omitempty"` // body | header | cookie | status
	Kind    string `json:"kind,omitempty"`   // jsonpath | regex | whole | header | cookie
	Path    string `json:"path,omitempty"`   // jsonpath 表达式 / 正则 / header 名 / cookie 名
	Group   int    `json:"group,omitempty"`  // 正则捕获组序号
	Default string `json:"default,omitempty"`
}

// Assert 对响应做断言，实现"类似自动化测试"。
type Assert struct {
	Name   string `json:"name,omitempty"`
	Source string `json:"source,omitempty"` // status | body | header | variable | jsonpath
	Path   string `json:"path,omitempty"`   // jsonpath 表达式 / header 名 / 变量名
	Op     string `json:"op,omitempty"`     // eq | ne | contains | exists | gt | gte | lt | lte
	Value  string `json:"value,omitempty"`
}

// Loop 循环配置，支持三种模式：
//   - count:   固定次数，配合 PageVar/PageStep 实现页码递增
//   - foreach: 遍历一个数组变量（如提取出的列表）
//   - until:   一直循环直到 BreakWhen 条件成立（常用于翻页到空数据）
type Loop struct {
	Mode      string `json:"mode"` // count | foreach | until
	Count     int    `json:"count,omitempty"`
	ListVar   string `json:"listVar,omitempty"`   // foreach: 数组变量名
	ItemVar   string `json:"itemVar,omitempty"`   // foreach: 每次迭代写入的变量名
	IndexVar  string `json:"indexVar,omitempty"`  // foreach: 索引变量名
	BreakWhen string `json:"breakWhen,omitempty"` // until: 条件表达式，如 "{{len(rows)}} == 0"
	PageVar   string `json:"pageVar,omitempty"`   // 页码变量名，每轮自动 +PageStep
	PageStart int    `json:"pageStart,omitempty"`
	PageStep  int    `json:"pageStep,omitempty"`
	Max       int    `json:"max,omitempty"` // 最大轮数保护，默认 100
}

// Condition 单个条件：对变量值做比较（可视化配置用）。
// 求值时先取 Var 变量的值，再按 Op 与 Value 比较。
type Condition struct {
	Var   string `json:"var"`            // 变量名，如 amount（也支持点路径）
	Op    string `json:"op"`             // gt|gte|lt|lte|eq|ne|contains|exists|empty|true
	Value string `json:"value,omitempty"` // 比较值；exists/empty/true 时忽略
}

// Branch 条件分支配置。Conditions 按 Logic（and/or）组合；
// 也可直接用 When 写原始表达式（非空时优先于 Conditions）。
type Branch struct {
	Logic      string      `json:"logic,omitempty"` // and | or，默认 and
	Conditions []Condition `json:"conditions,omitempty"`
	When       string      `json:"when,omitempty"` // 高级：原始表达式，如 {{amount}} > 0
	Then       []Step      `json:"then,omitempty"` // 条件成立时执行的子步骤
	Else       []Step      `json:"else,omitempty"` // 条件不成立时执行的子步骤
}

// SwitchCase switch 的单个分支：条件成立时执行 Steps，然后跳出整个 switch。
type SwitchCase struct {
	Name       string      `json:"name,omitempty"`       // 分支名称，便于 UI 展示与日志
	Logic      string      `json:"logic,omitempty"`      // and | or，默认 and
	Conditions []Condition `json:"conditions,omitempty"` // 结构化条件
	When       string      `json:"when,omitempty"`       // 高级原始表达式，非空时优先于 Conditions
	Steps      []Step      `json:"steps,omitempty"`      // 命中后执行的子步骤
}

// Switch 多分支配置：自上而下依次求值 Cases，命中第一个即执行其 Steps 并结束；
// 全部未命中时执行 Default。
type Switch struct {
	Cases   []SwitchCase `json:"cases,omitempty"`
	Default []Step       `json:"default,omitempty"` // 兜底分支（可选）
}

// ParallelTask 并行汇合中的一个子任务。
// 子任务在自己的变量沙箱中执行，完成后按 Var/Value 产出一个结果供汇合。
type ParallelTask struct {
	Name  string `json:"name,omitempty"`  // 任务名，object 模式下作为 key
	Steps []Step `json:"steps,omitempty"` // 任务内执行的子步骤
	Var   string `json:"var,omitempty"`   // 从子任务变量中取该变量作为产物；为空则取整批变量快照
	Value string `json:"value,omitempty"` // 直接产出常量值（渲染后），优先于 Var
}

// Parallel 并行结果汇合配置：所有 Tasks 并发执行，全部完成后按 Mode 汇合到 Target 变量。
//   - array:  各任务产物顺序拼成数组
//   - object: 以任务名为 key 组成对象
//   - table:  各任务产物须为数组/可转数组，合并为一张表格行数组
//   - concat: 各任务产物转为字符串后拼接（可用 Separator 分隔）
type Parallel struct {
	Tasks     []ParallelTask `json:"tasks,omitempty"`
	Var       string         `json:"var,omitempty"`       // 汇合结果写入的变量名
	Mode      string         `json:"mode,omitempty"`      // array | object | table | concat
	Separator string         `json:"separator,omitempty"` // concat 模式的拼接分隔符
	FailFast  bool           `json:"failFast,omitempty"`  // 子任务失败时是否立即失败（默认忽略并继续）
}

// Response 封装一次 HTTP 响应的结果，供提取、断言、脚本、导出使用。
type Response struct {
	StatusCode int
	Headers    map[string][]string
	Cookies    []*HTTPCookie
	Body       []byte
	JSON        interface{} // 解析后的 JSON（若 body 是 JSON）
	Text        string
}

// HTTPCookie 简化 cookie，避免 model 依赖 net/http。
type HTTPCookie struct {
	Name  string
	Value string
}
