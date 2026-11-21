// Package model 定义场景配置的数据模型。
// 一个场景 = 一个 JSON 文件，内含有序步骤列表与全局设置。
package model

// Scenario 一个可独立运行的自动化场景，对应磁盘上的一个 JSON 文件。
type Scenario struct {
	Name      string            `json:"name"`
	BaseURL   string            `json:"baseUrl,omitempty"`
	Variables map[string]string `json:"variables,omitempty"`
	Steps     []Step            `json:"steps"`

	// 环境变量分层：Environments 是「环境名 → 变量集」的映射，ActiveEnv 指定当前启用
	// 的环境。运行时变量优先级：环境变量 > 场景全局变量（同名时环境变量覆盖）。
	// 旧场景没有这两个字段时按空处理，行为与之前完全一致。
	Environments map[string]map[string]string `json:"environments,omitempty"`
	ActiveEnv    string                       `json:"activeEnv,omitempty"`
}

// EnvNames 返回按名称排序的环境列表，便于 UI 稳定展示。
func (s *Scenario) EnvNames() []string {
	names := make([]string, 0, len(s.Environments))
	for k := range s.Environments {
		names = append(names, k)
	}
	return names
}

// EnvVars 返回指定环境的变量集（只读语义，返回副本前的原始 map 由调用方修改场景）。
func (s *Scenario) EnvVars(name string) map[string]string {
	if s.Environments == nil {
		return nil
	}
	return s.Environments[name]
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
	StepTCP      StepType = "tcp"      // TCP Socket：连接→发送→读取响应，可提取变量、断言
	StepLoop     StepType = "loop"     // 循环执行子步骤，用于翻页/遍历
	StepBranch   StepType = "branch"   // 条件分支：when 成立执行 then，否则执行 else
	StepSwitch   StepType = "switch"   // 多分支：自上而下匹配第一个 case，否则 default
	StepParallel StepType = "parallel" // 扇出：多个子任务并行执行
	StepMerge    StepType = "merge"    // 扇入：等待并行任务全部成功后按条件门控放行
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

	// Body 类型：raw（默认，兼容旧场景）| form-data | urlencoded | binary | graphql | none
	BodyType         string      `json:"bodyType,omitempty"`
	FormData         []FormField `json:"formData,omitempty"`
	GraphQLQuery     string      `json:"graphqlQuery,omitempty"`
	GraphQLVariables string      `json:"graphqlVariables,omitempty"`

	// Auth: bearer token 从变量读取（如 {{token}}）；cookie 由全局 CookieJar 自动管理
	AuthType  string `json:"authType,omitempty"`  // none | bearer | basic | apikey | oauth2
	AuthToken string `json:"authToken,omitempty"` // bearer: "{{token}}"
	AuthUser  string `json:"authUser,omitempty"`
	AuthPass  string `json:"authPass,omitempty"`
	// apikey 认证
	AuthKeyName  string `json:"authKeyName,omitempty"`  // header/query 参数名
	AuthKeyValue string `json:"authKeyValue,omitempty"` // 参数值（支持 {{变量}}）
	AuthKeyIn    string `json:"authKeyIn,omitempty"`    // header（默认）| query
	// oauth2 认证（client_credentials / password）
	AuthTokenURL     string `json:"authTokenUrl,omitempty"`
	AuthClientID     string `json:"authClientId,omitempty"`
	AuthClientSecret string `json:"authClientSecret,omitempty"`
	AuthScope        string `json:"authScope,omitempty"`
	AuthGrantType    string `json:"authGrantType,omitempty"` // client_credentials（默认）| password

	Extract []Extract `json:"extract,omitempty"`
	Assert  []Assert  `json:"assert,omitempty"`

	// --- tcp 字段 ---
	TCPHost     string `json:"tcpHost,omitempty"`     // 目标主机，如 127.0.0.1 或 {{host}}
	TCPPort     string `json:"tcpPort,omitempty"`     // 端口，如 8080 或 {{port}}
	TCPSend     string `json:"tcpSend,omitempty"`     // 发送的数据（文本），支持 {{变量}}；可用 \n \r\n \xNN 转义
	TCPSendMode string `json:"tcpSendMode,omitempty"` // line（追加换行，默认）| raw（原样发送）| hex（按十六进制解码）
	TCPReadMode string `json:"tcpReadMode,omitempty"` // line（读一行）| all（读到关闭/超时）| fixed（固定字节）| until（读到标记）
	TCPReadNum  string `json:"tcpReadNum,omitempty"`  // fixed 模式的字节数 / until 模式的最大字节数，支持 {{变量}}
	TCPUntil    string `json:"tcpUntil,omitempty"`    // until 模式的结束标记，如 \n 或 >
	TCPTimeout  string `json:"tcpTimeout,omitempty"`  // 超时毫秒，默认 5000，支持 {{变量}}
	TCPTLS      bool   `json:"tcpTls,omitempty"`      // 是否使用 TLS（tcp over TLS）

	// 连接复用与帧协议
	TCPConnMode string `json:"tcpConnMode,omitempty"` // short（默认，每步独立连接）| long（按连接名跨步骤复用）
	TCPConnKey  string `json:"tcpConnKey,omitempty"`  // 长连接共享键，同名步骤复用同一 socket；空为 "_default"
	TCPClose    bool   `json:"tcpClose,omitempty"`    // 关闭 TCPConnKey 指定的长连接（不再发送数据）

	// 保活：TCPKeepAlive 为 TCP 保活探测周期（秒）；0/空=关闭（默认）。
	// 底层使用 net.TCPConn.SetKeepAlive + SetKeepAlivePeriod；TLS 连接作用于其底层 TCP socket。
	TCPKeepAlive string `json:"tcpKeepAlive,omitempty"`

	// 帧头类型（send/read 的 frame 模式使用）：
	//   - ascii：N 位十进制字符串长度头，位数自定义（如 3/5/8），不足左补零，如 8 字节报文→"00000008"（兼容旧场景）
	//   - bin1 ：1 字节二进制长度头，最大 255 字节报文
	//   - bin2 ：2 字节二进制长度头，最大 65535（64K-1）字节；字节序由 TCPFrameEndian 决定
	// 空值时按 TCPFrameHead>0 推断为 ascii，保证旧场景无需改动。
	TCPFrameType   string `json:"tcpFrameType,omitempty"`   // ascii | bin1 | bin2 | 空（不启用）
	TCPFrameHead   string `json:"tcpFrameHead,omitempty"`   // ascii 帧头位数，如 3/5/8；空按 8 处理；bin1/bin2 忽略
	TCPFrameEndian string `json:"tcpFrameEndian,omitempty"` // bin2 字节序：big（默认，网络序）| little

	// --- loop 字段 ---
	Loop *Loop `json:"loop,omitempty"`

	// --- branch 字段 ---
	Branch *Branch `json:"branch,omitempty"`

	// --- switch 字段 ---
	Switch *Switch `json:"switch,omitempty"`

	// --- parallel 字段 ---
	Parallel *Parallel `json:"parallel,omitempty"`

	// --- merge 字段 ---
	Merge *Merge `json:"merge,omitempty"`

	// --- process 字段 ---
	Op   string            `json:"op,omitempty"`
	Args map[string]string `json:"args,omitempty"`

	// --- script 字段 ---
	ScriptLang string `json:"scriptLang,omitempty"` // js（默认）| python；空值按 js 处理，兼容旧场景
	Code       string `json:"code,omitempty"`

	// 请求级脚本（Postman 风格的 Pre-request Script / Tests）：仅在 request 步骤使用。
	// PreScript 在请求发送前执行（可改写变量影响本次请求）；PostScript 在收到响应、
	// 提取与断言之后执行（可用 last 访问响应、vars 读写变量）。语言同 ScriptLang。
	PreScript  string `json:"preScript,omitempty"`
	PostScript string `json:"postScript,omitempty"`

	// --- export 字段 ---
	From   string `json:"from,omitempty"`   // 变量名，引用表格数据
	Format string `json:"format,omitempty"` // xlsx | csv | txt | json
	Path   string `json:"path,omitempty"`

	// --- 通用 ---
	Children       []Step `json:"children,omitempty"` // loop 的子步骤
	ContinueOnFail bool   `json:"continueOnFail,omitempty"`
}

// FormField form-data / x-www-form-urlencoded 的单个字段。
// Type 为 "text"（默认）时 Value 为字段文本；为 "file" 时 Value 为要上传的文件路径。
type FormField struct {
	Key   string `json:"key"`
	Type  string `json:"type,omitempty"`  // text | file
	Value string `json:"value,omitempty"` // 文本值或文件路径
}

// Extract 从响应中提取一个变量。
type Extract struct {
	Name    string `json:"name"`             // 变量名
	Source  string `json:"source,omitempty"` // body | header | cookie | status
	Kind    string `json:"kind,omitempty"`   // jsonpath | regex | whole | header | cookie | css
	Path    string `json:"path,omitempty"`   // jsonpath 表达式 / 正则 / header 名 / cookie 名 / CSS 选择器
	Group   int    `json:"group,omitempty"`  // 正则捕获组序号
	Default string `json:"default,omitempty"`

	// CSS 提取（kind=css）：多元素提取为数组。
	CssField string `json:"cssField,omitempty"` // text（默认）| html（内部 HTML）| attr（属性值）
	CssAttr  string `json:"cssAttr,omitempty"`  // cssField=attr 时的属性名，如 href / content

	// CSS 结构化行提取（kind=cssRows）：按行选择器逐行提取多个子字段，直接产出对象数组，
	// 从机制上避免「多个独立 css 选择器分别提数组再 zip」的错位风险。
	// 用法：Path=行选择器（如 div.item），CssFields 列出该行内每个字段的选择器与取值方式。
	CssFields []CSSField `json:"cssFields,omitempty"`

	// ExpectMin：提取结果最少条数校验。>0 时若提取到的元素数小于该值即报错（翻页抓空/反爬页兜底）。
	// 对 cssRows 是行数，对 css/jsonpath 是数组长度；标量按 1 计。0 表示不校验。
	ExpectMin int `json:"expectMin,omitempty"`

	// ResolveURL：CSS attr 提取到的 URL 值按 baseUrl 绝对化，并解码常见跳转链接
	// （百度 link?url=、Bing ck/a?u=a1...）。仅对 css/cssRows 的 attr 字段生效。
	ResolveURL bool `json:"resolveUrl,omitempty"`
}

// CSSField 定义 cssRows 行提取中的一个子字段。
// 每个字段在「行元素」内部再按 Path 选择（Path 为空表示取行元素自身）。
type CSSField struct {
	Name     string `json:"name"`               // 输出对象中的字段名
	Path     string `json:"path,omitempty"`     // 行内 CSS 选择器；空表示行元素自身
	CssField string `json:"cssField,omitempty"` // text（默认）| html | attr
	CssAttr  string `json:"cssAttr,omitempty"`  // cssField=attr 时的属性名
	Resolve  bool   `json:"resolve,omitempty"`  // 该字段值是否做 URL 绝对化与跳转解码
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

	// 循环结束后的累积去重（P1 易用性）：对 DedupVar 指定的数组变量按 DedupField 去重。
	//   - DedupVar   : 要被去重的累积数组变量名（如 allRows）；为空表示不启用去重。
	//   - DedupField : 可选，对象数组按此字段值去重（如 link）；为空按整个元素去重。
	// 去重后写回同名变量，并把被去掉的重复数量 emit 到日志提示；
	// DedupStat 非空时另把重复数写入该变量，供后续步骤/断言引用。
	DedupVar   string `json:"dedupVar,omitempty"`
	DedupField string `json:"dedupField,omitempty"`
	DedupStat  string `json:"dedupStat,omitempty"`

	// 健壮性（P2）：DelayMs 每轮子步骤执行前的请求间隔（毫秒），用于限速防反爬。
	// MinRowsVar 指定一个数组变量名，本轮结束后若该变量元素数 < MinRows 则视为空页，
	// 触发告警；RetryOnEmpty>0 时空页自动重试该次数（重新执行本轮子步骤）。
	DelayMs      int    `json:"delayMs,omitempty"`
	MinRowsVar   string `json:"minRowsVar,omitempty"`
	MinRows      int    `json:"minRows,omitempty"`
	RetryOnEmpty int    `json:"retryOnEmpty,omitempty"`
}

// Condition 单个条件：对变量值做比较（可视化配置用）。
// 求值时先取 Var 变量的值，再按 Op 与 Value 比较。
type Condition struct {
	Var   string `json:"var"`             // 变量名，如 amount（也支持点路径）
	Op    string `json:"op"`              // gt|gte|lt|lte|eq|ne|contains|exists|empty|true
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
	StatusVar string         `json:"statusVar,omitempty"` // 任务状态登记表写入的变量名，默认 task
}

// Merge 扇入门控配置：等待上游 parallel 的所有任务完成后，用高级表达式检查各任务状态/产物。
// 表达式可引用：task.<任务名>.status（success|fail）、task.<任务名>.output、
// task.<任务名>.error、task.<任务名>.duration，也支持 len(task) 等函数与 && / || 组合。
// 条件不成立即中止整个场景（类似断言失败）。
type Merge struct {
	When string `json:"when,omitempty"` // 门控表达式，如 task.A.status == "success" && task.B.status == "success"
}

// Response 封装一次 HTTP 响应的结果，供提取、断言、脚本、导出使用。
type Response struct {
	StatusCode int
	Headers    map[string][]string
	Cookies    []*HTTPCookie
	Body       []byte
	JSON       interface{} // 解析后的 JSON（若 body 是 JSON）
	Text       string
}

// HTTPCookie 简化 cookie，避免 model 依赖 net/http。
type HTTPCookie struct {
	Name  string
	Value string
}
