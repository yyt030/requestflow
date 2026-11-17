package engine

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"requestflow/model"
)

// regressionServer 返回自测用本地服务，覆盖回归场景涉及的所有接口。
// /orders 会校验 Bearer token，未携带则 401，用于验证步骤间的变量传递与鉴权头注入。
func regressionServer() *httptest.Server {
	const token = "rf-test-token-abc123"
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		switch r.URL.Path {
		case "/login":
			fmt.Fprint(w, `{"code":0,"token":"`+token+`","msg":"login ok"}`)
		case "/orders":
			if !strings.Contains(r.Header.Get("Authorization"), token) {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, `{"code":401,"msg":"unauthorized"}`)
				return
			}
			fmt.Fprint(w, `{"code":0,"orders":[{"id":1,"amount":120,"name":"订单A"},{"id":2,"amount":-35,"name":"订单B"},{"id":3,"amount":88,"name":"订单C"}]}`)
		case "/detail":
			fmt.Fprint(w, `{"page":"detail","balance":"`+r.URL.Query().Get("balance")+`"}`)
		case "/warning":
			fmt.Fprint(w, `{"page":"warning","balance":"`+r.URL.Query().Get("balance")+`"}`)
		case "/page":
			p := r.URL.Query().Get("page")
			fmt.Fprintf(w, `{"page":%s,"rows":["row-%s-0","row-%s-1"]}`, p, p, p)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"error":"not found"}`)
		}
	}))
}

// loadRegressionScenario 读取 testdata 场景，把 {{BASE}}/{{OUT}} 占位符替换为实际值后解析。
// 这条路径与 GUI/CLI 打开场景 JSON 的方式一致（os.ReadFile + json.Unmarshal）。
func loadRegressionScenario(t *testing.T, base, out string) *model.Scenario {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "regression-scenario.json"))
	if err != nil {
		t.Fatalf("读取回归场景失败: %v", err)
	}
	s := strings.ReplaceAll(string(data), "{{BASE}}", base)
	s = strings.ReplaceAll(s, "{{OUT}}", out)
	var sc model.Scenario
	if err := json.Unmarshal([]byte(s), &sc); err != nil {
		t.Fatalf("解析回归场景失败: %v", err)
	}
	return &sc
}

// fmtVar 把结果变量转成可比较的字符串。
func fmtVar(v interface{}) string { return fmt.Sprint(v) }

// TestRegression_FromJSON 端到端回归：从 JSON 加载场景并运行，
// 覆盖登录提取 → 鉴权请求 → 断言 → 条件分支(then) → 循环翻页 → JS 脚本 → 导出。
func TestRegression_FromJSON(t *testing.T) {
	srv := regressionServer()
	defer srv.Close()

	out := filepath.Join(t.TempDir(), "selftest-result.txt")
	sc := loadRegressionScenario(t, srv.URL, out)

	res := NewEngine(sc, nil).Run()
	if !res.OK {
		t.Fatalf("回归场景执行失败: %v", res.Err)
	}

	// 步骤间传递与提取
	if fmtVar(res.Vars["token"]) != "rf-test-token-abc123" {
		t.Errorf("token = %v, 期望登录提取的 token", res.Vars["token"])
	}
	if fmtVar(res.Vars["firstAmount"]) != "120" {
		t.Errorf("firstAmount = %v, 期望 120", res.Vars["firstAmount"])
	}
	if fmtVar(res.Vars["orderCount"]) != "3" {
		t.Errorf("orderCount = %v, 期望 3（gjson 数组长度 orders.#）", res.Vars["orderCount"])
	}

	// 条件分支走了 then（金额为正 → 详情页）
	if fmtVar(res.Vars["resultPage"]) != "detail" {
		t.Errorf("resultPage = %v, 期望 detail（应走 then 分支）", res.Vars["resultPage"])
	}

	// 循环翻页：page 递增到 3，脚本累积三页数据
	if fmtVar(res.Vars["page"]) != "3" {
		t.Errorf("page = %v, 期望循环结束为 3", res.Vars["page"])
	}
	got := fmtVar(res.Vars["allText"])
	for _, want := range []string{"row-1-0", "row-2-0", "row-3-0", "row-3-1"} {
		if !strings.Contains(got, want) {
			t.Errorf("allText 缺少 %q: %s", want, got)
		}
	}

	// 导出文件内容
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("读取导出文件失败: %v", err)
	}
	for _, want := range []string{"row-1-0", "row-2-0", "row-3-0"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("导出文件缺少 %q: %s", want, b)
		}
	}
}

// TestRegression_BranchElsePath 验证条件分支的另一侧：金额为负时走 else（告警页）。
// 直接构造场景，把 firstAmount 设为 -35，条件 gt 0 不成立。
func TestRegression_BranchElsePath(t *testing.T) {
	srv := regressionServer()
	defer srv.Close()

	setVar := enabledStep("v", "设负数", model.StepProcess)
	setVar.Op = "set"
	setVar.Args = map[string]string{"name": "firstAmount", "value": "-35"}

	br := enabledStep("b", "金额分支", model.StepBranch)
	br.Branch = &model.Branch{
		Logic:      "and",
		Conditions: []model.Condition{{Var: "firstAmount", Op: "gt", Value: "0"}},
		Then: []model.Step{func() model.Step {
			s := enabledStep("t", "详情", model.StepRequest)
			s.URL = srv.URL + "/detail?balance={{firstAmount}}"
			s.Extract = []model.Extract{{Name: "resultPage", Source: "body", Kind: "jsonpath", Path: "page"}}
			return s
		}()},
		Else: []model.Step{func() model.Step {
			s := enabledStep("e", "告警", model.StepRequest)
			s.URL = srv.URL + "/warning?balance={{firstAmount}}"
			s.Extract = []model.Extract{{Name: "resultPage", Source: "body", Kind: "jsonpath", Path: "page"}}
			return s
		}()},
	}

	res := NewEngine(&model.Scenario{Name: "branch else", Steps: []model.Step{setVar, br}}, nil).Run()
	if !res.OK {
		t.Fatalf("分支场景执行失败: %v", res.Err)
	}
	if got := fmtVar(res.Vars["resultPage"]); got != "warning" {
		t.Errorf("resultPage = %v, 期望 warning（应走 else 分支）", got)
	}
}

// TestRegression_PythonScript 端到端回归：从 JSON 加载含 Python 脚本步骤的场景，
// 验证脚本写回的变量能被后续步骤的 {{变量}} 渲染与断言使用。
func TestRegression_PythonScript(t *testing.T) {
	srv := regressionServer()
	defer srv.Close()

	data, err := os.ReadFile(filepath.Join("testdata", "regression-python-scenario.json"))
	if err != nil {
		t.Fatalf("读取 Python 回归场景失败: %v", err)
	}
	s := strings.ReplaceAll(string(data), "{{BASE}}", srv.URL)
	var sc model.Scenario
	if err := json.Unmarshal([]byte(s), &sc); err != nil {
		t.Fatalf("解析 Python 回归场景失败: %v", err)
	}

	res := NewEngine(&sc, nil).Run()
	if !res.OK {
		t.Fatalf("Python 回归场景执行失败: %v", res.Err)
	}
	if got := fmtVar(res.Vars["page_num"]); got != "70" {
		t.Errorf("page_num = %v, 期望 70（脚本 int(page)*10）", res.Vars["page_num"])
	}
	if got := fmtVar(res.Vars["names"]); !strings.Contains(got, "A") || !strings.Contains(got, "B") {
		t.Errorf("names = %v, 期望含大写 A/B", res.Vars["names"])
	}
	if got := fmtVar(res.Vars["joined"]); got != "A,B,C" {
		t.Errorf("joined = %v, 期望 A,B,C", res.Vars["joined"])
	}
}
