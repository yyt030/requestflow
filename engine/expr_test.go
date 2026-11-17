package engine

import (
	"testing"

	"requestflow/model"
)

func evalCtx(vars map[string]string) *Context { return NewContext(vars) }

// TestEval_LogicOperators：&&/|| 组合与短路求值。
func TestEval_LogicOperators(t *testing.T) {
	c := evalCtx(map[string]string{"a": "1", "b": "0", "s": "hello"})
	cases := []struct {
		expr string
		want bool
	}{
		{`1 == 1 && 2 == 2`, true},
		{`1 == 1 && 2 == 3`, false},
		{`1 == 3 || 2 == 2`, true},
		{`1 == 3 || 2 == 4`, false},
		{`{{a}} == 1 && {{b}} == 0`, true},
		{`{{s}} == "hello"`, true},
		{`contains({{s}}, "ell") && {{a}} == 1`, true},
		{`1 == 1 && 2 == 2 && 3 == 3`, true},
		{`0 || 0 || 5`, true},
	}
	for _, tc := range cases {
		got := truthy(c.Eval(tc.expr))
		if got != tc.want {
			t.Errorf("Eval(%q) truthy=%v want %v (raw=%v)", tc.expr, got, tc.want, c.Eval(tc.expr))
		}
	}
}

// TestEval_LogicRespectsQuotes：引号内的 && / || 不应被当作逻辑运算符切分。
func TestEval_LogicRespectsQuotes(t *testing.T) {
	c := evalCtx(map[string]string{"x": "a&&b"})
	if got := toStr(c.Eval(`{{x}}`)); got != "a&&b" {
		t.Errorf("变量中的 && 被误解析: %q", got)
	}
	if got := truthy(c.Eval(`"a&&b" == "a&&b"`)); !got {
		t.Error("引号内含 && 的字符串相等比较应为真")
	}
	if got := truthy(c.Eval(`"x||y" == "x||y"`)); !got {
		t.Error("引号内含 || 的字符串相等比较应为真")
	}
}

// TestEval_Truthy：truthy 对各类型的判定。
func TestEval_Truthy(t *testing.T) {
	cases := []struct {
		v    interface{}
		want bool
	}{
		{nil, false}, {false, false}, {true, true},
		{0, false}, {1, true}, {"", false}, {"false", false}, {"0", false},
		{"hello", true}, {[]interface{}{}, false}, {[]interface{}{1}, true},
	}
	for _, tc := range cases {
		if got := truthy(tc.v); got != tc.want {
			t.Errorf("truthy(%v)=%v want %v", tc.v, got, tc.want)
		}
	}
}

// TestEval_WhitespaceLogic：逻辑运算符两侧空白与换行不影响解析。
func TestEval_WhitespaceLogic(t *testing.T) {
	c := evalCtx(map[string]string{})
	if !truthy(c.Eval("1 == 1  &&\n 2 == 2")) {
		t.Error("带空白/换行的 && 应正确求值")
	}
}

// TestMerge_ScenarioShape 用一个内联场景确认 merge 步骤类型被正确调度（轻量补充）。
func TestMerge_ScenarioShape(t *testing.T) {
	sc := &model.Scenario{
		Name: "shape",
		Variables: map[string]string{"flag": "1"},
		Steps: []model.Step{
			{ID: "m", Name: "gate", Type: model.StepMerge, Enabled: true,
				Merge: &model.Merge{When: `{{flag}} == 1`}},
		},
	}
	res := NewEngine(sc, nil).Run()
	if !res.OK {
		t.Fatalf("merge 门控应通过: %v", res.Err)
	}
}
