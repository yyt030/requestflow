package engine

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"requestflow/model"
)

// faninServer 提供 ok/fail 两类接口：/ok 返回 success，/fail 返回 500 触发断言失败。
func faninServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/ok":
			fmt.Fprint(w, `{"amount":88}`)
		case "/zero":
			fmt.Fprint(w, `{"amount":0}`)
		default:
			w.WriteHeader(500)
			fmt.Fprint(w, `{"error":"boom"}`)
		}
	}))
}

// okTask 一个必定成功的并行子任务，提取 amount 为产物，命名为 name。
func okTask(name, path string) model.ParallelTask {
	return model.ParallelTask{
		Name: name,
		Steps: []model.Step{{
			ID: name + "-req", Name: name, Type: model.StepRequest, Enabled: true,
			Method: "GET", URL: path,
			Extract: []model.Extract{{Name: "amount", Source: "body", Kind: "jsonpath", Path: "amount"}},
		}},
		Var: "amount",
	}
}

// failTask 一个必定失败的子任务（断言状态码 200，实际 500）。
func failTask(name string) model.ParallelTask {
	return model.ParallelTask{
		Name: name,
		Steps: []model.Step{{
			ID: name + "-req", Name: name, Type: model.StepRequest, Enabled: true,
			Method: "GET", URL: "/fail",
			Assert: []model.Assert{{Source: "status", Op: "eq", Value: "200"}},
		}},
	}
}

// 放行标记步骤：往 vars 写 passed=1，用于判断 merge 是否放行。
func passedStep() model.Step {
	return model.Step{ID: "passed", Name: "放行", Type: model.StepScript, Enabled: true,
		Code: `vars.passed = '1';`}
}

func runFanin(t *testing.T, when string, tasks ...model.ParallelTask) *Result {
	t.Helper()
	srv := faninServer()
	defer srv.Close()
	sc := &model.Scenario{
		Name:    "扇入扇出",
		BaseURL: srv.URL,
		Steps: []model.Step{
			{ID: "fanout", Name: "扇出", Type: model.StepParallel, Enabled: true,
				Parallel: &model.Parallel{Mode: "object", Var: "merged", Tasks: tasks}},
			{ID: "gate", Name: "扇入门控", Type: model.StepMerge, Enabled: true,
				Merge: &model.Merge{When: when}},
			passedStep(),
		},
	}
	return NewEngine(sc, nil).Run()
}

// TestMerge_AllSuccessPasses：全部成功 → task.allSuccess 门控放行。
func TestMerge_AllSuccessPasses(t *testing.T) {
	res := runFanin(t, "task.allSuccess",
		okTask("A", "/ok"), okTask("B", "/ok"))
	if !res.OK {
		t.Fatalf("run failed: %v", res.Err)
	}
	if toStr(res.Vars["passed"]) != "1" {
		t.Fatal("merge 未放行后续步骤")
	}
}

// TestMerge_FailureAborts：有任务失败 → task.allSuccess 为假 → 中止，后续不执行。
func TestMerge_FailureAborts(t *testing.T) {
	res := runFanin(t, "task.allSuccess",
		okTask("A", "/ok"), failTask("B"))
	if res.OK {
		t.Fatal("期望因门控失败而中止，却成功了")
	}
	if toStr(res.Vars["passed"]) == "1" {
		t.Fatal("门控失败后不应执行后续步骤")
	}
	if res.Err == nil || !strings.Contains(res.Err.Error(), "扇入门控") {
		t.Fatalf("错误信息应指明门控失败, got %v", res.Err)
	}
}

// TestMerge_ExpressionAnd：&& 组合检查两个任务都成功 + 产物条件。
func TestMerge_ExpressionAnd(t *testing.T) {
	res := runFanin(t, `task.A.status == "success" && task.B.output > 0`,
		okTask("A", "/ok"), okTask("B", "/ok"))
	if !res.OK || toStr(res.Vars["passed"]) != "1" {
		t.Fatalf("&& 门控应通过: ok=%v err=%v vars=%v", res.OK, res.Err, res.Vars)
	}
}

// TestMerge_ExpressionOr：|| 组合，一个成功即可放行。
func TestMerge_ExpressionOr(t *testing.T) {
	res := runFanin(t, `task.B.status == "success" || task.C.status == "success"`,
		okTask("B", "/ok"), failTask("C"))
	if !res.OK || toStr(res.Vars["passed"]) != "1" {
		t.Fatalf("|| 门控应通过: ok=%v err=%v", res.OK, res.Err)
	}
}

// TestMerge_CountCondition：按失败数判断（允许部分失败）。
func TestMerge_CountCondition(t *testing.T) {
	res := runFanin(t, `task.failed == 0 && task.total == 2`,
		okTask("A", "/ok"), okTask("B", "/ok"))
	if !res.OK || toStr(res.Vars["passed"]) != "1" {
		t.Fatalf("计数门控应通过: ok=%v err=%v", res.OK, res.Err)
	}
}

// TestMerge_OutputGateBlocks：产物条件不满足（amount=0）→ 中止。
func TestMerge_OutputGateBlocks(t *testing.T) {
	res := runFanin(t, `task.A.output > 0`, okTask("A", "/zero"))
	if res.OK {
		t.Fatal("产物为 0 时门控应失败")
	}
	if toStr(res.Vars["passed"]) == "1" {
		t.Fatal("门控失败后不应放行")
	}
}

// TestMerge_LenCondition：len(task) 统计任务表（含汇总键，>=3 合理）。
func TestMerge_LenCondition(t *testing.T) {
	res := runFanin(t, `len(task) >= 3 && task.allSuccess`,
		okTask("A", "/ok"), okTask("B", "/ok"))
	if !res.OK || toStr(res.Vars["passed"]) != "1" {
		t.Fatalf("len 门控应通过: ok=%v err=%v", res.OK, res.Err)
	}
}

// TestMerge_DefaultWhen：When 留空默认 task.allSuccess。
func TestMerge_DefaultWhen(t *testing.T) {
	srv := faninServer()
	defer srv.Close()
	sc := &model.Scenario{
		Name:    "默认门控",
		BaseURL: srv.URL,
		Steps: []model.Step{
			{ID: "f", Name: "扇出", Type: model.StepParallel, Enabled: true,
				Parallel: &model.Parallel{Tasks: []model.ParallelTask{okTask("A", "/ok")}}},
			{ID: "g", Name: "门控", Type: model.StepMerge, Enabled: true, Merge: &model.Merge{}},
			passedStep(),
		},
	}
	res := NewEngine(sc, nil).Run()
	if !res.OK || toStr(res.Vars["passed"]) != "1" {
		t.Fatalf("默认门控应通过: ok=%v err=%v", res.OK, res.Err)
	}
}

// TestTaskStatusRegistered：验证 parallel 后 task.<名>.* 状态变量正确登记。
func TestTaskStatusRegistered(t *testing.T) {
	srv := faninServer()
	defer srv.Close()
	sc := &model.Scenario{
		Name:    "状态登记",
		BaseURL: srv.URL,
		Steps: []model.Step{{
			ID: "f", Name: "扇出", Type: model.StepParallel, Enabled: true,
			Parallel: &model.Parallel{Tasks: []model.ParallelTask{okTask("A", "/ok"), failTask("B")}},
		}},
	}
	res := NewEngine(sc, nil).Run()
	if !res.OK {
		t.Fatalf("run failed: %v", res.Err)
	}
	taskMap, ok := res.Vars["task"].(map[string]interface{})
	if !ok {
		t.Fatalf("task 应为 map, got %T", res.Vars["task"])
	}
	a := taskMap["A"].(map[string]interface{})
	b := taskMap["B"].(map[string]interface{})
	if toStr(a["status"]) != "success" {
		t.Errorf("A.status=%v want success", a["status"])
	}
	if toStr(b["status"]) != "fail" {
		t.Errorf("B.status=%v want fail", b["status"])
	}
	if b["error"] == "" || b["error"] == nil {
		t.Error("B.error 应记录失败原因")
	}
	if toStr(taskMap["success"]) != "1" || toStr(taskMap["failed"]) != "1" {
		t.Errorf("汇总计数错误: %v", taskMap)
	}
	if truthy(taskMap["allSuccess"]) {
		t.Error("allSuccess 应为 false（有失败任务）")
	}
}

// TestFanout_ActuallyConcurrent：验证多个子任务确实并发执行（用互斥计数峰值）。
func TestFanout_ActuallyConcurrent(t *testing.T) {
	var mu sync.Mutex
	inFlight, peak := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		inFlight++
		if inFlight > peak {
			peak = inFlight
		}
		mu.Unlock()
		time.Sleep(50 * time.Millisecond)
		mu.Lock()
		inFlight--
		mu.Unlock()
		fmt.Fprint(w, `{"v":1}`)
	}))
	defer srv.Close()
	tasks := []model.ParallelTask{}
	for _, n := range []string{"A", "B", "C"} {
		tasks = append(tasks, model.ParallelTask{Name: n, Var: "v",
			Steps: []model.Step{{ID: n, Name: n, Type: model.StepRequest, Enabled: true,
				Method: "GET", URL: "/x", Extract: []model.Extract{{Name: "v", Kind: "jsonpath", Path: "v"}}}}})
	}
	sc := &model.Scenario{Name: "并发", BaseURL: srv.URL,
		Steps: []model.Step{{ID: "f", Name: "扇出", Type: model.StepParallel, Enabled: true,
			Parallel: &model.Parallel{Mode: "array", Var: "merged", Tasks: tasks}}}}
	res := NewEngine(sc, nil).Run()
	if !res.OK {
		t.Fatalf("run failed: %v", res.Err)
	}
	if peak < 2 {
		t.Errorf("子任务应并发执行, peak=%d", peak)
	}
}
