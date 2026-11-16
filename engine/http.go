package engine

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"requestflow/model"
)

// HTTPClient 封装一个共享 CookieJar 的客户端，使登录步骤设置的会话 cookie
// 在后续所有步骤中自动携带（Cookie 会话支持）。
type HTTPClient struct {
	client *http.Client
	jar    *cookiejar.Jar
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
	}
}

// Cookies 返回当前 jar 中指定 host 的 cookie（用于 UI 展示与调试）。
func (h *HTTPClient) Cookies(host string) []*http.Cookie {
	u, err := url.Parse(host)
	if err != nil {
		return nil
	}
	return h.jar.Cookies(u)
}

// Do 根据步骤配置（已渲染）发起一次请求。
func (h *HTTPClient) Do(method, rawURL string, headers, query map[string]string, body string, auth *model.Step) (*model.Response, error) {
	if method == "" {
		method = http.MethodGet
	}
	method = strings.ToUpper(method)

	// 拼接 query
	if len(query) > 0 {
		u, err := url.Parse(rawURL)
		if err != nil {
			return nil, err
		}
		q := u.Query()
		for k, v := range query {
			q.Add(k, v)
		}
		u.RawQuery = q.Encode()
		rawURL = u.String()
	}

	var bodyReader io.Reader
	if body != "" {
		bodyReader = bytes.NewBufferString(body)
	}
	req, err := http.NewRequest(method, rawURL, bodyReader)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	switch strings.ToLower(auth.AuthType) {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+auth.AuthToken)
	case "basic":
		req.SetBasicAuth(auth.AuthUser, auth.AuthPass)
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

	out := &model.Response{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       data,
		Text:       string(data),
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

// responseView 把响应转换为供脚本/变量引用的结构化 map。
func responseView(resp *model.Response) map[string]interface{} {
	return map[string]interface{}{
		"status":  resp.StatusCode,
		"headers": headerMap(resp.Headers),
		"body":    resp.Text,
		"json":    resp.JSON,
	}
}

func headerMap(h map[string][]string) map[string]string {
	out := make(map[string]string, len(h))
	for k, v := range h {
		if len(v) > 0 {
			out[k] = v[0]
		}
	}
	return out
}
