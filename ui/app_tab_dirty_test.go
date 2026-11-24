package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"requestflow/model"
)

func newDirtyTestApp(step model.Step) (*App, *model.Step) {
	a := &App{
		tabDirty: map[string]string{},
		scenario: &model.Scenario{Steps: []model.Step{step}},
	}
	return a, &a.scenario.Steps[0]
}

// TestTabDirtyRevertToEmpty 验证"填了又清空"与"从没填过"指纹一致，圆圈自动消失。
// 覆盖 map（Query/Headers）与 slice（Extract）两类空容器，这正是此前 bug 的根因。
func TestTabDirtyRevertToEmpty(t *testing.T) {
	step := model.Step{ID: "s1", Type: model.StepRequest, Method: "GET"}
	a, live := newDirtyTestApp(step)
	a.refreshTabBaseline()

	// 基线干净。
	if a.tabIsDirty(live, "Params") || a.tabIsDirty(live, "Headers") || a.tabIsDirty(live, "提取") {
		t.Fatal("建立基线时不应有任何标记")
	}

	// 填入内容 → 变脏。
	live.Query = map[string]string{"page": "2"}
	live.Headers = map[string]string{"X-Token": "abc"}
	live.Extract = []model.Extract{{Name: "token", Source: "body", Kind: "jsonpath", Path: "data.token"}}
	if !a.tabIsDirty(live, "Params") || !a.tabIsDirty(live, "Headers") || !a.tabIsDirty(live, "提取") {
		t.Fatal("填入内容后相关标签应被标记")
	}

	// 清空内容（模拟真实 UI：stringToMap 返回空 map、移除最后一条规则返回空 slice）→ 自动变干净。
	live.Query = map[string]string{}
	live.Headers = map[string]string{}
	live.Extract = []model.Extract{}
	if a.tabIsDirty(live, "Params") || a.tabIsDirty(live, "Headers") || a.tabIsDirty(live, "提取") {
		t.Fatal("清空内容后圆圈应自动消失，不应仍被标记")
	}
}

// TestTabDirtyRevertToExactValue 验证改回原始值后圆圈消失（字符串字段的常规撤销）。
func TestTabDirtyRevertToExactValue(t *testing.T) {
	step := model.Step{ID: "s2", Type: model.StepRequest, Method: "GET", URL: "https://a.example"}
	a, live := newDirtyTestApp(step)
	a.refreshTabBaseline()

	live.URL = "https://b.example"
	if !a.tabIsDirty(live, "常规") {
		t.Fatal("修改 URL 后应被标记")
	}
	live.URL = "https://a.example"
	if a.tabIsDirty(live, "常规") {
		t.Fatal("改回原值后圆圈应自动消失")
	}
}

// TestCanonicalFingerprintEmptyEqualsNil 验证规范化函数本身：各种"空"形态指纹相同，非空有区分。
func TestCanonicalFingerprintEmptyEqualsNil(t *testing.T) {
	emptyMap := canonicalFingerprint([]any{map[string]string{}})
	emptySlice := canonicalFingerprint([]any{[]string{}})
	nilVal := canonicalFingerprint([]any{nil})
	if emptyMap != nilVal || emptySlice != nilVal {
		t.Fatalf("空容器应与 nil 指纹一致: map=%s slice=%s nil=%s", emptyMap, emptySlice, nilVal)
	}
	nonEmpty := canonicalFingerprint([]any{map[string]string{"a": "1"}})
	if nonEmpty == nilVal {
		t.Fatalf("非空容器指纹不应等于空: %s", nonEmpty)
	}
}

// TestCanonicalFingerprintNumberPrecision 验证数字精度不被 float64 破坏。
func TestCanonicalFingerprintNumberPrecision(t *testing.T) {
	big := "12345678901234567890"
	got := canonicalFingerprint([]any{json.Number(big)})
	if !strings.Contains(got, big) {
		t.Fatalf("大整数精度丢失: %s", got)
	}
}
