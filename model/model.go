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

// EnvNames 返回按名称排序的环境列表，便于 UI 稳定展示。
func (s *Scenario) EnvNames() []string {
	names := make([]string, 0, len(s.Environments))
	for k := range s.Environments {
		names = append(names, k)
	}
	return names
}

// EnvVars 返回指定环境的变量集（只读语义，返回副本前的原始 map 由调用方修改场景）。

// EnvVars 返回指定环境的变量集（只读语义，返回副本前的原始 map 由调用方修改场景）。
func (s *Scenario) EnvVars(name string) map[string]string {
	if s.Environments == nil {
		return nil
	}
	return s.Environments[name]
}

// NewScenario 返回一个带默认值的空场景。

// NewScenario 返回一个带默认值的空场景。
func NewScenario(name string) *Scenario {
	return &Scenario{
		Name:      name,
		Variables: map[string]string{},
		Steps:     []Step{},
	}
}

// StepType 步骤类型。

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

	// --- request 字段（步骤内联，是请求配置的唯一真相源）---
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
