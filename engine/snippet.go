package engine

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"requestflow/model"
)

// SnippetLang 代码生成支持的目标语言。
var SnippetLangs = []string{"python", "go", "javascript", "curl"}

// BuildSnippet 把一次请求配置反向生成为目标语言的可运行代码片段。
// method/rawURL/headers/query/body 为已渲染后的值（不再含 {{变量}}），
// step 提供 Body 类型与认证信息，与 cURL 预览保持同一套语义。
func BuildSnippet(lang, method, rawURL string, headers, query map[string]string, body string, step *model.Step) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = "GET"
	}
	full := mergeQuery(rawURL, query)
	h := collectHeaders(headers, step)
	switch strings.ToLower(lang) {
	case "python":
		return snippetPython(method, full, h, body, step)
	case "go":
		return snippetGo(method, full, h, body, step)
	case "javascript":
		return snippetJS(method, full, h, body, step)
	default:
		return BuildCurlPreview(method, rawURL, headers, query, body, step)
	}
}

// collectHeaders 合并用户请求头与认证头，返回排序后的键值（稳定输出）。
func collectHeaders(headers map[string]string, step *model.Step) map[string]string {
	out := make(map[string]string, len(headers)+2)
	for k, v := range headers {
		out[k] = v
	}
	if step != nil {
		switch strings.ToLower(step.AuthType) {
		case "bearer":
			out["Authorization"] = "Bearer " + step.AuthToken
		case "basic":
			out["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(step.AuthUser+":"+step.AuthPass))
		case "apikey":
			if !strings.EqualFold(step.AuthKeyIn, "query") && step.AuthKeyName != "" {
				out[step.AuthKeyName] = step.AuthKeyValue
			}
		}
		if strings.EqualFold(strings.TrimSpace(step.BodyType), "graphql") {
			out["Content-Type"] = "application/json"
		}
	}
	return out
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// effectiveBody 返回最终请求体文本（form-data/graphql 给出可读占位）。
func effectiveBody(body string, step *model.Step) string {
	if step == nil {
		return body
	}
	switch strings.ToLower(strings.TrimSpace(step.BodyType)) {
	case "graphql":
		vars := step.GraphQLVariables
		if strings.TrimSpace(vars) == "" {
			vars = "{}"
		}
		q, _ := jsonMarshal(step.GraphQLQuery)
		return fmt.Sprintf(`{"query":%s,"variables":%s}`, string(q), vars)
	case "form-data", "urlencoded":
		parts := make([]string, 0, len(step.FormData))
		for _, f := range step.FormData {
			if f.Key == "" {
				continue
			}
			parts = append(parts, fmt.Sprintf("%s=%s", f.Key, f.Value))
		}
		return strings.Join(parts, "&")
	default:
		return body
	}
}

func snippetPython(method, full string, headers map[string]string, body string, step *model.Step) string {
	var b strings.Builder
	b.WriteString("import requests\n\n")
	ct := contentTypeOf(headers)
	body = effectiveBody(body, step)
	switch strings.ToLower(contentTypeOf(headers)) {
	case "application/json":
		b.WriteString("payload = " + toPythonLiteral(body) + "\n")
		b.WriteString(fmt.Sprintf("r = requests.%s(%s, json=payload, headers=%s)\n",
			strings.ToLower(method), pyQuote(full), pyHeaders(headers)))
	default:
		if body != "" {
			b.WriteString("payload = " + pyQuote(body) + "\n")
			b.WriteString(fmt.Sprintf("r = requests.%s(%s, data=payload, headers=%s)\n",
				strings.ToLower(method), pyQuote(full), pyHeaders(headers)))
		} else {
			b.WriteString(fmt.Sprintf("r = requests.%s(%s, headers=%s)\n",
				strings.ToLower(method), pyQuote(full), pyHeaders(headers)))
		}
	}
	_ = ct
	b.WriteString("print(r.status_code)\nprint(r.text)\n")
	return b.String()
}

func pyHeaders(headers map[string]string) string {
	if len(headers) == 0 {
		return "{}"
	}
	parts := make([]string, 0, len(headers))
	for _, k := range sortedKeys(headers) {
		parts = append(parts, pyQuote(k)+": "+pyQuote(headers[k]))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func pyQuote(s string) string {
	// 用 repr 风格：优先单引号，内含单引号时改双引号，转义反斜杠与换行。
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		return `"` + s + `"`
	}
	return "'" + strings.ReplaceAll(s, "'", `\'`) + "'"
}

func toPythonLiteral(s string) string {
	s = strings.TrimSpace(s)
	if jsonValid(s) {
		// JSON 对象/数组基本兼容 Python 字面量（true/false/null 需转换）
		s = strings.ReplaceAll(s, "true", "True")
		s = strings.ReplaceAll(s, "false", "False")
		s = strings.ReplaceAll(s, "null", "None")
		return s
	}
	return pyQuote(s)
}

func snippetGo(method, full string, headers map[string]string, body string, step *model.Step) string {
	var b strings.Builder
	b.WriteString("package main\n\nimport (\n\t\"fmt\"\n\t\"io\"\n\t\"net/http\"\n\t\"strings\"\n)\n\n")
	b.WriteString("func main() {\n")
	body = effectiveBody(body, step)
	if body != "" {
		b.WriteString("\tpayload := strings.NewReader(" + goQuote(body) + ")\n")
		b.WriteString(fmt.Sprintf("\treq, err := http.NewRequest(%s, %s, payload)\n", goQuote(method), goQuote(full)))
	} else {
		b.WriteString(fmt.Sprintf("\treq, err := http.NewRequest(%s, %s, nil)\n", goQuote(method), goQuote(full)))
	}
	b.WriteString("\tif err != nil { panic(err) }\n")
	for _, k := range sortedKeys(headers) {
		b.WriteString(fmt.Sprintf("\treq.Header.Set(%s, %s)\n", goQuote(k), goQuote(headers[k])))
	}
	b.WriteString("\n\tresp, err := http.DefaultClient.Do(req)\n\tif err != nil { panic(err) }\n\tdefer resp.Body.Close()\n")
	b.WriteString("\tdata, _ := io.ReadAll(resp.Body)\n\tfmt.Println(resp.StatusCode)\n\tfmt.Println(string(data))\n}\n")
	return b.String()
}

func goQuote(s string) string { return strconv.Quote(s) }

func snippetJS(method, full string, headers map[string]string, body string, step *model.Step) string {
	var b strings.Builder
	b.WriteString("const headers = " + jsHeaders(headers) + ";\n")
	body = effectiveBody(body, step)
	opts := ""
	if body != "" {
		opts = ",\n  body: " + jsQuote(body)
	}
	b.WriteString(fmt.Sprintf("fetch(%s, {\n  method: %s,\n  headers%s\n})%s\n",
		jsQuote(full), jsQuote(method), opts, strings.Join([]string{
			"\n  .then(r => r.text().then(t => console.log(r.status, t)))",
			"\n  .catch(e => console.error(e));",
		}, "")))
	return b.String()
}

func jsHeaders(headers map[string]string) string {
	if len(headers) == 0 {
		return "{}"
	}
	parts := make([]string, 0, len(headers))
	for _, k := range sortedKeys(headers) {
		parts = append(parts, jsQuote(k)+": "+jsQuote(headers[k]))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func jsQuote(s string) string {
	b, _ := jsonMarshal(s)
	return string(b)
}

func contentTypeOf(headers map[string]string) string {
	for k, v := range headers {
		if strings.EqualFold(k, "Content-Type") {
			return strings.ToLower(strings.TrimSpace(strings.Split(v, ";")[0]))
		}
	}
	return ""
}

// 以下为无第三方依赖的轻量辅助，避免 snippet 包引入额外依赖。
func jsonValid(s string) bool {
	s = strings.TrimSpace(s)
	return (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]"))
}

func jsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
