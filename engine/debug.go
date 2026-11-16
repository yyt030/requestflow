package engine

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"requestflow/model"
)

// DebugRequest 一次接口调试请求的配置（变量可写成 {{name}}）。
type DebugRequest struct {
	Method      string
	URL         string
	Headers     map[string]string
	Query       map[string]string
	Body        string
	AuthType    string // none | bearer | basic
	AuthToken   string
	AuthUser    string
	AuthPass    string
	TimeoutSec  int  // <=0 时默认 30 秒
	InsecureTLS bool // 跳过 TLS 证书校验
}

// KV 用于有序展示 header/cookie。
type KV struct {
	Key   string
	Value string
}

// DebugResult 调试请求的结果，含状态码、耗时、响应头、Cookie、格式化响应体与反向 cURL。
type DebugResult struct {
	OK         bool
	Err        string
	Method     string
	URL        string // 渲染后的实际请求 URL（含 query）
	StatusCode int
	StatusText string
	DurationMS int64
	Size       int
	Headers    []KV
	Cookies    []KV
	Body       string // 原始响应体
	Formatted  string // JSON 美化后的响应体（非 JSON 则原文）
	IsJSON     bool
	Curl       string
}

// Debug 执行一次调试请求。vars 用于 {{变量}} 渲染，通常传场景全局变量（含 baseUrl）。
func Debug(req DebugRequest, vars map[string]string) *DebugResult {
	ctx := NewContext(vars)

	method := strings.ToUpper(strings.TrimSpace(ctx.Render(req.Method)))
	if method == "" {
		method = http.MethodGet
	}
	rawURL := ctx.Render(req.URL)
	headers := ctx.RenderMap(req.Headers)
	query := ctx.RenderMap(req.Query)
	body := ctx.Render(req.Body)
	auth := &model.Step{
		AuthType:  ctx.Render(req.AuthType),
		AuthToken: ctx.Render(req.AuthToken),
		AuthUser:  ctx.Render(req.AuthUser),
		AuthPass:  ctx.Render(req.AuthPass),
	}

	timeout := time.Duration(req.TimeoutSec) * time.Second
	if req.TimeoutSec <= 0 {
		timeout = 30 * time.Second
	}
	client := NewHTTPClient(timeout, req.InsecureTLS)

	start := time.Now()
	resp, err := client.Do(method, rawURL, headers, query, body, auth)
	elapsed := time.Since(start)

	res := &DebugResult{
		Method:     method,
		URL:        mergeQuery(rawURL, query),
		DurationMS: elapsed.Milliseconds(),
	}
	if err != nil {
		res.Err = err.Error()
		return res
	}

	res.OK = true
	res.StatusCode = resp.StatusCode
	res.StatusText = http.StatusText(resp.StatusCode)
	res.Size = len(resp.Body)
	res.Body = resp.Text

	keys := make([]string, 0, len(resp.Headers))
	for k := range resp.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range resp.Headers[k] {
			res.Headers = append(res.Headers, KV{Key: k, Value: v})
		}
	}
	for _, c := range resp.Cookies {
		res.Cookies = append(res.Cookies, KV{Key: c.Name, Value: c.Value})
	}

	trimmed := strings.TrimSpace(resp.Text)
	var buf bytes.Buffer
	if json.Valid([]byte(trimmed)) && json.Indent(&buf, []byte(trimmed), "", "  ") == nil {
		res.Formatted = buf.String()
		res.IsJSON = true
	} else {
		res.Formatted = resp.Text
	}

	res.Curl = BuildCurlPreview(method, rawURL, headers, query, body, auth)
	return res
}

// DebugStep 从场景 request 步骤构造调试请求并复用 Debug，用于"试发送此请求"。
func DebugStep(step *model.Step, vars map[string]string) *DebugResult {
	return Debug(DebugRequest{
		Method:      step.Method,
		URL:         step.URL,
		Headers:     step.Headers,
		Query:       step.Query,
		Body:        step.Body,
		AuthType:    step.AuthType,
		AuthToken:   step.AuthToken,
		AuthUser:    step.AuthUser,
		AuthPass:    step.AuthPass,
	}, vars)
}

// mergeQuery 把 query 合并进 URL，用于展示实际请求地址。
func mergeQuery(rawURL string, query map[string]string) string {
	if len(query) == 0 {
		return rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := u.Query()
	for k, v := range query {
		q.Add(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// shellQuote 用单引号包裹 shell 参数并转义内部的单引号。
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// BuildCurlPreview 把请求配置反向生成 cURL 命令，方便复制复现（请求预览）。
func BuildCurlPreview(method, rawURL string, headers, query map[string]string, body string, auth *model.Step) string {
	full := mergeQuery(rawURL, query)
	parts := []string{"curl " + shellQuote(full)}

	m := strings.ToUpper(strings.TrimSpace(method))
	if m != "" && m != http.MethodGet {
		parts = append(parts, "-X "+shellQuote(m))
	}

	all := make(map[string]string, len(headers)+2)
	for k, v := range headers {
		all[k] = v
	}
	if auth != nil {
		switch strings.ToLower(auth.AuthType) {
		case "bearer":
			all["Authorization"] = "Bearer " + auth.AuthToken
		case "basic":
			token := base64.StdEncoding.EncodeToString([]byte(auth.AuthUser + ":" + auth.AuthPass))
			all["Authorization"] = "Basic " + token
		}
	}
	hkeys := make([]string, 0, len(all))
	for k := range all {
		hkeys = append(hkeys, k)
	}
	sort.Strings(hkeys)
	for _, k := range hkeys {
		parts = append(parts, "-H "+shellQuote(k+": "+all[k]))
	}
	if body != "" {
		parts = append(parts, "--data-raw "+shellQuote(body))
	}
	return strings.Join(parts, " \\\n  ")
}
