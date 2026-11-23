package engine

import (
	"encoding/json"
	"fmt"
	"strings"

	"requestflow/model"
)

// 本文件提供「配置试算」薄封装：复用引擎真实的提取/处理逻辑，
// 让 UI 能用最近一次调试响应即时预览结果，而不改动任何运行时变量。
// 所有函数只读输入、返回结构化结果，不写回 Context，便于每帧调用。

// PreviewExtractItem 单条提取规则的试算结果。
type PreviewExtractItem struct {
	Name    string      // 变量名
	Source  string      // 来源
	Kind    string      // 方式
	Path    string      // 表达式/选择器
	Value   interface{} // 提取到的值（可能是标量/数组/对象）
	Text    string      // 面向展示的紧凑文本
	Count   int         // 提取条数（标量=1，无结果=0）
	Default bool        // 是否用了默认值
	Err     string      // 错误提示（空=成功）
}

// PreviewResponseFromDebug 把调试结果转换为提取/试算所需的响应对象。
// 仅使用 HTTP 类字段；TCP 调试复用 Body/Text 通道。
func PreviewResponseFromDebug(dr *DebugResult) *model.Response {
	if dr == nil {
		return nil
	}
	h := make(map[string][]string, len(dr.Headers))
	for _, kv := range dr.Headers {
		h[kv.Key] = append(h[kv.Key], kv.Value)
	}
	cookies := make([]*model.HTTPCookie, 0, len(dr.Cookies))
	for _, kv := range dr.Cookies {
		cookies = append(cookies, &model.HTTPCookie{Name: kv.Key, Value: kv.Value})
	}
	var j interface{}
	trimmed := strings.TrimSpace(dr.Body)
	if dr.IsJSON || json.Valid([]byte(trimmed)) {
		_ = json.Unmarshal([]byte(trimmed), &j)
	}
	return &model.Response{
		StatusCode: dr.StatusCode,
		Headers:    h,
		Cookies:    cookies,
		Body:       []byte(dr.Body),
		Text:       dr.Body,
		JSON:       j,
	}
}

// PreviewExtracts 用变量上下文和响应文本，对一组提取规则逐条试算。
// 复用引擎 extractOne 的真实逻辑，保证「预览即运行」。
func PreviewExtracts(extracts []model.Extract, vars map[string]string, resp *model.Response) []PreviewExtractItem {
	ctx := NewContext(vars)
	out := make([]PreviewExtractItem, 0, len(extracts))
	for _, ex := range extracts {
		item := PreviewExtractItem{Name: ex.Name, Source: ex.Source, Kind: ex.Kind, Path: ex.Path}
		if strings.TrimSpace(ex.Name) == "" {
			item.Name = "(未命名)"
		}
		if resp == nil {
			item.Err = "暂无调试响应，请先在【接口调试】发送一次请求"
			out = append(out, item)
			continue
		}
		v, err := extractOne(ctx, resp, ex)
		if err != nil {
			item.Err = err.Error()
			out = append(out, item)
			continue
		}
		if v == nil || toStr(v) == "" {
			if ex.Default != "" {
				v = ex.Default
				item.Default = true
			}
		}
		item.Value = v
		item.Count = extractedCount(v)
		item.Text = previewCompactText(v)
		if ex.ExpectMin > 0 && item.Count < ex.ExpectMin {
			item.Err = fmt.Sprintf("仅 %d 条，低于 expectMin=%d（可能是反爬页或抓空页）", item.Count, ex.ExpectMin)
		}
		out = append(out, item)
	}
	return out
}

// PreviewProcessItem 单个处理操作的试算结果。
type PreviewProcessItem struct {
	Op       string      // 处理操作
	OutName  string      // 输出变量名
	Before   interface{} // 执行前来源变量值（用于对比）
	After    interface{} // 执行后输出变量值
	BeforeT  string
	AfterT   string
	Delta    string // 变化摘要，如 "5 条 → 3 条"
	Err      string
	HintOnly bool // 仅提示（如来源变量缺失），非执行错误
}

// PreviewProcess 在隔离的 Context 中试算一个 process 步骤，返回来源/输出变量的前后对比。
// 输入变量由 vars 提供（场景变量 + 提取示例），不写回任何真实上下文。
func PreviewProcess(step model.Step, vars map[string]string) PreviewProcessItem {
	op := strings.ToLower(strings.TrimSpace(step.Op))
	item := PreviewProcessItem{Op: step.Op}
	ctx := NewContext(vars)
	get := func(k string) string { return ctx.Render(step.Args[k]) }

	// 推断来源变量与输出变量（与 processor*.go 的 key 约定一致）。
	src := firstNonEmpty(get("from"), get("values"), get("list"))
	out := get("to")
	if out == "" {
		out = defaultOutName(op)
	}
	item.OutName = out
	if src != "" {
		item.Before = ctx.resolveArg(step.Args["from"])
		item.BeforeT = previewCompactText(item.Before)
	}

	if err := runProcess(ctx, step); err != nil {
		item.Err = err.Error()
		return item
	}
	item.After = ctx.Get(out)
	if item.After == nil {
		// set 类操作输出来自 name 或 value。
		if n := get("name"); n != "" {
			item.OutName = n
			item.After = ctx.Get(n)
		}
	}
	item.AfterT = previewCompactText(item.After)
	item.Delta = previewDelta(item.BeforeT, item.AfterT, op)
	return item
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func defaultOutName(op string) string {
	switch op {
	case "set":
		return "value"
	case "append":
		return "rows"
	case "json_to_table":
		return "table"
	case "filter":
		return "filtered"
	case "sort":
		return "sorted"
	case "pick":
		return "picked"
	case "flatten":
		return "allRows"
	case "dedup":
		return "deduped"
	case "zip":
		return "zipped"
	case "row":
		return "final"
	case "limit", "take", "slice":
		return "limited"
	case "resolve_url", "urlencode":
		return "encoded"
	case "urldecode", "base64decode":
		return "decoded"
	case "base64encode":
		return "encoded"
	case "format_time":
		return "time"
	case "join":
		return "joined"
	default:
		return "result"
	}
}

// PreviewValueString 把试算值转成可回填到变量上下文的字符串。
// 标量直接返回文本，数组/对象序列化为 JSON，供下游 process 步骤 resolveArg 解析。
func PreviewValueString(v interface{}) string {
	return toStr(v)
}

// previewCompactText 把任意值转成单行紧凑文本，数组/对象展示条数与前若干元素。
func previewCompactText(v interface{}) string {
	if v == nil {
		return "∅（无结果）"
	}
	switch x := v.(type) {
	case []interface{}:
		if len(x) == 0 {
			return "[] 0 条"
		}
		parts := make([]string, 0, 3)
		for i, e := range x {
			if i >= 3 {
				break
			}
			parts = append(parts, toStr(e))
		}
		s := strings.Join(parts, ", ")
		if len(x) > 3 {
			s += " …"
		}
		return fmt.Sprintf("[%d 条] %s", len(x), s)
	case map[string]interface{}:
		b, _ := json.Marshal(x)
		s := string(b)
		if len(s) > 120 {
			s = s[:120] + "…"
		}
		return s
	default:
		s := strings.ReplaceAll(toStr(v), "\n", " ")
		if len([]rune(s)) > 160 {
			s = string([]rune(s)[:160]) + "…"
		}
		return s
	}
}

func previewDelta(before, after, op string) string {
	if before == "" && after != "" {
		return "→ " + after
	}
	if before == after {
		return "值未变化（检查来源变量与参数）"
	}
	return before + " → " + after
}
