package engine

import (
	"strings"
	"testing"

	"requestflow/model"
)

func TestBuildSnippet(t *testing.T) {
	headers := map[string]string{"Content-Type": "application/json", "X-Token": "abc"}
	query := map[string]string{"page": "1"}
	step := &model.Step{AuthType: "bearer", AuthToken: "xyz"}

	py := BuildSnippet("python", "POST", "https://api.example.com/users", headers, query, `{"a":1}`, step)
	if !strings.Contains(py, "import requests") || !strings.Contains(py, "requests.post") {
		t.Fatalf("Python snippet 错误:\n%s", py)
	}
	if !strings.Contains(py, "page=1") {
		t.Fatalf("Python snippet 未含 query:\n%s", py)
	}
	if !strings.Contains(py, "Bearer xyz") {
		t.Fatalf("Python snippet 未含认证头:\n%s", py)
	}

	goCode := BuildSnippet("go", "GET", "https://api.example.com/users", headers, query, "", step)
	if !strings.Contains(goCode, "http.NewRequest") || !strings.Contains(goCode, "req.Header.Set") {
		t.Fatalf("Go snippet 错误:\n%s", goCode)
	}

	js := BuildSnippet("javascript", "POST", "https://api.example.com/users", headers, query, `{"a":1}`, step)
	if !strings.Contains(js, "fetch(") || !strings.Contains(js, "method: \"POST\"") {
		t.Fatalf("JS snippet 错误:\n%s", js)
	}

	curl := BuildSnippet("curl", "GET", "https://api.example.com/users", headers, query, "", step)
	if !strings.Contains(curl, "curl") {
		t.Fatalf("cURL snippet 错误:\n%s", curl)
	}
}

func TestBuildSnippetFormData(t *testing.T) {
	step := &model.Step{BodyType: "form-data", FormData: []model.FormField{
		{Key: "username", Value: "zhangsan"},
		{Key: "password", Value: "123456"},
	}}
	py := BuildSnippet("python", "POST", "https://api.example.com/login", nil, nil, "", step)
	if !strings.Contains(py, "username=zhangsan") || !strings.Contains(py, "password=123456") {
		t.Fatalf("form-data snippet 未含字段:\n%s", py)
	}
}
