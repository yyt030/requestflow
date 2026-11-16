package engine

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"requestflow/model"
)

var branchServers = map[*model.Scenario]*httptest.Server{}

// 构造"根据金额正负请求不同页面"的场景，金额由第一个请求提取。
func branchScenario(amount float64) (*model.Scenario, *[]string) {
	visited := &[]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/amount" {
			*visited = append(*visited, "amount")
			fmt.Fprintf(w, `{"amount":%v}`, amount)
			return
		}
		page := r.URL.Query().Get("page")
		*visited = append(*visited, page)
		fmt.Fprintf(w, `{"page":%q}`, page)
	}))
	sc := &model.Scenario{
		Name:    "分支测试",
		BaseURL: srv.URL,
		Steps: []model.Step{
			{
				ID: "1", Name: "取金额", Type: model.StepRequest, Enabled: true,
				Method: "GET", URL: "/amount",
				Extract: []model.Extract{{Name: "amount", Source: "body", Kind: "jsonpath", Path: "amount"}},
			},
			{
				ID: "2", Name: "按金额分支", Type: model.StepBranch, Enabled: true,
				Branch: &model.Branch{
					Conditions: []model.Condition{{Var: "amount", Op: "gt", Value: "0"}},
					Then: []model.Step{{
						ID: "2a", Name: "请求页面1", Type: model.StepRequest, Enabled: true,
						Method: "GET", URL: "/page?page=1",
					}},
					Else: []model.Step{{
						ID: "2b", Name: "请求页面2", Type: model.StepRequest, Enabled: true,
						Method: "GET", URL: "/page?page=2",
					}},
				},
			},
		},
	}
	branchServers[sc] = srv
	return sc, visited
}

func TestBranch_PositiveGoesThen(t *testing.T) {
	sc, visited := branchScenario(100)
	defer branchServers[sc].Close()
	res := NewEngine(sc, nil).Run()
	if !res.OK {
		t.Fatalf("run failed: %v", res.Err)
	}
	assertVisited(t, *visited, "amount", "1")
}

func TestBranch_NegativeGoesElse(t *testing.T) {
	sc, visited := branchScenario(-50)
	defer branchServers[sc].Close()
	res := NewEngine(sc, nil).Run()
	if !res.OK {
		t.Fatalf("run failed: %v", res.Err)
	}
	assertVisited(t, *visited, "amount", "2")
}

func TestBranch_WhenExpression(t *testing.T) {
	sc := &model.Scenario{
		Name: "when 表达式",
		Steps: []model.Step{
			{ID: "s", Name: "set", Type: model.StepProcess, Enabled: true, Op: "set",
				Args: map[string]string{"name": "score", "value": "88"}},
			{ID: "b", Name: "branch", Type: model.StepBranch, Enabled: true,
				Branch: &model.Branch{
					When: "{{score}} >= 60",
					Then: []model.Step{{ID: "t", Name: "then", Type: model.StepScript, Enabled: true,
						Code: `vars.branch = 'then';`}},
					Else: []model.Step{{ID: "e", Name: "else", Type: model.StepScript, Enabled: true,
						Code: `vars.branch = 'else';`}},
				}},
		},
	}
	res := NewEngine(sc, nil).Run()
	if !res.OK {
		t.Fatalf("run failed: %v", res.Err)
	}
	if got := toStr(res.Vars["branch"]); got != "then" {
		t.Errorf("branch = %q, want then", got)
	}
}

func TestBranch_LogicAndOr(t *testing.T) {
	sc := &model.Scenario{
		Name:      "逻辑组合",
		Variables: map[string]string{"a": "1", "b": "0"},
		Steps: []model.Step{
			{ID: "b", Name: "branch", Type: model.StepBranch, Enabled: true,
				Branch: &model.Branch{
					Logic: "and",
					Conditions: []model.Condition{
						{Var: "a", Op: "eq", Value: "1"},
						{Var: "b", Op: "eq", Value: "0"},
					},
					Then: []model.Step{{ID: "t", Name: "then", Type: model.StepScript, Enabled: true,
						Code: `vars.result = 'and-ok';`}},
				}},
		},
	}
	res := NewEngine(sc, nil).Run()
	if !res.OK {
		t.Fatalf("run failed: %v", res.Err)
	}
	if got := toStr(res.Vars["result"]); got != "and-ok" {
		t.Errorf("result = %q, want and-ok", got)
	}
}

func TestBranch_Nested(t *testing.T) {
	sc := &model.Scenario{
		Name:      "嵌套分支",
		Variables: map[string]string{"x": "yes"},
		Steps: []model.Step{
			{ID: "outer", Name: "outer", Type: model.StepBranch, Enabled: true,
				Branch: &model.Branch{
					Conditions: []model.Condition{{Var: "x", Op: "eq", Value: "yes"}},
					Then: []model.Step{
						{ID: "inner", Name: "inner", Type: model.StepBranch, Enabled: true,
							Branch: &model.Branch{
								Conditions: []model.Condition{{Var: "x", Op: "contains", Value: "ye"}},
								Then: []model.Step{{ID: "deep", Name: "deep", Type: model.StepScript, Enabled: true,
									Code: `vars.deep = 'reached';`}},
							}},
					},
				}},
		},
	}
	res := NewEngine(sc, nil).Run()
	if !res.OK {
		t.Fatalf("run failed: %v", res.Err)
	}
	if got := toStr(res.Vars["deep"]); got != "reached" {
		t.Errorf("deep = %q, want reached", got)
	}
}

func assertVisited(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("visited = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("visited = %v, want %v", got, want)
		}
	}
}
