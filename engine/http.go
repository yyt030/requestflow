package engine

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
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
func (h *HTTPClient) Cookies(host string) []*http.Cookie {
	u, err := url.Parse(host)
	if err != nil {
		return nil
	}
	return h.jar.Cookies(u)
}

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

// buildBody 根据 step.BodyType 生成请求体 reader 与默认 Content-Type。
func buildBody(step *model.Step) (io.Reader, string, error) {
	if step == nil {
		return nil, "", nil
	}
	switch strings.ToLower(strings.TrimSpace(step.BodyType)) {
	case "none":
		return nil, "", nil
	case "form-data":
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		for _, f := range step.FormData {
			if f.Key == "" {
				continue
			}
			if strings.EqualFold(f.Type, "file") {
				if f.Value == "" {
					continue
				}
				file, err := os.Open(f.Value)
				if err != nil {
					return nil, "", fmt.Errorf("读取上传文件 %q: %w", f.Value, err)
				}
				part, err := w.CreateFormFile(f.Key, filepath.Base(f.Value))
				if err != nil {
					file.Close()
					return nil, "", err
				}
				if _, err := io.Copy(part, file); err != nil {
					file.Close()
					return nil, "", err
				}
				file.Close()
			} else {
				if err := w.WriteField(f.Key, f.Value); err != nil {
					return nil, "", err
				}
			}
		}
		if err := w.Close(); err != nil {
			return nil, "", err
		}
		return bytes.NewReader(buf.Bytes()), w.FormDataContentType(), nil
	case "urlencoded":
		vals := url.Values{}
		for _, f := range step.FormData {
			if f.Key != "" {
				vals.Add(f.Key, f.Value)
			}
		}
		return strings.NewReader(vals.Encode()), "application/x-www-form-urlencoded", nil
	case "binary":
		if step.Body == "" {
			return nil, "application/octet-stream", nil
		}
		data, err := os.ReadFile(step.Body)
		if err != nil {
			return nil, "", fmt.Errorf("读取二进制请求体 %q: %w", step.Body, err)
		}
		return bytes.NewReader(data), "application/octet-stream", nil
	case "graphql":
		vars := map[string]interface{}{}
		if strings.TrimSpace(step.GraphQLVariables) != "" {
			if err := json.Unmarshal([]byte(step.GraphQLVariables), &vars); err != nil {
				return nil, "", fmt.Errorf("GraphQL variables 不是合法 JSON: %w", err)
			}
		}
		payload := map[string]interface{}{"query": step.GraphQLQuery, "variables": vars}
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, "", err
		}
		return bytes.NewReader(b), "application/json", nil
	default: // raw / 空：兼容旧场景
		if step.Body == "" {
			return nil, "", nil
		}
		ct := ""
		trimmed := strings.TrimSpace(step.Body)
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			ct = "application/json"
		}
		return strings.NewReader(step.Body), ct, nil
	}
}

// applyAuth 注入认证：bearer / basic / apikey(header) / oauth2。
func (h *HTTPClient) applyAuth(req *http.Request, step *model.Step, headers map[string]string) error {
	switch strings.ToLower(strings.TrimSpace(step.AuthType)) {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+step.AuthToken)
	case "basic":
		req.SetBasicAuth(step.AuthUser, step.AuthPass)
	case "apikey":
		if strings.EqualFold(step.AuthKeyIn, "query") {
			return nil // query 已在 URL 拼接阶段处理
		}
		if step.AuthKeyName != "" {
			req.Header.Set(step.AuthKeyName, step.AuthKeyValue)
		}
	case "oauth2":
		token, err := h.oauthToken(step)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return nil
}

// oauthToken 获取（或复用缓存的）OAuth2 access_token。
func (h *HTTPClient) oauthToken(step *model.Step) (string, error) {
	if strings.TrimSpace(step.AuthTokenURL) == "" {
		return "", fmt.Errorf("oauth2 缺少 Token URL")
	}
	key := strings.Join([]string{step.AuthTokenURL, step.AuthClientID, step.AuthGrantType, step.AuthScope}, "|")
	h.tokenMu.Lock()
	if t, ok := h.tokenCache[key]; ok && time.Now().Before(t.expires) {
		h.tokenMu.Unlock()
		return t.access, nil
	}
	h.tokenMu.Unlock()

	vals := url.Values{}
	grant := strings.ToLower(strings.TrimSpace(step.AuthGrantType))
	if grant == "" {
		grant = "client_credentials"
	}
	vals.Set("grant_type", grant)
	vals.Set("client_id", step.AuthClientID)
	vals.Set("client_secret", step.AuthClientSecret)
	if step.AuthScope != "" {
		vals.Set("scope", step.AuthScope)
	}
	if grant == "password" {
		vals.Set("username", step.AuthUser)
		vals.Set("password", step.AuthPass)
	}
	httpReq, err := http.NewRequest(http.MethodPost, step.AuthTokenURL, strings.NewReader(vals.Encode()))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := h.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("oauth2 获取 token 失败: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var parsed struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil || parsed.AccessToken == "" {
		return "", fmt.Errorf("oauth2 token 响应无效: %s", strings.TrimSpace(string(data)))
	}
	ttl := time.Duration(parsed.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	// 提前 30 秒过期，避免边界失效
	if ttl > 30*time.Second {
		ttl -= 30 * time.Second
	}
	h.tokenMu.Lock()
	h.tokenCache[key] = oauthToken{access: parsed.AccessToken, expires: time.Now().Add(ttl)}
	h.tokenMu.Unlock()
	return parsed.AccessToken, nil
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
