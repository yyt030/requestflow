package ui

import (
	"testing"

	"requestflow/model"
)

func TestDebugTabFingerprintDistinguishesTabs(t *testing.T) {
	a := &App{
		debugQuery:    "page: 1",
		debugHeaders:  "A: b",
		debugBody:     `{"x":1}`,
		debugBodyType: "raw",
		debugAuthType: "bearer",
	}
	fp := make([]string, len(debugReqTabNames))
	for i := range debugReqTabNames {
		fp[i] = debugTabFingerprint(a, i)
	}
	for i := range fp {
		for j := i + 1; j < len(fp); j++ {
			if fp[i] == fp[j] {
				t.Fatalf("标签 %q 与 %q 指纹相同，无法区分: %s", debugReqTabNames[i], debugReqTabNames[j], fp[i])
			}
		}
	}
}

func TestDebugTabDirtyLifecycle(t *testing.T) {
	a := &App{debugQuery: "page: 1", debugHeaders: "A: b"}
	a.refreshDebugTabBaseline()
	if a.debugTabIsDirty(0) || a.debugTabIsDirty(1) || a.debugTabIsDirty(2) || a.debugTabIsDirty(3) {
		t.Fatal("刚建立基线时不应有任何标签被标记")
	}

	// 编辑 Params：只有 Params 变脏。
	a.debugQuery = "page: 2"
	if !a.debugTabIsDirty(0) {
		t.Fatal("编辑 debugQuery 后 Params 应被标记")
	}
	if a.debugTabIsDirty(1) || a.debugTabIsDirty(2) || a.debugTabIsDirty(3) {
		t.Fatal("未编辑的标签不应被标记")
	}

	// 编辑 Body 的一个字段：Body 变脏，Authorization 保持干净。
	a.debugBody = "hello"
	a.debugBodyType = "raw"
	if !a.debugTabIsDirty(2) {
		t.Fatal("编辑 Body 后 Body 标签应被标记")
	}
	if a.debugTabIsDirty(3) {
		t.Fatal("Authorization 未编辑不应被标记")
	}

	// 刷新基线应清除所有标记。
	a.refreshDebugTabBaseline()
	for i := range debugReqTabNames {
		if a.debugTabIsDirty(i) {
			t.Fatalf("刷新基线后 %q 不应再被标记", debugReqTabNames[i])
		}
	}
}

func TestDebugTabDirtyUnchangedAcrossLoad(t *testing.T) {
	step := &model.Step{Method: "POST", URL: "/x", Query: map[string]string{"a": "1"}, Headers: map[string]string{"H": "v"}}
	a := &App{}
	a.loadStepToDebug(step)
	for i := range debugReqTabNames {
		if a.debugTabIsDirty(i) {
			t.Fatalf("loadStepToDebug 后 %q 应为干净基线", debugReqTabNames[i])
		}
	}
}
