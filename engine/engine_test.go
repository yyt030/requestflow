package engine

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"requestflow/model"
)

func enabledStep(id, name string, typ model.StepType) model.Step {
	return model.Step{ID: id, Name: name, Type: typ, Enabled: true}
}

// TestLoginTokenExtractRequest 验证：登录 -> 提取 token -> bearer 携带 -> 断言。
func TestLoginTokenExtractRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"code":0,"data":{"token":"abc123"}}`)
		case "/profile":
			if r.Header.Get("Authorization") != "Bearer abc123" {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, `{"error":"no token"}`)
				return
			}
			fmt.Fprint(w, `{"name":"Ada","age":36}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	login := enabledStep("1", "登录", model.StepRequest)
	login.Method, login.URL = "POST", srv.URL+"/login"
	login.Extract = []model.Extract{{Name: "token", Source: "body", Kind: "jsonpath", Path: "data.token"}}
	login.Assert = []model.Assert{{Source: "status", Op: "eq", Value: "200"}}

	profile := enabledStep("2", "资料", model.StepRequest)
	profile.Method, profile.URL = "GET", srv.URL+"/profile"
	profile.AuthType, profile.AuthToken = "bearer", "{{token}}"
	profile.Extract = []model.Extract{{Name: "age", Source: "body", Kind: "jsonpath", Path: "age"}}
	profile.Assert = []model.Assert{{Source: "jsonpath", Path: "name", Op: "eq", Value: "Ada"}}

	sc := &model.Scenario{Name: "token test", Steps: []model.Step{login, profile}}
	res := NewEngine(sc, nil).Run()
	if !res.OK {
		t.Fatalf("执行失败: %v", res.Err)
	}
	if got := res.Vars["token"]; got != "abc123" {
		t.Fatalf("token = %v, 期望 abc123", got)
	}
	if fmt.Sprint(res.Vars["age"]) != "36" {
		t.Fatalf("age = %v", res.Vars["age"])
	}
}

// TestCookieSession 验证登录设置的 cookie 在后续请求自动携带。
func TestCookieSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "xyz", Path: "/"})
			fmt.Fprint(w, `{"ok":true}`)
			return
		}
		c, err := r.Cookie("sid")
		if err != nil || c.Value != "xyz" {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{"ok":false}`)
			return
		}
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer srv.Close()

	login := enabledStep("1", "登录", model.StepRequest)
	login.Method, login.URL = "POST", srv.URL+"/login"
	check := enabledStep("2", "校验cookie", model.StepRequest)
	check.URL = srv.URL + "/home"
	check.Assert = []model.Assert{{Source: "body", Op: "contains", Value: `"ok":true`}}

	sc := &model.Scenario{Name: "cookie test", Steps: []model.Step{login, check}}
	res := NewEngine(sc, nil).Run()
	if !res.OK {
		t.Fatalf("cookie 会话失败: %v", res.Err)
	}
}

// TestLoopPagination 验证 until 模式翻页：page 变量递增，翻到空数组时终止。
func TestLoopPagination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		switch page {
		case "1":
			fmt.Fprint(w, `{"rows":[{"id":1,"amount":10},{"id":2,"amount":20}]}`)
		case "2":
			fmt.Fprint(w, `{"rows":[{"id":3,"amount":30}]}`)
		default:
			fmt.Fprint(w, `{"rows":[]}`)
		}
	}))
	defer srv.Close()

	req := enabledStep("r", "拉取", model.StepRequest)
	req.Method, req.URL = "GET", srv.URL+"/list"
	req.Query = map[string]string{"page": "{{page}}"}
	req.Extract = []model.Extract{{Name: "rows", Source: "body", Kind: "jsonpath", Path: "rows"}}

	loop := enabledStep("l", "翻页", model.StepLoop)
	loop.Loop = &model.Loop{
		Mode: "until", BreakWhen: "{{len(rows)}} == 0",
		PageVar: "page", PageStart: 1, PageStep: 1, Max: 10,
	}
	loop.Children = []model.Step{req}

	flush := enabledStep("f", "合并", model.StepProcess)
	flush.Op = "set"
	flush.Args = map[string]string{"name": "total_pages_hit", "value": "{{page}}"}

	sc := &model.Scenario{Name: "pagination", Steps: []model.Step{loop, flush}}
	res := NewEngine(sc, nil).Run()
	if !res.OK {
		t.Fatalf("翻页执行失败: %v", res.Err)
	}
	// page=1 有数据, page=2 有数据, page=3 空→退出；终止后 page 停在 3
	if fmt.Sprint(res.Vars["page"]) != "3" {
		t.Fatalf("page = %v, 期望 3", res.Vars["page"])
	}
}

// TestScriptAndExport 验证自定义 JS 脚本写变量 + 导出 Excel。
func TestScriptAndExport(t *testing.T) {
	ctx := NewContext(map[string]string{"raw": `["a","b","c"]`})
	step := enabledStep("s", "脚本", model.StepScript)
	step.Code = `
		var arr = JSON.parse(vars.raw);
		vars.upper = arr.map(function(x){ return x.toUpperCase(); }).join(",");
	`

	if err := runScript(ctx, step, nil); err != nil {
		t.Fatalf("脚本失败: %v", err)
	}
	if ctx.Get("upper") != "A,B,C" {
		t.Fatalf("upper = %v", ctx.Get("upper"))
	}

	ctx.Set("rows", []interface{}{
		map[string]interface{}{"name": "Ada", "score": 90.0},
		map[string]interface{}{"name": "Bob", "score": 80.0},
	})
	dir := t.TempDir()
	path := filepath.Join(dir, "out.xlsx")
	got, err := runExport(ctx, "rows", "xlsx", path)
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	info, _ := os.Stat(got)
	if info == nil || info.Size() == 0 {
		t.Fatal("导出的文件为空")
	}
	txtPath := filepath.Join(dir, "out.txt")
	if _, err := runExport(ctx, "rows", "txt", txtPath); err != nil {
		t.Fatalf("txt 导出失败: %v", err)
	}
	data, _ := os.ReadFile(txtPath)
	if !strings.Contains(string(data), "Ada") {
		t.Fatalf("txt 内容异常: %s", data)
	}
}

// TestContextExpressions 验证模板与表达式求值。
func TestContextExpressions(t *testing.T) {
	ctx := NewContext(map[string]string{
		"n":    "5",
		"obj":  `{"a":{"b":"hello"}}`,
		"list": `["x","y","z"]`,
	})
	cases := []struct {
		expr string
		want string
	}{
		{"{{n}}", "5"},
		{"{{add(n, 3)}}", "8"},
		{"{{obj.a.b}}", "hello"},
		{"{{len(list)}}", "3"},
		{"{{upper(obj.a.b)}}", "HELLO"},
		{"{{n}} 加 {{add(n,1)}}", "5 加 6"},
	}
	for _, c := range cases {
		if got := ctx.Render(c.expr); got != c.want {
			t.Errorf("Render(%q) = %q, 期望 %q", c.expr, got, c.want)
		}
	}
	if ctx.Eval("{{len(list)}}").(int) != 3 {
		t.Error("Eval len 失败")
	}
	_ = strconv.Itoa
}
