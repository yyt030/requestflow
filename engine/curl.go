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
