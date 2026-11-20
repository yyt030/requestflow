package importer

import (
	"testing"

	"requestflow/model"
)

const postmanSample = `{
  "info": {"name": "电商 API", "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},
  "variable": [{"key": "baseUrl", "value": "https://api.example.com"}],
  "item": [
    {"name": "登录", "request": {
      "method": "POST",
      "header": [{"key": "Content-Type", "value": "application/json"}],
      "body": {"mode": "raw", "raw": "{\"user\":\"{{user}}\"}"},
      "auth": {"type": "bearer", "bearer": [{"key":"token","value":"{{token}}"}]},
      "url": {"raw": "{{baseUrl}}/login", "host": ["{{baseUrl}}"], "path": ["login"]}
    }},
    {"name": "订单分组", "item": [
      {"name": "查订单", "request": {"method": "GET", "url": "{{baseUrl}}/orders?id=1"}}
    ]}
  ]
}`

func TestImportPostman(t *testing.T) {
	if got := Detect([]byte(postmanSample)); got != "postman" {
		t.Fatalf("Detect: 期望 postman，得到 %s", got)
	}
	sc, err := ImportPostman([]byte(postmanSample))
	if err != nil {
		t.Fatalf("ImportPostman 失败: %v", err)
	}
	if sc.Name != "电商 API" {
		t.Fatalf("场景名错误: %s", sc.Name)
	}
	if sc.Variables["baseUrl"] != "https://api.example.com" {
		t.Fatalf("collection 变量未导入: %v", sc.Variables)
	}
	if len(sc.Steps) != 2 {
		t.Fatalf("期望 2 个步骤，得到 %d", len(sc.Steps))
	}
	login := sc.Steps[0]
	if login.Method != "POST" || login.URL != "{{baseUrl}}/login" {
		t.Fatalf("登录步骤错误: %s %s", login.Method, login.URL)
	}
	if login.AuthType != "bearer" || login.AuthToken != "{{token}}" {
		t.Fatalf("认证未导入: %+v", login)
	}
	if login.Headers["Content-Type"] != "application/json" {
		t.Fatalf("请求头未导入: %v", login.Headers)
	}
	if sc.Steps[1].Name != "查订单" {
		t.Fatalf("folder 未展开: %s", sc.Steps[1].Name)
	}
}

const openAPISample = `{
  "openapi": "3.0.0",
  "info": {"title": "用户服务"},
  "servers": [{"url": "https://api.example.com/v1", "variables": {"version": {"default": "v1"}}}],
  "paths": {
    "/users/{id}": {
      "get": {
        "summary": "查用户",
        "parameters": [
          {"name": "id", "in": "path", "required": true},
          {"name": "verbose", "in": "query"}
        ],
        "security": [{"bearerAuth": []}]
      }
    },
    "/users": {
      "post": {
        "operationId": "createUser",
        "requestBody": {"content": {"application/json": {"schema": {
          "type": "object",
          "properties": {"name": {"type": "string"}, "age": {"type": "integer"}}
        }}}}
      }
    }
  }
}`

func TestImportOpenAPI(t *testing.T) {
	if got := Detect([]byte(openAPISample)); got != "openapi" {
		t.Fatalf("Detect: 期望 openapi，得到 %s", got)
	}
	sc, err := ImportOpenAPI([]byte(openAPISample))
	if err != nil {
		t.Fatalf("ImportOpenAPI 失败: %v", err)
	}
	if len(sc.Steps) != 2 {
		t.Fatalf("期望 2 个步骤，得到 %d", len(sc.Steps))
	}
	// 按方法定位步骤（顺序由路径排序决定）
	var getStep, postStep *model.Step
	for i := range sc.Steps {
		if sc.Steps[i].Method == "GET" {
			getStep = &sc.Steps[i]
		}
		if sc.Steps[i].Method == "POST" {
			postStep = &sc.Steps[i]
		}
	}
	if getStep == nil || getStep.URL != "/users/{{id}}" {
		t.Fatalf("GET 步骤错误: %+v", getStep)
	}
	if getStep.Query["verbose"] != "{{verbose}}" {
		t.Fatalf("query 参数未导入: %v", getStep.Query)
	}
	if getStep.AuthType != "bearer" {
		t.Fatalf("security 未导入: %s", getStep.AuthType)
	}
	if postStep == nil || postStep.Name != "createUser" {
		t.Fatalf("POST 步骤错误: %+v", postStep)
	}
	if postStep.Body == "" {
		t.Fatalf("requestBody 示例未生成")
	}
}

func TestImportAutoDetect(t *testing.T) {
	sc, err := Import([]byte(postmanSample))
	if err != nil || len(sc.Steps) != 2 {
		t.Fatalf("自动检测导入失败: %v", err)
	}
	if _, err := Import([]byte(`{"foo":"bar"}`)); err == nil {
		t.Fatal("未知格式应返回错误")
	}
}
