package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"requestflow/engine"
)

func TestSaveHTMLAndJSON(t *testing.T) {
	res := &engine.Result{
		Scenario: "登录流程",
		OK:       false,
		Started:  time.Now().Add(-2 * time.Second),
		Ended:    time.Now(),
		Steps: []engine.StepTrace{
			{Name: "登录", Type: "request", Status: "pass", DurationMS: 100},
			{Name: "断言", Type: "assert", Status: "fail", Err: "断言失败 [状态]: 期望 200", DurationMS: 5},
		},
		Assertions: []engine.AssertDetail{
			{Name: "状态", Source: "status", Op: "eq", Expected: "200", Passed: false, Detail: "实际=500"},
		},
	}
	if res.Err != nil {
		_ = res.Err
	}

	d := NewFromResult(res)
	if d.Total != 1 || d.Passed != 0 || d.Failed != 1 {
		t.Fatalf("summary = %+v", d)
	}
	dir := t.TempDir()

	htmlPath := filepath.Join(dir, "r.html")
	if err := Save(d, htmlPath); err != nil {
		t.Fatal(err)
	}
	hb, _ := os.ReadFile(htmlPath)
	if !strings.Contains(string(hb), "登录流程") || !strings.Contains(string(hb), "断言失败") {
		t.Fatalf("HTML 内容缺失: %s", string(hb)[:min(len(hb), 300)])
	}

	jsonPath := filepath.Join(dir, "r.json")
	if err := Save(d, jsonPath); err != nil {
		t.Fatal(err)
	}
	jb, _ := os.ReadFile(jsonPath)
	if !strings.Contains(string(jb), `"failed": 1`) || !strings.Contains(string(jb), "登录") {
		t.Fatalf("JSON 内容缺失: %s", string(jb)[:min(len(jb), 300)])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
