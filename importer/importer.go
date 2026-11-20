// Package importer 把第三方 API 定义（Postman Collection v2.1、OpenAPI 3.x、
// Swagger 2.0）转换为本工程的 model.Scenario，便于复用已有接口资产。
package importer

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"requestflow/model"
)

// Detect 简单识别输入格式：postman / openapi / swagger / unknown。
// 依据顶层特征字段判断，避免要求用户手动选择格式。
func Detect(data []byte) string {
	var top map[string]interface{}
	if json.Unmarshal(data, &top) != nil {
		return "unknown"
	}
	if _, ok := top["info"]; ok {
		if _, ok := top["item"]; ok {
			return "postman" // Postman Collection: info + item[]
		}
	}
	if v, ok := top["openapi"]; ok && strings.HasPrefix(fmt.Sprint(v), "3") {
		return "openapi"
	}
	if _, ok := top["swagger"]; ok {
		return "swagger"
	}
	return "unknown"
}

// Import 自动识别格式并导入，返回场景。
func Import(data []byte) (*model.Scenario, error) {
	switch Detect(data) {
	case "postman":
		return ImportPostman(data)
	case "openapi", "swagger":
		return ImportOpenAPI(data)
	default:
		return nil, fmt.Errorf("无法识别格式：请用 Postman Collection v2.1 / OpenAPI 3.x / Swagger 2.0 文件")
	}
}

// ---------- Postman Collection v2.1 ----------

// ImportPostman 解析 Postman Collection v2.1 JSON，转成场景。
// 支持 folder 递归、request（method/url/header/body/auth）、变量（含 collection 变量）。
func ImportPostman(data []byte) (*model.Scenario, error) {
	var col map[string]interface{}
	if err := json.Unmarshal(data, &col); err != nil {
		return nil, fmt.Errorf("解析 Postman Collection 失败: %w", err)
	}
	info, _ := col["info"].(map[string]interface{})
	name := str(info["name"])
	if name == "" {
		name = "导入的 Postman 集合"
	}
	sc := model.NewScenario(name)

	// collection 变量 → scenario.Variables
	if vars, ok := col["variable"].([]interface{}); ok {
		for _, v := range vars {
			vm, _ := v.(map[string]interface{})
			k := str(vm["key"])
			if k != "" {
				sc.Variables[k] = str(vm["value"])
			}
		}
	}

	steps := make([]model.Step, 0)
	if items, ok := col["item"].([]interface{}); ok {
		steps = importPostmanItems(items, &steps, 0)
	}
	// 去重：importPostmanItems 用 append 语义，这里直接取结果
	sc.Steps = steps
	if len(sc.Steps) == 0 {
		return nil, fmt.Errorf("集合中没有可导入的请求")
	}
	return sc, nil
}

// importPostmanItems 递归处理 item 列表（folder 展开，request 转步骤）。
func importPostmanItems(items []interface{}, _ *[]model.Step, depth int) []model.Step {
	if depth > 20 {
		return nil
	}
	out := make([]model.Step, 0, len(items))
	for _, it := range items {
		m, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		if _, isFolder := m["item"]; isFolder {
			if sub, ok := m["item"].([]interface{}); ok {
				out = append(out, importPostmanItems(sub, nil, depth+1)...)
			}
			continue
		}
		if req, ok := m["request"]; ok {
			step := postmanRequestToStep(str(m["name"]), req)
			out = append(out, step)
		}
	}
	return out
}

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
func ImportOpenAPI(data []byte) (*model.Scenario, error) {
	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("解析 OpenAPI 失败: %w", err)
	}
	info, _ := doc["info"].(map[string]interface{})
	name := str(info["title"])
	if name == "" {
		name = "导入的 API"
	}
	sc := model.NewScenario(name)
	sc.BaseURL = openAPIBaseURL(doc)

	// OpenAPI 变量化 servers：servers[0].variables → scenario.Variables
	if servers, ok := doc["servers"].([]interface{}); ok && len(servers) > 0 {
		if sv, ok := servers[0].(map[string]interface{}); ok {
			if vars, ok := sv["variables"].(map[string]interface{}); ok {
				for k, v := range vars {
					vm, _ := v.(map[string]interface{})
					sc.Variables[k] = str(vm["default"])
				}
			}
		}
	}

	paths, _ := doc["paths"].(map[string]interface{})
	methods := []string{"get", "post", "put", "patch", "delete", "head", "options"}
	pathNames := make([]string, 0, len(paths))
	for p := range paths {
		pathNames = append(pathNames, p)
	}
	sort.Strings(pathNames)

	steps := make([]model.Step, 0)
	for _, p := range pathNames {
		item, _ := paths[p].(map[string]interface{})
		for _, m := range methods {
			op, ok := item[m].(map[string]interface{})
			if !ok {
				continue
			}
			step := openAPIOperationToStep(p, m, op)
			steps = append(steps, step)
		}
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("文档中没有可导入的 operation")
	}
	sc.Steps = steps
	return sc, nil
}

func openAPIBaseURL(doc map[string]interface{}) string {
	if servers, ok := doc["servers"].([]interface{}); ok && len(servers) > 0 {
		if sv, ok := servers[0].(map[string]interface{}); ok {
			u := str(sv["url"])
			if u != "" {
				// OpenAPI server 变量占位 {x} → {{x}}，与本工程模板一致
				return pathVarToTemplate(u)
			}
		}
	}
	// Swagger 2.0：scheme + host + basePath
	host := str(doc["host"])
	base := str(doc["basePath"])
	schemes, _ := doc["schemes"].([]interface{})
	scheme := "https"
	if len(schemes) > 0 {
		scheme = str(schemes[0])
	}
	if host == "" {
		return ""
	}
	return scheme + "://" + host + base
}

func openAPIOperationToStep(path, method string, op map[string]interface{}) model.Step {
	summary := str(op["summary"])
	if summary == "" {
		summary = str(op["operationId"])
	}
	if summary == "" {
		summary = strings.ToUpper(method) + " " + path
	}
	step := model.Step{
		ID: newImpID(), Name: summary, Type: model.StepRequest, Enabled: true,
		Method: strings.ToUpper(method), URL: pathVarToTemplate(path),
	}
	// 路径参数 / 查询参数
	if params, ok := op["parameters"].([]interface{}); ok {
		for _, p := range params {
			pm, _ := p.(map[string]interface{})
			k := str(pm["name"])
			loc := str(pm["in"])
			if k == "" {
				continue
			}
			if loc == "query" {
				if step.Query == nil {
					step.Query = map[string]string{}
				}
				step.Query[k] = "{{" + k + "}}"
			} else if loc == "path" {
				step.URL = strings.ReplaceAll(step.URL, "{{"+k+"}}", "{{"+k+"}}")
			} else if loc == "header" {
				if step.Headers == nil {
					step.Headers = map[string]string{}
				}
				step.Headers[k] = "{{" + k + "}}"
			}
		}
	}
	// requestBody → raw json 示例
	if rb, ok := op["requestBody"].(map[string]interface{}); ok {
		step.BodyType = "raw"
		step.Body = exampleFromContent(rb["content"])
	}
	// Swagger 2.0 body 参数
	if params, ok := op["parameters"].([]interface{}); ok {
		for _, p := range params {
			pm, _ := p.(map[string]interface{})
			if str(pm["in"]) == "body" {
				step.BodyType = "raw"
				step.Body = toJSONString(pm["schema"])
				break
			}
		}
	}
	// security → bearer（粗略：任何 security 需求都注入 bearer 占位）
	if secs, ok := op["security"].([]interface{}); ok && len(secs) > 0 {
		step.AuthType = "bearer"
		step.AuthToken = "{{token}}"
	}
	return step
}

func exampleFromContent(content interface{}) string {
	cm, ok := content.(map[string]interface{})
	if !ok {
		return ""
	}
	// 优先 application/json 的 example
	for _, key := range []string{"application/json", "application/*+json"} {
		if media, ok := cm[key].(map[string]interface{}); ok {
			if ex, ok := media["example"]; ok {
				return toJSONString(ex)
			}
			if ex, ok := media["examples"].(map[string]interface{}); ok {
				for _, v := range ex {
					vm, _ := v.(map[string]interface{})
					if e, ok := vm["value"]; ok {
						return toJSONString(e)
					}
				}
			}
			if schema, ok := media["schema"]; ok {
				return schemaExample(schema)
			}
		}
	}
	return ""
}

// schemaExample 根据 schema 生成一个占位 JSON 示例（递归处理 properties）。
func schemaExample(schema interface{}) string {
	sm, ok := schema.(map[string]interface{})
	if !ok {
		return "{}"
	}
	if ex, ok := sm["example"]; ok {
		return toJSONString(ex)
	}
	props, _ := sm["properties"].(map[string]interface{})
	if len(props) == 0 {
		return "{}"
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		pm, _ := props[k].(map[string]interface{})
		typ := str(pm["type"])
		var val string
		switch typ {
		case "integer", "number":
			val = "0"
		case "boolean":
			val = "false"
		case "array":
			val = "[]"
		case "object":
			val = schemaExample(pm["properties"])
		default:
			val = `"` + k + `"`
		}
		parts = append(parts, `"`+k+`":`+val)
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// ---------- 工具 ----------

func str(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func toJSONString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(b)
}

// pathVarToTemplate 把 {var} 占位转成 {{var}}，避免与本工程模板冲突（已是 {{}} 的不动）。
func pathVarToTemplate(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '{' && i+1 < len(s) && s[i+1] != '{' {
			if j := strings.IndexByte(s[i+1:], '}'); j > 0 {
				b.WriteString("{{" + s[i+1:i+1+j] + "}}")
				i = i + 1 + j + 1
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

var impIDCounter int64

func newImpID() string {
	impIDCounter++
	return fmt.Sprintf("imp-%d", impIDCounter)
}
