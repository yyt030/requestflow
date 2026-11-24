package ui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"requestflow/model"
)

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

// mapToColonString 与 mapToString 相同，但输出 "key: value"（HTTP 头/查询参数的标准写法，
// 也是 Chrome DevTools「Copy request headers」的格式）。用于步骤编排与调试面板，
// 让显示格式与界面提示、外部工具一致——避免复制出去后因 " = " 格式导致发送失败。
// stringToMap 对 ":" 与 "=" 都能解析，故不影响发送时的还原。
func mapToColonString(m map[string]string) string {
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
		b.WriteString(": ")
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
