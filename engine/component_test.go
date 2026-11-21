package engine

import (
	"fmt"
	"testing"

	"requestflow/model"
)

// TestSwitch_FirstMatchWins：多个 case 都满足时只执行第一个，并跳出 switch。
func TestSwitch_FirstMatchWins(t *testing.T) {
	mkScript := func(name, val string) model.Step {
		return model.Step{ID: name, Name: name, Type: model.StepScript, Enabled: true,
			Code: fmt.Sprintf(`vars.%s = '%s';`, name, val)}
	}
	sc := &model.Scenario{
		Name:      "switch 首个命中",
		Variables: map[string]string{"score": "85"},
		Steps: []model.Step{{
			ID: "sw", Name: "switch", Type: model.StepSwitch, Enabled: true,
			Switch: &model.Switch{
				Cases: []model.SwitchCase{
					{Name: "优秀", Conditions: []model.Condition{{Var: "score", Op: "gte", Value: "80"}},
						Steps: []model.Step{mkScript("grade", "A")}},
					{Name: "及格", Conditions: []model.Condition{{Var: "score", Op: "gte", Value: "60"}},
						Steps: []model.Step{mkScript("grade", "B")}},
				},
				Default: []model.Step{mkScript("grade", "C")},
			},
		}},
	}
	res := NewEngine(sc, nil).Run()
	if !res.OK {
		t.Fatalf("run failed: %v", res.Err)
	}
	if got := toStr(res.Vars["grade"]); got != "A" {
		t.Errorf("grade = %q, want A (首个命中)", got)
	}
}

// TestSwitch_Default：都不命中时执行 default。

// TestSwitch_Default：都不命中时执行 default。
func TestSwitch_Default(t *testing.T) {
	sc := &model.Scenario{
		Name:      "switch default",
		Variables: map[string]string{"score": "30"},
		Steps: []model.Step{{
			ID: "sw", Name: "switch", Type: model.StepSwitch, Enabled: true,
			Switch: &model.Switch{
				Cases: []model.SwitchCase{
					{Name: "优秀", Conditions: []model.Condition{{Var: "score", Op: "gte", Value: "80"}},
						Steps: []model.Step{{ID: "a", Name: "a", Type: model.StepScript, Enabled: true, Code: `vars.grade = 'A';`}}},
				},
				Default: []model.Step{{ID: "d", Name: "d", Type: model.StepScript, Enabled: true, Code: `vars.grade = 'C';`}},
			},
		}},
	}
	res := NewEngine(sc, nil).Run()
	if !res.OK {
		t.Fatalf("run failed: %v", res.Err)
	}
	if got := toStr(res.Vars["grade"]); got != "C" {
		t.Errorf("grade = %q, want C (default)", got)
	}
}

// TestSwitch_WhenExpr：case 支持 When 原始表达式与中文操作符。

// TestSwitch_WhenExpr：case 支持 When 原始表达式与中文操作符。
func TestSwitch_WhenExpr(t *testing.T) {
	sc := &model.Scenario{
		Name: "switch when",
		Steps: []model.Step{
			{ID: "set", Name: "set", Type: model.StepProcess, Enabled: true, Op: "set",
				Args: map[string]string{"name": "amount", "value": "120"}},
			{ID: "sw", Name: "switch", Type: model.StepSwitch, Enabled: true,
				Switch: &model.Switch{
					Cases: []model.SwitchCase{
						{Name: "正", Conditions: []model.Condition{{Var: "amount", Op: "大于", Value: "0"}},
							Steps: []model.Step{{ID: "p", Name: "p", Type: model.StepScript, Enabled: true, Code: `vars.sign = 'positive';`}}},
					},
					Default: []model.Step{{ID: "n", Name: "n", Type: model.StepScript, Enabled: true, Code: `vars.sign = 'negative';`}},
				}},
		},
	}
	res := NewEngine(sc, nil).Run()
	if !res.OK {
		t.Fatalf("run failed: %v", res.Err)
	}
	if got := toStr(res.Vars["sign"]); got != "positive" {
		t.Errorf("sign = %q, want positive", got)
	}
}

// TestSwitch_OrLogic：case 内多条件 or 组合。

// TestSwitch_OrLogic：case 内多条件 or 组合。
func TestSwitch_OrLogic(t *testing.T) {
	sc := &model.Scenario{
		Name:      "switch or",
		Variables: map[string]string{"status": "500"},
		Steps: []model.Step{{
			ID: "sw", Name: "switch", Type: model.StepSwitch, Enabled: true,
			Switch: &model.Switch{
				Cases: []model.SwitchCase{{
					Name: "异常", Logic: "or",
					Conditions: []model.Condition{
						{Var: "status", Op: "eq", Value: "404"},
						{Var: "status", Op: "eq", Value: "500"},
					},
					Steps: []model.Step{{ID: "e", Name: "e", Type: model.StepScript, Enabled: true, Code: `vars.result = 'error';`}},
				}},
			},
		}},
	}
	res := NewEngine(sc, nil).Run()
	if !res.OK {
		t.Fatalf("run failed: %v", res.Err)
	}
	if got := toStr(res.Vars["result"]); got != "error" {
		t.Errorf("result = %q, want error", got)
	}
}

// ---------- parallel ----------

// parallelHTTPServer 返回按路径区分的 JSON，用于验证并行请求。
