package importer

import (
	"strings"

	"requestflow/model"
)

func postmanRequestToStep(name string, req interface{}) model.Step {
	step := model.Step{ID: newImpID(), Name: name, Type: model.StepRequest, Enabled: true, Method: "GET"}
	rm, ok := req.(map[string]interface{})
	if !ok {
		return step
	}
	step.Method = strings.ToUpper(str(rm["method"]))
	if step.Method == "" {
		step.Method = "GET"
	}
	// URL：可能是字符串或 {raw, host, path, query[]} 对象
	switch u := rm["url"].(type) {
	case string:
		step.URL = u
	case map[string]interface{}:
		step.URL = str(u["raw"])
		if step.URL == "" {
			// 拼 host + path
			host := ""
			switch h := u["host"].(type) {
			case string:
				host = h
			case []interface{}:
				parts := make([]string, 0, len(h))
				for _, x := range h {
					parts = append(parts, str(x))
				}
				host = strings.Join(parts, ".")
			}
			path := ""
			if pa, ok := u["path"].([]interface{}); ok {
				segs := make([]string, 0, len(pa))
				for _, x := range pa {
					segs = append(segs, str(x))
				}
				path = strings.Join(segs, "/")
			}
			step.URL = host
			if path != "" {
				step.URL = strings.TrimRight(host, "/") + "/" + strings.TrimLeft(path, "/")
			}
		}
		if qa, ok := u["query"].([]interface{}); ok {
			for _, q := range qa {
				qm, _ := q.(map[string]interface{})
				k := str(qm["key"])
				if k != "" && str(qm["value"]) != "" {
					if step.Query == nil {
						step.Query = map[string]string{}
					}
					step.Query[k] = str(qm["value"])
				}
			}
		}
	}

	// Headers
	if ha, ok := rm["header"].([]interface{}); ok {
		for _, h := range ha {
			hm, _ := h.(map[string]interface{})
			k := str(hm["key"])
			if k == "" {
				continue
			}
			if step.Headers == nil {
				step.Headers = map[string]string{}
			}
			step.Headers[k] = str(hm["value"])
		}
	}

	// Auth
	if auth, ok := rm["auth"].(map[string]interface{}); ok {
		applyPostmanAuth(&step, auth)
	}

	// Body
	if body, ok := rm["body"].(map[string]interface{}); ok {
		applyPostmanBody(&step, body)
	}

	return step
}

func applyPostmanAuth(step *model.Step, auth map[string]interface{}) {
	typ := str(auth["type"])
	switch typ {
	case "bearer":
		step.AuthType = "bearer"
		for _, kv := range authKVs(auth["bearer"]) {
			if kv.key == "token" {
				step.AuthToken = kv.value
			}
		}
	case "basic":
		step.AuthType = "basic"
		for _, kv := range authKVs(auth["basic"]) {
			switch kv.key {
			case "username":
				step.AuthUser = kv.value
			case "password":
				step.AuthPass = kv.value
			}
		}
	case "apikey":
		step.AuthType = "apikey"
		for _, kv := range authKVs(auth["apikey"]) {
			switch kv.key {
			case "key":
				step.AuthKeyName = kv.value
			case "value":
				step.AuthKeyValue = kv.value
			case "in":
				step.AuthKeyIn = kv.value
			}
		}
	}
}

type authKV struct{ key, value string }

func authKVs(v interface{}) []authKV {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]authKV, 0, len(arr))
	for _, x := range arr {
		m, _ := x.(map[string]interface{})
		out = append(out, authKV{str(m["key"]), str(m["value"])})
	}
	return out
}

func applyPostmanBody(step *model.Step, body map[string]interface{}) {
	mode := str(body["mode"])
	switch mode {
	case "raw":
		step.Body = str(body["raw"])
		step.BodyType = "raw"
		// 尝试从 header 里找 Content-Type，用于保留 JSON 语义
		if opts, ok := body["options"].(map[string]interface{}); ok {
			_ = opts
		}
	case "formdata":
		step.BodyType = "form-data"
		if arr, ok := body["formdata"].([]interface{}); ok {
			for _, x := range arr {
				f, _ := x.(map[string]interface{})
				field := model.FormField{Key: str(f["key"]), Value: str(f["value"])}
				if str(f["type"]) == "file" {
					field.Type = "file"
					field.Value = str(f["src"])
				}
				if field.Key != "" {
					step.FormData = append(step.FormData, field)
				}
			}
		}
	case "urlencoded":
		step.BodyType = "urlencoded"
		if arr, ok := body["urlencoded"].([]interface{}); ok {
			for _, x := range arr {
				f, _ := x.(map[string]interface{})
				if k := str(f["key"]); k != "" {
					step.FormData = append(step.FormData, model.FormField{Key: k, Value: str(f["value"])})
				}
			}
		}
	case "graphql":
		step.BodyType = "graphql"
		if g, ok := body["graphql"].(map[string]interface{}); ok {
			step.GraphQLQuery = str(g["query"])
			step.GraphQLVariables = str(g["variables"])
		}
	}
}

// ---------- OpenAPI 3.x / Swagger 2.0 ----------

// ImportOpenAPI 解析 OpenAPI/Swagger 的 paths，把每个 operation 转成一个请求步骤。
// baseUrl 取 servers[0].url（OpenAPI）或 host+basePath（Swagger），支持 {{变量}} 占位。
