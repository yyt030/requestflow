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

// ---------- 可拖拽文本框高度钳制 ----------

func TestClampHeight(t *testing.T) {
	cases := []struct {
		v, min, max, want float32
	}{
		{100, 40, 600, 100}, // 区间内原样
		{20, 40, 600, 40},   // 低于下限
		{900, 40, 600, 600}, // 超过上限
		{40, 40, 600, 40},   // 正好下限
		{600, 40, 600, 600}, // 正好上限
		{-5, 40, 600, 40},   // 负值（向上拖过头）
	}
	for _, c := range cases {
		if got := clampHeight(c.v, c.min, c.max); got != c.want {
			t.Fatalf("clampHeight(%v,%v,%v)=%v, want %v", c.v, c.min, c.max, got, c.want)
		}
	}
}

type errString string

func (e errString) Error() string { return string(e) }

// ---------- 双击步骤进入调试：request 载入调试面板，其它类型不误入 ----------

func TestOpenStepDebugRequest(t *testing.T) {
	a := &App{scenario: model.NewScenario("s"), selectedStep: -1}
	a.appendStep(model.StepRequest)
	// 填充一个带完整参数的 request 步骤
	step := &a.scenario.Steps[0]
	step.Method = "POST"
	step.URL = "https://api.example.com/login"
	step.Headers = map[string]string{"Content-Type": "application/json"}
	step.Query = map[string]string{"page": "2"}
	step.Body = `{"u":"x"}`

	a.openStepDebug(0)

	if !a.showDebug {
		t.Fatal("双击 request 步骤应进入调试面板")
	}
	if a.debugMethod != "POST" || a.debugURL != step.URL || a.debugBody != step.Body {
		t.Fatalf("调试参数未正确载入: method=%q url=%q body=%q", a.debugMethod, a.debugURL, a.debugBody)
	}
	if !strings.Contains(a.debugHeaders, "Content-Type = application/json") ||
		!strings.Contains(a.debugQuery, "page = 2") {
		t.Fatalf("调试 Header/Query 未载入: %q %q", a.debugHeaders, a.debugQuery)
	}
	if a.debugResult != nil {
		t.Fatal("仅载入不应自动发送请求（debugResult 应为空）")
	}
}

func TestOpenStepDebugNonRequest(t *testing.T) {
	a := &App{scenario: model.NewScenario("s"), selectedStep: -1}
	a.appendStep(model.StepLoop)
	a.showDebug = true // 预设处于调试视图

	a.openStepDebug(0)

	if a.showDebug {
		t.Fatal("双击非 request 步骤不应停留在调试面板")
	}
	if a.selectedStep != 0 {
		t.Fatalf("双击应选中该步骤, got %d", a.selectedStep)
	}
}

func TestLoadStepToDebugDefaults(t *testing.T) {
	a := &App{scenario: model.NewScenario("s"), selectedStep: -1}
	a.appendStep(model.StepRequest)
	step := &a.scenario.Steps[0]
	step.Method = "" // 空方法应默认 GET
	step.AuthType = ""

	a.loadStepToDebug(step)

	if a.debugMethod != "GET" {
		t.Fatalf("空方法应默认 GET, got %q", a.debugMethod)
	}
	if a.debugAuthType != "none" {
		t.Fatalf("空认证应默认 none, got %q", a.debugAuthType)
	}
	if a.debugTimeout != "30" {
		t.Fatalf("超时应默认 30, got %q", a.debugTimeout)
	}
}

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
