package history

import (
	"path/filepath"
	"testing"
)

func TestHistoryAppendLoadClear(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "history.jsonl")
	SetPath(tmp)
	defer SetPath("")

	e1 := Entry{Method: "GET", URL: "https://a.example.com", Status: 200, DurationMS: 12, Source: "debug"}
	e2 := Entry{Method: "POST", URL: "https://b.example.com", Status: 201, DurationMS: 34, Source: "debug"}
	if err := Append(e1); err != nil {
		t.Fatalf("Append e1: %v", err)
	}
	if err := Append(e2); err != nil {
		t.Fatalf("Append e2: %v", err)
	}
	list, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("期望 2 条，得到 %d", len(list))
	}
	// 最新在前
	if list[0].URL != "https://b.example.com" {
		t.Fatalf("顺序错误，最新应为 b，得到 %s", list[0].URL)
	}
	if list[1].Status != 200 {
		t.Fatalf("e1 状态错误: %d", list[1].Status)
	}
	if list[0].Time == "" {
		t.Fatal("时间未自动填充")
	}
	if err := Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	list2, _ := Load()
	if len(list2) != 0 {
		t.Fatalf("清空后应为空，得到 %d", len(list2))
	}
}

func TestLoadNoFile(t *testing.T) {
	SetPath(filepath.Join(t.TempDir(), "not-exist.jsonl"))
	defer SetPath("")
	list, err := Load()
	if err != nil {
		t.Fatalf("文件不存在应返回空而非错误: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("应为空，得到 %d", len(list))
	}
}
