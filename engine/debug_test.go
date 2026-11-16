package engine

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"requestflow/model"
)

func TestDebug_BasicRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("X-Token"); got != "abc" {
			t.Errorf("X-Token = %q, want abc", got)
		}
		if got := r.URL.Query().Get("page"); got != "2" {
			t.Errorf("query page = %q, want 2", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"name":"alice","age":30}`))
	}))
	defer srv.Close()

	res := Debug(DebugRequest{
		Method:  "POST",
		URL:     srv.URL + "/users",
		Headers: map[string]string{"X-Token": "abc"},
		Query:   map[string]string{"page": "2"},
		Body:    `{"a":1}`,
	}, nil)

	if !res.OK {
		t.Fatalf("Debug failed: %s", res.Err)
	}
	if res.StatusCode != 201 {
		t.Errorf("status = %d, want 201", res.StatusCode)
	}
	if res.StatusText != "Created" {
		t.Errorf("statusText = %q, want Created", res.StatusText)
	}
	if !res.IsJSON {
		t.Error("response should be detected as JSON")
	}
	if !strings.Contains(res.Formatted, "\n  \"name\"") {
		t.Errorf("response not pretty-printed: %q", res.Formatted)
	}
	if res.Size != len(`{"name":"alice","age":30}`) {
		t.Errorf("size = %d", res.Size)
	}
	if len(res.Headers) == 0 {
		t.Error("expected response headers")
	}
	if !strings.Contains(res.Curl, "-X 'POST'") || !strings.Contains(res.Curl, "page=2") {
		t.Errorf("curl preview incomplete: %s", res.Curl)
	}
}

func TestDebug_VariableRendering(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/orders" {
			t.Errorf("path = %q, want /orders", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token123" {
			t.Errorf("auth = %q, want Bearer token123", got)
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	vars := map[string]string{"baseUrl": srv.URL, "token": "token123"}
	res := Debug(DebugRequest{
		Method:    "GET",
		URL:       "{{baseUrl}}/orders",
		AuthType:  "bearer",
		AuthToken: "{{token}}",
	}, vars)

	if !res.OK {
		t.Fatalf("Debug failed: %s", res.Err)
	}
	if res.URL != srv.URL+"/orders" {
		t.Errorf("rendered URL = %q", res.URL)
	}
	if res.IsJSON {
		t.Error("plain text should not be marked as JSON")
	}
	if res.Formatted != "ok" {
		t.Errorf("formatted = %q, want ok", res.Formatted)
	}
}

func TestDebugStep_AndCurlRoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	step := &model.Step{
		Method:  "GET",
		URL:     srv.URL + "/ping",
		Headers: map[string]string{"X-Test": "yes"},
	}
	res := DebugStep(step, nil)
	if !res.OK {
		t.Fatalf("DebugStep failed: %s", res.Err)
	}

	// 反向生成的 cURL 应能被解析器重新解析（往返一致）。
	parsed, err := ParseCurl(res.Curl)
	if err != nil {
		t.Fatalf("ParseCurl on preview failed: %v\ncurl=%s", err, res.Curl)
	}
	if parsed.Method != "GET" {
		t.Errorf("roundtrip method = %q, want GET", parsed.Method)
	}
	if !strings.Contains(parsed.URL, "/ping") {
		t.Errorf("roundtrip URL = %q", parsed.URL)
	}
}

func TestDebug_ErrorNoServer(t *testing.T) {
	res := Debug(DebugRequest{
		Method:     "GET",
		URL:        "http://127.0.0.1:1/nope",
		TimeoutSec: 1,
	}, nil)
	if res.OK {
		t.Error("expected failure for unreachable server")
	}
	if res.Err == "" {
		t.Error("expected error message")
	}
}
