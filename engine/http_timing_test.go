package engine

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"requestflow/model"
)

// TestDebugHTTPTiming 验证 HTTP 请求能采集到分段耗时：
// 服务端引入可观测延迟，TTFB（Wait）应捕获该延迟；各段之和与总耗时接近。
func TestDebugHTTPTiming(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(25 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":1}`))
	}))
	defer srv.Close()

	res := Debug(DebugRequest{Method: "GET", URL: srv.URL}, nil)
	if !res.OK {
		t.Fatal(res.Err)
	}
	tm := res.Timing
	if tm == nil || !tm.Available {
		t.Fatal("HTTP 请求应采集到分段耗时")
	}
	if tm.Wait < 20 {
		t.Fatalf("TTFB 应捕获服务端 25ms 延迟，实际 %.2f ms", tm.Wait)
	}
	if tm.Total < 20 {
		t.Fatalf("总耗时过小: %.2f ms", tm.Total)
	}
	sum := tm.DNS + tm.Connect + tm.TLS + tm.Send + tm.Wait + tm.Download + tm.Other
	diff := tm.Total - sum
	if sum < 0 || diff > 1 || diff < -1 {
		t.Fatalf("分段之和应与总耗时接近: total=%.2f sum=%.2f", tm.Total, sum)
	}
	phases := tm.Phases()
	if len(phases) == 0 {
		t.Fatal("应至少有一个可展示分段")
	}
	foundWait := false
	for _, p := range phases {
		if p.Name == "等待响应 TTFB" {
			foundWait = true
		}
	}
	if !foundWait {
		t.Fatalf("分段明细应包含等待响应 TTFB，实际: %+v", phases)
	}
}

// TestTimingPhasesNil 验证 nil / 不可用时不返回分段（TCP 与失败响应走此路径）。
func TestTimingPhasesNil(t *testing.T) {
	var tm *model.HTTPTiming
	if got := tm.Phases(); got != nil {
		t.Fatalf("nil 应返回 nil，实际 %+v", got)
	}
	unavailable := &model.HTTPTiming{Available: false, Total: 100}
	if got := unavailable.Phases(); got != nil {
		t.Fatalf("不可用应返回 nil，实际 %+v", got)
	}
}
