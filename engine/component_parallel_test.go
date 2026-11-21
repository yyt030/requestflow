package engine

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"requestflow/model"
)

// parallelHTTPServer 返回按路径区分的 JSON，用于验证并行请求。
func parallelHTTPServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/a":
			fmt.Fprint(w, `{"items":[{"id":"a1"},{"id":"a2"}]}`)
		case "/b":
			fmt.Fprint(w, `{"items":[{"id":"b1"}]}`)
		case "/slow":
			time.Sleep(40 * time.Millisecond)
			fmt.Fprint(w, `{"items":[{"id":"slow"}]}`)
		default:
			fmt.Fprint(w, `{"items":[]}`)
		}
	}))
}

// reqTask 构造一个并行子任务：请求 path 并把 items 提取为 rows 变量。

// reqTask 构造一个并行子任务：请求 path 并把 items 提取为 rows 变量。
func reqTask(name, path string) model.ParallelTask {
	return model.ParallelTask{
		Name: name,
		Steps: []model.Step{{
			ID: name + "-req", Name: name, Type: model.StepRequest, Enabled: true,
			Method: "GET", URL: path,
			Extract: []model.Extract{{Name: "rows", Source: "body", Kind: "jsonpath", Path: "items"}},
		}},
		Var: "rows",
	}
}

func runParallelScenario(t *testing.T, mode string, tasks ...model.ParallelTask) *Result {
	t.Helper()
	srv := parallelHTTPServer()
	defer srv.Close()
	sc := &model.Scenario{
		Name:    "parallel",
		BaseURL: srv.URL,
		Steps: []model.Step{{
			ID: "p", Name: "并行汇合", Type: model.StepParallel, Enabled: true,
			Parallel: &model.Parallel{Mode: mode, Var: "merged", Tasks: tasks},
		}},
	}
	res := NewEngine(sc, nil).Run()
	if !res.OK {
		t.Fatalf("run failed: %v", res.Err)
	}
	return res
}

// TestParallel_Table：多个任务并行取数后展开合并成一张表。

// TestParallel_Table：多个任务并行取数后展开合并成一张表。
func TestParallel_Table(t *testing.T) {
	res := runParallelScenario(t, "table",
		reqTask("A", "/a"), reqTask("B", "/b"), reqTask("slow", "/slow"))
	rows, ok := res.Vars["merged"].([]interface{})
	if !ok {
		t.Fatalf("merged type = %T, want []interface{}", res.Vars["merged"])
	}
	if len(rows) != 4 {
		t.Fatalf("merged rows = %d, want 4", len(rows))
	}
	// 顺序由任务下标固定，与完成先后无关
	ids := []string{}
	for _, r := range rows {
		m := r.(map[string]interface{})
		ids = append(ids, toStr(m["id"]))
	}
	want := []string{"a1", "a2", "b1", "slow"}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("ids = %v, want %v", ids, want)
		}
	}
}

// TestParallel_Array：array 模式保留每个任务的产物为独立元素。

// TestParallel_Array：array 模式保留每个任务的产物为独立元素。
func TestParallel_Array(t *testing.T) {
	res := runParallelScenario(t, "array", reqTask("A", "/a"), reqTask("B", "/b"))
	arr, ok := res.Vars["merged"].([]interface{})
	if !ok || len(arr) != 2 {
		t.Fatalf("merged = %T len=%d, want array len 2", res.Vars["merged"], len(arr))
	}
}

// TestParallel_Object：object 模式以任务名为 key。

// TestParallel_Object：object 模式以任务名为 key。
func TestParallel_Object(t *testing.T) {
	res := runParallelScenario(t, "object", reqTask("A", "/a"), reqTask("B", "/b"))
	obj, ok := res.Vars["merged"].(map[string]interface{})
	if !ok {
		t.Fatalf("merged type = %T, want map", res.Vars["merged"])
	}
	if _, ok := obj["A"]; !ok {
		t.Error("object missing key A")
	}
	if _, ok := obj["B"]; !ok {
		t.Error("object missing key B")
	}
}

// TestParallel_Concat：concat 模式把各任务的文本产物拼接。

// TestParallel_Concat：concat 模式把各任务的文本产物拼接。
func TestParallel_Concat(t *testing.T) {
	tasks := []model.ParallelTask{
		{Name: "x", Value: "hello", Steps: []model.Step{}},
		{Name: "y", Value: "world", Steps: []model.Step{}},
	}
	res := runParallelScenario(t, "concat", tasks...)
	// 用默认分隔符为空字符串，实验中通过显式 sep 场景覆盖
	if got := toStr(res.Vars["merged"]); got != "helloworld" {
		t.Errorf("merged = %q, want helloworld", got)
	}
}

// TestParallel_ConcatWithSeparator：自定义分隔符。

// TestParallel_ConcatWithSeparator：自定义分隔符。
func TestParallel_ConcatWithSeparator(t *testing.T) {
	srv := parallelHTTPServer()
	defer srv.Close()
	sc := &model.Scenario{
		Name:    "parallel concat sep",
		BaseURL: srv.URL,
		Steps: []model.Step{{
			ID: "p", Name: "p", Type: model.StepParallel, Enabled: true,
			Parallel: &model.Parallel{Mode: "concat", Var: "merged", Separator: ",",
				Tasks: []model.ParallelTask{
					{Name: "x", Value: "a"},
					{Name: "y", Value: "b"},
				}},
		}},
	}
	res := NewEngine(sc, nil).Run()
	if !res.OK {
		t.Fatalf("run failed: %v", res.Err)
	}
	if got := toStr(res.Vars["merged"]); got != "a,b" {
		t.Errorf("merged = %q, want a,b", got)
	}
}

// TestParallel_FailFast：子任务失败且 failFast 时整体失败。

// TestParallel_FailFast：子任务失败且 failFast 时整体失败。
func TestParallel_FailFast(t *testing.T) {
	sc := &model.Scenario{
		Name: "parallel failfast",
		Steps: []model.Step{{
			ID: "p", Name: "p", Type: model.StepParallel, Enabled: true,
			Parallel: &model.Parallel{Mode: "array", Var: "merged", FailFast: true,
				Tasks: []model.ParallelTask{{
					Name: "bad",
					Steps: []model.Step{{ID: "s", Name: "s", Type: model.StepScript, Enabled: true,
						Code: `throw new Error('boom');`}},
				}}},
		}},
	}
	res := NewEngine(sc, nil).Run()
	if res.OK {
		t.Fatal("expected failure with failFast=true, got OK")
	}
}

// TestParallel_SandboxIsolation：并行子任务写同名变量不互相污染父上下文。

// TestParallel_SandboxIsolation：并行子任务写同名变量不互相污染父上下文。
func TestParallel_SandboxIsolation(t *testing.T) {
	sc := &model.Scenario{
		Name: "parallel sandbox",
		Steps: []model.Step{
			{ID: "init", Name: "init", Type: model.StepProcess, Enabled: true, Op: "set",
				Args: map[string]string{"name": "shared", "value": "parent"}},
			{ID: "p", Name: "p", Type: model.StepParallel, Enabled: true,
				Parallel: &model.Parallel{Mode: "object", Var: "merged",
					Tasks: []model.ParallelTask{
						{Name: "t1", Steps: []model.Step{{ID: "s1", Name: "s1", Type: model.StepScript, Enabled: true,
							Code: `vars.shared = 'child1';`}}, Var: "shared"},
						{Name: "t2", Steps: []model.Step{{ID: "s2", Name: "s2", Type: model.StepScript, Enabled: true,
							Code: `vars.shared = 'child2';`}}, Var: "shared"},
					}}},
		},
	}
	res := NewEngine(sc, nil).Run()
	if !res.OK {
		t.Fatalf("run failed: %v", res.Err)
	}
	if got := toStr(res.Vars["shared"]); got != "parent" {
		t.Errorf("parent shared = %q, want parent (沙箱不得回写)", got)
	}
	obj := res.Vars["merged"].(map[string]interface{})
	if toStr(obj["t1"]) != "child1" || toStr(obj["t2"]) != "child2" {
		t.Errorf("沙箱产物错误: %v", obj)
	}
}

// TestParallel_EventCallbackSafe：并行任务的事件回调被串行调用（-race 下验证）。

// TestParallel_EventCallbackSafe：并行任务的事件回调被串行调用（-race 下验证）。
func TestParallel_EventCallbackSafe(t *testing.T) {
	var mu sync.Mutex
	count := 0
	onEvent := func(Event) {
		mu.Lock()
		count++
		mu.Unlock()
	}
	sc := &model.Scenario{
		Name: "parallel events",
		Steps: []model.Step{{
			ID: "p", Name: "p", Type: model.StepParallel, Enabled: true,
			Parallel: &model.Parallel{Mode: "array", Var: "merged",
				Tasks: []model.ParallelTask{
					{Name: "t1", Steps: []model.Step{{ID: "s1", Name: "s1", Type: model.StepProcess, Enabled: true,
						Op: "set", Args: map[string]string{"name": "x", "value": "1"}}}, Var: "x"},
					{Name: "t2", Steps: []model.Step{{ID: "s2", Name: "s2", Type: model.StepProcess, Enabled: true,
						Op: "set", Args: map[string]string{"name": "x", "value": "2"}}}, Var: "x"},
				}},
		}},
	}
	res := NewEngine(sc, onEvent).Run()
	if !res.OK {
		t.Fatalf("run failed: %v", res.Err)
	}
	if count == 0 {
		t.Error("expected events, got none")
	}
}
