package importer

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"requestflow/model"
)

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
