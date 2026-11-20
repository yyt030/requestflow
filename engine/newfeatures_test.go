package engine

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"requestflow/model"
)

// TestEnvVariablesOverride 验证环境变量覆盖全局变量，且不影响原始数据。
func TestEnvVariablesOverride(t *testing.T) {
	sc := &model.Scenario{
		Name:      "env test",
		Variables: map[string]string{"host": "global.example.com", "token": "global-token"},
		Environments: map[string]map[string]string{
			"dev":  {"host": "dev.example.com"},
			"prod": {"host": "prod.example.com", "token": "prod-token"},
		},
		ActiveEnv: "prod",
		Steps:     []model.Step{},
	}
	merged := mergeEnvVariables(sc)
	if merged["host"] != "prod.example.com" {
		t.Fatalf("host: 期望 prod.example.com，得到 %s", merged["host"])
	}
	if merged["token"] != "prod-token" {
		t.Fatalf("token: 期望 prod-token，得到 %s", merged["token"])
	}
	// 原始数据未被修改
	if sc.Variables["host"] != "global.example.com" {
		t.Fatalf("原始全局变量被修改: %s", sc.Variables["host"])
	}
	// 无激活环境时回退全局变量
	sc.ActiveEnv = ""
	merged2 := mergeEnvVariables(sc)
	if merged2["host"] != "global.example.com" {
		t.Fatalf("无环境时应回退全局变量，得到 %s", merged2["host"])
	}
}

// TestPrePostScript 验证请求级 Pre-request 与 Tests 脚本执行顺序与变量传递。
func TestPrePostScript(t *testing.T) {
	var gotPreHeader, gotPostHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPreHeader = r.Header.Get("X-Pre")
		gotPostHeader = r.Header.Get("X-Post-From-Server")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data":{"id":42}}`))
	}))
	defer srv.Close()

	sc := &model.Scenario{
		Name: "pre/post test",
		Steps: []model.Step{
			{
				ID: "1", Name: "带脚本请求", Type: model.StepRequest, Enabled: true,
				Method: "GET", URL: srv.URL,
				ScriptLang: "js",
				Headers:    map[string]string{"X-Pre": "{{preValue}}"},
				PreScript:  `vars.preValue = "injected-by-pre";`,
				PostScript: `vars.savedId = last.json.data.id; log("post ran");`,
			},
		},
	}
	res := NewEngine(sc, nil).Run()
	if !res.OK {
		t.Fatalf("场景执行失败: %v", res.Err)
	}
	if gotPreHeader != "injected-by-pre" {
		t.Fatalf("PreScript 未影响请求头，得到 %q", gotPreHeader)
	}
	if toStr(res.Vars["savedId"]) != "42" {
		t.Fatalf("PostScript 未写回变量，得到 %v", res.Vars["savedId"])
	}
	_ = gotPostHeader
}

// TestPrePostScriptPython 验证 Python(Starlark) 语言的请求级脚本。
func TestPrePostScriptPython(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	sc := &model.Scenario{
		Name: "py pre/post",
		Steps: []model.Step{
			{
				ID: "1", Name: "py脚本", Type: model.StepRequest, Enabled: true,
				Method: "GET", URL: srv.URL, ScriptLang: "python",
				PreScript:  `vars["pyvar"] = "pre-ok"`,
				PostScript: `vars["pyresult"] = last["status"]`,
			},
		},
	}
	res := NewEngine(sc, nil).Run()
	if !res.OK {
		t.Fatalf("执行失败: %v", res.Err)
	}
	if res.Vars["pyvar"] != "pre-ok" {
		t.Fatalf("py PreScript 未生效: %v", res.Vars["pyvar"])
	}
	if toStr(res.Vars["pyresult"]) != "200" {
		t.Fatalf("py PostScript 未生效: %v", res.Vars["pyresult"])
	}
}
