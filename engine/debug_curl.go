package engine

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"requestflow/model"
)

// buildCurlPreviewFromStep 基于渲染后的 step 生成 cURL（含新认证/body 类型）。
func buildCurlPreviewFromStep(step *model.Step, method, rawURL string, headers, query map[string]string, body string) string {
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
	if step != nil {
		switch strings.ToLower(step.AuthType) {
		case "bearer":
			all["Authorization"] = "Bearer " + step.AuthToken
		case "basic":
			token := base64.StdEncoding.EncodeToString([]byte(step.AuthUser + ":" + step.AuthPass))
			all["Authorization"] = "Basic " + token
		case "apikey":
			if !strings.EqualFold(step.AuthKeyIn, "query") && step.AuthKeyName != "" {
				all[step.AuthKeyName] = step.AuthKeyValue
			}
		}
		// 表单/GraphQL 的 body 由 buildBody 处理，此处仅展示可读提示
		switch strings.ToLower(step.BodyType) {
		case "form-data", "urlencoded":
			for _, f := range step.FormData {
				all["# form "+f.Key] = f.Value
			}
		case "graphql":
			all["Content-Type"] = "application/json"
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
	if strings.EqualFold(strings.TrimSpace(step.BodyType), "graphql") {
		vars := step.GraphQLVariables
		if strings.TrimSpace(vars) == "" {
			vars = "{}"
		}
		payload := fmt.Sprintf(`{"query":%s,"variables":%s}`, shellQuote(step.GraphQLQuery), vars)
		parts = append(parts, "--data-raw "+shellQuote("(graphql) "+payload))
	} else if body != "" {
		parts = append(parts, "--data-raw "+shellQuote(body))
	}
	return strings.Join(parts, " \\\n  ")
}

// BuildCurlPreview 把请求配置反向生成 cURL 命令，方便复制复现（请求预览，兼容旧调用）。

// BuildCurlPreview 把请求配置反向生成 cURL 命令，方便复制复现（请求预览，兼容旧调用）。
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
