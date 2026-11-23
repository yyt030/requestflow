package engine

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"requestflow/model"
)

// HTTPClient 封装一个共享 CookieJar 的客户端，使登录步骤设置的会话 cookie
// 在后续所有步骤中自动携带（Cookie 会话支持）。
type HTTPClient struct {
	client *http.Client
	jar    *cookiejar.Jar

	// oauth2 token 缓存：同一 tokenURL+clientID+grant 只获取一次，过期前复用。
	tokenMu    sync.Mutex
	tokenCache map[string]oauthToken
}

type oauthToken struct {
	access  string
	expires time.Time
}

func NewHTTPClient(timeout time.Duration, insecureTLS bool) *HTTPClient {
	jar, _ := cookiejar.New(nil)
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureTLS},
	}
	return &HTTPClient{
		jar: jar,
		client: &http.Client{
			Jar:       jar,
			Timeout:   timeout,
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
		tokenCache: map[string]oauthToken{},
	}
}

// Cookies 返回当前 jar 中指定 host 的 cookie（用于 UI 展示与调试）。

// Cookies 返回当前 jar 中指定 host 的 cookie（用于 UI 展示与调试）。
func (h *HTTPClient) Cookies(host string) []*http.Cookie {
	u, err := url.Parse(host)
	if err != nil {
		return nil
	}
	return h.jar.Cookies(u)
}

// Do 根据步骤配置（已渲染）发起一次请求（兼容旧调用）。
// body 为 raw 文本；auth 承载认证配置；bodyType/formData/graphql 为空时按 raw 处理。

// Do 根据步骤配置（已渲染）发起一次请求（兼容旧调用）。
// body 为 raw 文本；auth 承载认证配置；bodyType/formData/graphql 为空时按 raw 处理。
func (h *HTTPClient) Do(method, rawURL string, headers, query map[string]string, body string, auth *model.Step) (*model.Response, error) {
	step := &model.Step{}
	if auth != nil {
		*step = *auth
	}
	step.Body = body
	return h.DoRequest(step, method, rawURL, headers, query)
}

// DoRequest 发起一次请求，step 提供 Body/BodyType/FormData/GraphQL 与认证字段（均已渲染）。

// DoRequest 发起一次请求，step 提供 Body/BodyType/FormData/GraphQL 与认证字段（均已渲染）。
func (h *HTTPClient) DoRequest(step *model.Step, method, rawURL string, headers, query map[string]string) (*model.Response, error) {
	if method == "" {
		method = http.MethodGet
	}
	method = strings.ToUpper(method)

	// 拼接 query（API Key in=query 也在此注入）。始终基于 URL 现有 query 构建，
	// 避免注入 apikey 时丢掉 URL 里已有的查询串。
	apiKeyInQuery := step != nil && strings.EqualFold(step.AuthType, "apikey") &&
		strings.EqualFold(step.AuthKeyIn, "query") && step.AuthKeyName != ""
	q := url.Values{}
	if len(query) > 0 || apiKeyInQuery {
		u, err := url.Parse(rawURL)
		if err != nil {
			return nil, err
		}
		q = u.Query()
		for k, v := range query {
			q.Add(k, v)
		}
		if apiKeyInQuery {
			q.Add(step.AuthKeyName, step.AuthKeyValue)
		}
		u.RawQuery = q.Encode()
		rawURL = u.String()
	}

	// 构建请求体（按 bodyType 编码）
	bodyReader, contentType, err := buildBody(step)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, rawURL, bodyReader)
	if err != nil {
		return nil, err
	}
	hasContentType := false
	for k, v := range headers {
		req.Header.Set(k, v)
		if strings.EqualFold(k, "Content-Type") {
			hasContentType = true
		}
	}
	if contentType != "" && !hasContentType {
		req.Header.Set("Content-Type", contentType)
	}

	// 认证注入
	if step != nil {
		if err := h.applyAuth(req, step, headers); err != nil {
			return nil, err
		}
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 解压 + charset 转码：Go 传输层仅在未显式设置 Accept-Encoding 时自动解压
	// gzip；用户从浏览器粘贴 Accept-Encoding 或服务端返回 br/GBK 时需在此兜底。
	decoded := decodeResponseBody(data, resp.Header)

	out := &model.Response{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       decoded,
		Text:       string(decoded),
	}
	// Transfer-Encoding 由 net/http 放在 resp.TransferEncoding，不在 resp.Header，
	// 补进展示用 Header，避免调试面板看不到 chunked 等分块传输标记。
	if len(resp.TransferEncoding) > 0 {
		if out.Headers == nil {
			out.Headers = map[string][]string{}
		}
		if _, exists := out.Headers["Transfer-Encoding"]; !exists {
			out.Headers["Transfer-Encoding"] = []string{strings.Join(resp.TransferEncoding, ", ")}
		}
	}
	for _, c := range resp.Cookies() {
		out.Cookies = append(out.Cookies, &model.HTTPCookie{Name: c.Name, Value: c.Value})
	}
	// 尝试解析 JSON，失败则保持 nil（不影响纯文本响应）
	var parsed interface{}
	trimmed := strings.TrimSpace(out.Text)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		if json.Unmarshal(data, &parsed) == nil {
			out.JSON = parsed
		}
	}
	return out, nil
}

// buildBody 根据 step.BodyType 生成请求体 reader 与默认 Content-Type。
