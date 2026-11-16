package engine

import (
	"fmt"
	"net/url"
	"strings"

	"requestflow/model"
)

// CurlRequest 是从 cURL 命令解析出的请求信息。
type CurlRequest struct {
	Method    string
	URL       string
	Headers   map[string]string
	Query     map[string]string
	Body      string
	Cookie    string // Cookie 请求头原始值
	AuthType  string // none | bearer | basic
	AuthToken string // bearer token
	AuthUser  string // basic 用户名
	AuthPass  string // basic 密码
}

// ParseCurl 解析 cURL 命令字符串，支持 Chrome DevTools "Copy as cURL" 的
// bash（单引号 + 反斜杠续行）、cmd（双引号 + ^ 续行）、PowerShell（反引号续行）三种格式。
func ParseCurl(s string) (*CurlRequest, error) {
	tokens, err := tokenizeCurl(s)
	if err != nil {
		return nil, err
	}
	req := &CurlRequest{
		Method:  "GET",
		Headers: map[string]string{},
		Query:   map[string]string{},
	}
	var bodyParts []string
	var positional []string

	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		lower := strings.ToLower(t)

		// 跳过程序名
		if lower == "curl" || lower == "curl.exe" {
			continue
		}

		// 带值的选项（传入原始 token 以区分 -X 与 -x 等大小写敏感选项）
		if val, ok := curlValueOption(t, lower); ok {
			if i+1 >= len(tokens) {
				return nil, fmt.Errorf("选项 %s 缺少值", t)
			}
			i++
			v := tokens[i]
			switch val {
			case "method":
				req.Method = strings.ToUpper(v)
			case "header":
				k, vv, ok := splitHeader(v)
				if ok {
					req.Headers[k] = vv
				}
			case "url":
				req.URL = v
			case "data":
				bodyParts = append(bodyParts, v)
			case "cookie":
				req.Cookie = joinCookie(req.Cookie, v)
			case "user":
				u, p := splitBasic(v)
				req.AuthUser, req.AuthPass, req.AuthType = u, p, "basic"
			case "user-agent":
				req.Headers["User-Agent"] = v
			case "referer":
				req.Headers["Referer"] = v
			case "ignore":
				// 忽略该选项的值（如 --max-time 5）
			}
			continue
		}

		// 无值标志
		if curlBoolFlag(lower) {
			continue
		}

		// 未知的选项：跳过其可能的参数（保守处理，不误把值当 URL）
		if strings.HasPrefix(t, "-") {
			continue
		}

		positional = append(positional, t)
	}

	// URL：优先 --url 指定的，否则取第一个位置参数
	if req.URL == "" && len(positional) > 0 {
		req.URL = positional[0]
	}
	if req.URL == "" {
		return nil, fmt.Errorf("未找到 URL，请确认粘贴了完整的 cURL 命令")
	}

	// 有 --data 时默认 POST（除非显式指定了 -X）
	if len(bodyParts) > 0 && !curlHasMethod(tokens) {
		req.Method = "POST"
	}
	req.Body = strings.Join(bodyParts, "&")

	// 解析 URL：拆分 query 到 Query map
	if u, err := url.Parse(req.URL); err == nil {
		q := u.Query()
		for k, vs := range q {
			if len(vs) > 0 {
				req.Query[k] = vs[0]
			}
		}
		u.RawQuery = ""
		req.URL = u.String()
	}

	// 规范化 headers：提取 Authorization、Cookie，过滤无意义头
	clean := map[string]string{}
	for k, v := range req.Headers {
		lk := strings.ToLower(strings.TrimSpace(k))
		if isIgnoredHeader(lk) {
			continue
		}
		switch lk {
		case "authorization":
			if strings.HasPrefix(strings.ToLower(v), "bearer ") {
				req.AuthType = "bearer"
				req.AuthToken = strings.TrimSpace(v[len("Bearer "):])
			} else if strings.HasPrefix(strings.ToLower(v), "basic ") {
				u, p := splitBasic(strings.TrimSpace(v[len("Basic "):]))
				req.AuthUser, req.AuthPass, req.AuthType = u, p, "basic"
			} else {
				clean[k] = v
			}
		case "cookie":
			req.Cookie = joinCookie(req.Cookie, v)
		default:
			clean[k] = v
		}
	}
	req.Headers = clean
	if req.AuthType == "" {
		req.AuthType = "none"
	}
	return req, nil
}

// ToStep 把解析结果转换为一个 request 步骤。
// baseURL 非空时，若请求 URL 以该前缀开头，则替换为 {{baseUrl}} 便于环境切换。
func (r *CurlRequest) ToStep(name, baseURL string) model.Step {
	u := r.URL
	if baseURL != "" && strings.HasPrefix(u, baseURL) {
		u = "{{baseUrl}}" + strings.TrimPrefix(u, baseURL)
	}
	headers := map[string]string{}
	for k, v := range r.Headers {
		headers[k] = v
	}
	if r.Cookie != "" {
		headers["Cookie"] = r.Cookie
	}
	if name == "" {
		name = r.Method + " " + shortURL(u)
	}
	return model.Step{
		ID:        newStepID(),
		Name:      name,
		Type:      model.StepRequest,
		Enabled:   true,
		Method:    r.Method,
		URL:       u,
		Headers:   headers,
		Query:     r.Query,
		Body:      r.Body,
		AuthType:  r.AuthType,
		AuthToken: r.AuthToken,
		AuthUser:  r.AuthUser,
		AuthPass:  r.AuthPass,
	}
}

// shortURL 去掉协议，取路径用于步骤默认名。
func shortURL(u string) string {
	if i := strings.Index(u, "://"); i >= 0 {
		u = u[i+3:]
	}
	if len(u) > 40 {
		return u[:40] + "…"
	}
	return u
}

// newStepID 生成一个简单的唯一 ID（时间无关，仅界面定位用）。
func newStepID() string {
	curlSeq++
	return fmt.Sprintf("curl-%d", curlSeq)
}

var curlSeq int

// splitHeader 把 "Key: value" 拆成键值对。
func splitHeader(s string) (string, string, bool) {
	i := strings.Index(s, ":")
	if i <= 0 {
		return "", "", false
	}
	return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:]), true
}

// joinCookie 合并多个 Cookie 来源。
func joinCookie(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "; " + b
}

// splitBasic 解析 user:pass。
func splitBasic(s string) (string, string) {
	if i := strings.Index(s, ":"); i > 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

// isIgnoredHeader 过滤掉对导入无意义或会干扰 Go HTTP 客户端的头。
func isIgnoredHeader(k string) bool {
	switch k {
	case "accept-encoding", "content-length", "connection", "host", "authority",
		":authority", ":method", ":path", ":scheme", "upgrade-insecure-requests",
		"sec-fetch-dest", "sec-fetch-mode", "sec-fetch-site", "sec-ch-ua",
		"sec-ch-ua-mobile", "sec-ch-ua-platform", "priority":
		return true
	}
	return false
}

// curlValueOption 把选项映射为语义名。orig 用于区分大小写敏感选项
// （-X 指定方法、-x 指定代理），lower 用于其余长选项匹配。
func curlValueOption(orig, lower string) (string, bool) {
	if orig == "-X" {
		return "method", true
	}
	if orig == "-x" {
		return "ignore", true // 小写 -x 是 --proxy
	}
	switch lower {
	case "--request":
		return "method", true
	case "-h", "--header":
		return "header", true
	case "--url":
		return "url", true
	case "-d", "--data", "--data-raw", "--data-binary", "--data-ascii", "--data-urlencode":
		return "data", true
	case "-b", "--cookie":
		return "cookie", true
	case "-u", "--user":
		return "user", true
	case "-a", "--user-agent":
		return "user-agent", true
	case "-e", "--referer":
		return "referer", true
	case "-m", "--max-time", "--connect-timeout", "-o", "--output", "--proxy",
		"--resolve", "-w", "--write-out", "--cacert", "--cert", "--key", "-t", "--form":
		return "ignore", true
	}
	return "", false
}

// curlBoolFlag 判断是否为无值标志。
func curlBoolFlag(lower string) bool {
	switch lower {
	case "--compressed", "-k", "--insecure", "-l", "--location", "-v", "--verbose",
		"-s", "--silent", "--show-error", "-i", "--include", "-I", "--head",
		"--http1.1", "--http2", "-#", "--progress-bar", "-g", "--globoff", "--location-trusted":
		return true
	}
	return false
}

// curlHasMethod 检查命令里是否显式指定了请求方法。
func curlHasMethod(tokens []string) bool {
	for _, t := range tokens {
		if t == "-X" || strings.ToLower(t) == "--request" {
			return true
		}
	}
	return false
}

// tokenizeCurl 把 cURL 命令按 shell 规则切分，支持单引号、双引号、转义、
// 以及 bash 的 \ 续行、cmd 的 ^ 续行、PowerShell 的 ` 续行。
func tokenizeCurl(s string) ([]string, error) {
	var tokens []string
	var cur strings.Builder
	inSingle, inDouble, escaped := false, false, false
	has := false

	flush := func() {
		if has || cur.Len() > 0 {
			tokens = append(tokens, cur.String())
			cur.Reset()
			has = false
		}
	}

	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		if escaped {
			switch ch {
			case '\n', '\r', '^', '`':
				// 续行符：跳过
			case 'n':
				cur.WriteRune('\n')
			case 't':
				cur.WriteRune('\t')
			default:
				cur.WriteRune(ch)
			}
			escaped = false
			has = true
			continue
		}
		if ch == '\\' && !inSingle {
			escaped = true
			continue
		}
		// cmd 的 ^ 续行、PowerShell 的 ` 续行：仅当紧跟换行时生效（可在引号内）
		if (ch == '^' || ch == '`') && i+1 < len(runes) && (runes[i+1] == '\n' || runes[i+1] == '\r') {
			i++ // 跳过后续换行
			continue
		}
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			has = true
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			has = true
			continue
		}
		if (ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r') && !inSingle && !inDouble {
			flush()
			continue
		}
		cur.WriteRune(ch)
		has = true
	}
	if escaped {
		cur.WriteRune('\\')
	}
	flush()
	if inSingle || inDouble {
		return nil, fmt.Errorf("cURL 命令中有未闭合的引号")
	}
	return tokens, nil
}
