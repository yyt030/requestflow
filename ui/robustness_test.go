package ui

import (
	"strings"
	"sync"
	"testing"

	"requestflow/engine"
	"requestflow/model"
)

// ---------- 步骤列表操作：索引越界不应 panic ----------

func TestDeleteStepOutOfRange(t *testing.T) {
	a := &App{scenario: model.NewScenario("s"), selectedStep: -1}
	a.appendStep(model.StepRequest)
	// 越界索引：负数与超长都应被安全忽略，不 panic
	for _, idx := range []int{-1, 5, 100} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("deleteStep(%d) panic: %v", idx, r)
				}
			}()
			a.deleteStep(idx)
		}()
	}
	if len(a.scenario.Steps) != 1 {
		t.Fatalf("越界删除不应改变步骤数, got %d", len(a.scenario.Steps))
	}
}

func TestMoveStepOutOfRange(t *testing.T) {
	a := &App{scenario: model.NewScenario("s"), selectedStep: -1}
	a.appendStep(model.StepRequest)
	for _, c := range []struct{ i, dir int }{{-1, 1}, {0, -1}, {0, 5}, {100, 1}} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("moveStep(%d,%d) panic: %v", c.i, c.dir, r)
				}
			}()
			a.moveStep(c.i, c.dir)
		}()
	}
	if len(a.scenario.Steps) != 1 {
		t.Fatalf("越界移动不应改变步骤数, got %d", len(a.scenario.Steps))
	}
}

func TestDeleteStepKeepsSelectionValid(t *testing.T) {
	a := &App{scenario: model.NewScenario("s"), selectedStep: -1}
	for i := 0; i < 3; i++ {
		a.appendStep(model.StepRequest)
	}
	a.selectedStep = 2
	a.deleteStep(2) // 删除选中的最后一步
	if a.selectedStep < 0 || a.selectedStep >= len(a.scenario.Steps) {
		t.Fatalf("删除后选中索引越界: %d (steps=%d)", a.selectedStep, len(a.scenario.Steps))
	}
}

// ---------- 运行结果的错误信息：Err 为 nil 不应 panic ----------

func TestResultErrTextSafe(t *testing.T) {
	cases := []struct {
		res  *engine.Result
		want string
	}{
		{&engine.Result{OK: true}, ""},
		{&engine.Result{OK: false}, "未知错误"},
		{&engine.Result{OK: false, Err: errString("boom")}, "boom"},
		{nil, ""},
	}
	for _, c := range cases {
		got := resultErrText(c.res)
		if c.want != "" && !strings.Contains(got, c.want) {
			t.Fatalf("resultErrText 期望含 %q, got %q", c.want, got)
		}
		if c.want == "" && got == "" && c.res != nil && !c.res.OK {
			t.Fatal("失败结果应给出兜底错误文本")
		}
	}
}

type errString string

func (e errString) Error() string { return string(e) }

// ---------- currentEng 并发访问：不应有数据竞争（-race 下验证）----------

func TestCurrentEngConcurrentAccess(t *testing.T) {
	a := &App{}
	sc := model.NewScenario("x")
	eng := engine.NewEngine(sc, nil)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			a.setEngine(eng)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			_ = a.getEngine()
			a.stop()
		}
	}()
	wg.Wait()
}

// ---------- openScenario 解析恶意/畸形 JSON 不应产生半初始化场景 ----------

func TestApplyLoadedScenarioGuards(t *testing.T) {
	a := &App{scenario: model.NewScenario("old"), selectedStep: 0}
	a.lastResult = &engine.Result{}
	sc := &model.Scenario{Name: "new", Variables: nil, Steps: nil}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("applyLoadedScenario panic: %v", r)
			}
		}()
		a.applyLoadedScenario(sc, "/tmp/x.json")
	}()
	if a.selectedStep != -1 || a.lastResult != nil || a.filePath != "/tmp/x.json" {
		t.Fatalf("加载后状态未正确重置: %+v", a)
	}
	if a.scenario.Variables == nil {
		t.Fatal("加载后 Variables 应被初始化")
	}
}

// ---------- 引擎对 nil 场景不应 panic ----------

func TestNewEngineNilScenario(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("NewEngine(nil) panic: %v", r)
		}
	}()
	e := engine.NewEngine(nil, nil)
	res := e.Run()
	// nil 场景应被降级为空场景并安全执行完成，不应 panic 或产生半初始化状态。
	if res == nil {
		t.Fatal("nil 场景应返回一个有效结果")
	}
	if !res.OK || !strings.Contains(res.Scenario, "空场景") {
		t.Fatalf("nil 场景应安全执行, got OK=%v Scenario=%q", res.OK, res.Scenario)
	}
}
