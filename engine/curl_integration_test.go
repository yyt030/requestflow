package engine

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"requestflow/model"
)

// 验证从 cURL 导入的步骤能被引擎真实执行（url/query/header/cookie/body 正确送达）。
func TestCurlImportedStep_Executes(t *testing.T) {
	var gotMethod, gotQuery, gotHeader, gotCookie, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotQuery = r.URL.Query().Get("page")
		gotHeader = r.Header.Get("X-Custom")
		gotCookie = r.Header.Get("Cookie")
		buf := make([]byte, r.ContentLength)
		if r.ContentLength > 0 {
			r.Body.Read(buf)
		}
		gotBody = string(buf)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	raw := "curl -X POST '" + srv.URL + "/items?page=7' \\\n" +
		"  -H 'X-Custom: hello' \\\n" +
		"  -H 'Cookie: sid=xyz' \\\n" +
		"  -H 'Content-Type: application/json' \\\n" +
		"  --data-raw '{\"name\":\"test\"}'"

	req, err := ParseCurl(raw)
	if err != nil {
		t.Fatal(err)
	}
	step := req.ToStep("导入的用例", "")

	sc := &model.Scenario{Name: "curl import test", Steps: []model.Step{step}}
	res := NewEngine(sc, nil).Run()
	if !res.OK {
		t.Fatalf("执行失败: %v", res.Err)
	}
	if gotMethod != "POST" || gotQuery != "7" || gotHeader != "hello" || gotCookie != "sid=xyz" {
		t.Fatalf("请求未正确送达: method=%q query=%q header=%q cookie=%q", gotMethod, gotQuery, gotHeader, gotCookie)
	}
	if gotBody != `{"name":"test"}` {
		t.Fatalf("body=%q", gotBody)
	}
}
