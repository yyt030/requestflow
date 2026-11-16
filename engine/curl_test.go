package engine

import (
	"strings"
	"testing"
)

func TestParseCurl_BashWithAuthAndCookie(t *testing.T) {
	raw := `curl 'https://api.example.com/orders?page=2&size=20' \
  -H 'authority: api.example.com' \
  -H 'accept: application/json' \
  -H 'authorization: Bearer eyJhbGciOiJIUzI1NiJ9.xxx' \
  -H 'content-type: application/json' \
  -H 'cookie: session=abc123; theme=dark' \
  -H 'user-agent: Mozilla/5.0' \
  --compressed`

	req, err := ParseCurl(raw)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "GET" {
		t.Fatalf("method=%q 期望 GET", req.Method)
	}
	if req.URL != "https://api.example.com/orders" {
		t.Fatalf("url=%q", req.URL)
	}
	if req.Query["page"] != "2" || req.Query["size"] != "20" {
		t.Fatalf("query=%v", req.Query)
	}
	if req.AuthType != "bearer" || req.AuthToken != "eyJhbGciOiJIUzI1NiJ9.xxx" {
		t.Fatalf("auth=%q token=%q", req.AuthType, req.AuthToken)
	}
	if !strings.Contains(req.Cookie, "session=abc123") || !strings.Contains(req.Cookie, "theme=dark") {
		t.Fatalf("cookie=%q", req.Cookie)
	}
	if req.Headers["accept"] != "application/json" {
		t.Fatalf("accept=%q", req.Headers["accept"])
	}
	// 这些头应被过滤
	for _, h := range []string{"authority", "content-length", "user-agent"} {
		if _, ok := req.Headers[h]; ok && h != "user-agent" {
			t.Fatalf("头 %q 应被过滤", h)
		}
	}
}

func TestParseCurl_PostJsonBody(t *testing.T) {
	raw := `curl -X POST 'https://api.example.com/login' -H 'Content-Type: application/json' --data-raw '{"user":"admin","pwd":"123456"}'`
	req, err := ParseCurl(raw)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "POST" {
		t.Fatalf("method=%q 期望 POST", req.Method)
	}
	if !strings.Contains(req.Body, `"user":"admin"`) {
		t.Fatalf("body=%q", req.Body)
	}
}

func TestParseCurl_ImplicitPostByData(t *testing.T) {
	raw := `curl 'https://api.example.com/x' -d 'a=1' -d 'b=2'`
	req, err := ParseCurl(raw)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "POST" {
		t.Fatalf("method=%q 期望隐式 POST", req.Method)
	}
	if req.Body != "a=1&b=2" {
		t.Fatalf("body=%q 期望 a=1&b=2", req.Body)
	}
}

func TestParseCurl_CmdAndPowerShell(t *testing.T) {
	// cmd 的 ^ 续行 + 双引号
	cmd := `curl "https://api.example.com/a" -H "X-Token: abc^
def"`
	req, err := ParseCurl(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if req.URL != "https://api.example.com/a" {
		t.Fatalf("cmd url=%q", req.URL)
	}
	if req.Headers["X-Token"] != "abcdef" {
		t.Fatalf("cmd header=%q", req.Headers["X-Token"])
	}

	// PowerShell 的反引号续行
	ps := "curl.exe 'https://api.example.com/b' `\n  -H 'X-Test: ok'"
	req2, err := ParseCurl(ps)
	if err != nil {
		t.Fatal(err)
	}
	if req2.URL != "https://api.example.com/b" || req2.Headers["X-Test"] != "ok" {
		t.Fatalf("ps 解析失败: url=%q headers=%v", req2.URL, req2.Headers)
	}
}

func TestParseCurl_BasicAuth(t *testing.T) {
	raw := `curl -u admin:secret 'https://api.example.com/x'`
	req, err := ParseCurl(raw)
	if err != nil {
		t.Fatal(err)
	}
	if req.AuthType != "basic" || req.AuthUser != "admin" || req.AuthPass != "secret" {
		t.Fatalf("basic auth=%+v", req)
	}
}

func TestParseCurl_MethodCaseSensitivity(t *testing.T) {
	// -X POST 与 -x proxy 不能混淆
	raw := `curl -X PUT -x http://proxy:8080 'https://api.example.com/x'`
	req, err := ParseCurl(raw)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "PUT" {
		t.Fatalf("method=%q 期望 PUT（不应被 -x 代理覆盖）", req.Method)
	}
	if req.URL != "https://api.example.com/x" {
		t.Fatalf("url=%q 期望排除代理参数", req.URL)
	}
}

func TestToStep_BaseURLReplace(t *testing.T) {
	req, err := ParseCurl(`curl 'https://api.example.com/orders?page=1' -H 'X-A: 1'`)
	if err != nil {
		t.Fatal(err)
	}
	step := req.ToStep("查询订单", "https://api.example.com")
	if step.URL != "{{baseUrl}}/orders" {
		t.Fatalf("url=%q 期望 {{baseUrl}}/orders", step.URL)
	}
	if step.Query["page"] != "1" {
		t.Fatalf("query=%v", step.Query)
	}
	if step.Type != "request" || !step.Enabled || step.Name != "查询订单" {
		t.Fatalf("step=%+v", step)
	}
}

func TestParseCurl_UnclosedQuote(t *testing.T) {
	_, err := ParseCurl(`curl 'https://api.example.com/x`)
	if err == nil || !strings.Contains(err.Error(), "引号") {
		t.Fatalf("期望引号未闭合错误，得到 %v", err)
	}
}
