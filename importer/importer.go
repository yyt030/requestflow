// Package importer 把第三方 API 定义（Postman Collection v2.1、OpenAPI 3.x、
// Swagger 2.0）转换为本工程的 model.Scenario，便于复用已有接口资产。
package importer

import (
	"encoding/json"
	"fmt"
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
