package engine

import (
	"bytes"
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
	Method           string
	URL              string
	Headers          map[string]string
	Query            map[string]string
	Body             string
	BodyType         string // raw | form-data | urlencoded | binary | graphql | none
	FormData         []model.FormField
	GraphQLQuery     string
	GraphQLVariables string
	AuthType         string // none | bearer | basic | apikey | oauth2
	AuthToken        string
	AuthUser         string
	AuthPass         string
	AuthKeyName      string
	AuthKeyValue     string
	AuthKeyIn        string
	AuthTokenURL     string
	AuthClientID     string
	AuthClientSecret string
	AuthScope        string
	AuthGrantType    string
	TimeoutSec       int  // <=0 时默认 30 秒
	InsecureTLS      bool // 跳过 TLS 证书校验
}

// KV 用于有序展示 header/cookie。

// KV 用于有序展示 header/cookie。
type KV struct {
	Key   string
	Value string
}

// DebugResult 调试请求的结果，含状态码、耗时、响应头、Cookie、格式化响应体与反向 cURL。

// DebugResult 调试请求的结果，含状态码、耗时、响应头、Cookie、格式化响应体与反向 cURL。
type DebugResult struct {
	OK         bool
	Err        string
	Protocol   string // http（默认，空串也按 http）| tcp
	Method     string
	URL        string // 渲染后的实际请求 URL（含 query）；TCP 时为 host:port
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
	Snippets   map[string]string // 代码生成：python / go / javascript / curl

	// TCP 单步调试专用字段
	Sent        []byte // 实际发送的字节（用于十六进制展示）
	ReceivedRaw []byte // 原始接收字节（二进制响应时避免 string() 破坏）
	IsBinary    bool   // 响应含不可打印字节时以十六进制展示
}

// Debug 执行一次调试请求。vars 用于 {{变量}} 渲染，通常传场景全局变量（含 baseUrl）。

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
	step := &model.Step{
		BodyType:         ctx.Render(req.BodyType),
		Body:             body,
		GraphQLQuery:     ctx.Render(req.GraphQLQuery),
		GraphQLVariables: ctx.Render(req.GraphQLVariables),
		AuthType:         ctx.Render(req.AuthType),
		AuthToken:        ctx.Render(req.AuthToken),
		AuthUser:         ctx.Render(req.AuthUser),
		AuthPass:         ctx.Render(req.AuthPass),
		AuthKeyName:      ctx.Render(req.AuthKeyName),
		AuthKeyValue:     ctx.Render(req.AuthKeyValue),
		AuthKeyIn:        ctx.Render(req.AuthKeyIn),
		AuthTokenURL:     ctx.Render(req.AuthTokenURL),
		AuthClientID:     ctx.Render(req.AuthClientID),
		AuthClientSecret: ctx.Render(req.AuthClientSecret),
		AuthScope:        ctx.Render(req.AuthScope),
		AuthGrantType:    ctx.Render(req.AuthGrantType),
	}
	for _, f := range req.FormData {
		step.FormData = append(step.FormData, model.FormField{Key: ctx.Render(f.Key), Type: f.Type, Value: ctx.Render(f.Value)})
	}

	timeout := time.Duration(req.TimeoutSec) * time.Second
	if req.TimeoutSec <= 0 {
		timeout = 30 * time.Second
	}
	client := NewHTTPClient(timeout, req.InsecureTLS)

	start := time.Now()
	resp, err := client.DoRequest(step, method, rawURL, headers, query)
	elapsed := time.Since(start)

	res := &DebugResult{
		Method:     method,
		URL:        mergeQuery(rawURL, query),
		DurationMS: elapsed.Milliseconds(),
	}
	// 代码生成基于已渲染的请求配置，与请求是否成功无关。
	res.Snippets = map[string]string{}
	for _, lang := range SnippetLangs {
		res.Snippets[lang] = BuildSnippet(lang, method, rawURL, headers, query, body, step)
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

	res.Curl = buildCurlPreviewFromStep(step, method, rawURL, headers, query, body)
	return res
}

// DebugStep 从场景 request 步骤构造调试请求并复用 Debug，用于"试发送此请求"。

// DebugStep 从场景 request 步骤构造调试请求并复用 Debug，用于"试发送此请求"。
func DebugStep(step *model.Step, vars map[string]string) *DebugResult {
	return Debug(DebugRequest{
		Method:           step.Method,
		URL:              step.URL,
		Headers:          step.Headers,
		Query:            step.Query,
		Body:             step.Body,
		BodyType:         step.BodyType,
		FormData:         step.FormData,
		GraphQLQuery:     step.GraphQLQuery,
		GraphQLVariables: step.GraphQLVariables,
		AuthType:         step.AuthType,
		AuthToken:        step.AuthToken,
		AuthUser:         step.AuthUser,
		AuthPass:         step.AuthPass,
		AuthKeyName:      step.AuthKeyName,
		AuthKeyValue:     step.AuthKeyValue,
		AuthKeyIn:        step.AuthKeyIn,
		AuthTokenURL:     step.AuthTokenURL,
		AuthClientID:     step.AuthClientID,
		AuthClientSecret: step.AuthClientSecret,
		AuthScope:        step.AuthScope,
		AuthGrantType:    step.AuthGrantType,
	}, vars)
}

// mergeQuery 把 query 合并进 URL，用于展示实际请求地址。

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

// shellQuote 用单引号包裹 shell 参数并转义内部的单引号。
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// buildCurlPreviewFromStep 基于渲染后的 step 生成 cURL（含新认证/body 类型）。
