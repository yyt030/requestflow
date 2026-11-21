package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"requestflow/model"
)

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
