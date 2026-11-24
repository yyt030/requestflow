package ui

import (
	"strings"
	"testing"

	"requestflow/engine"
	"requestflow/model"
)

func complexSampleResp() *model.Response {
	dr := &engine.DebugResult{StatusCode: 200, Body: sampleComplexJSONResponse, IsJSON: true}
	return engine.PreviewResponseFromDebug(dr)
}

// TestComplexJSONExamplesExtract 验证 7 条复杂 JSON 提取例子都能出预期结果。
func TestComplexJSONExamplesExtract(t *testing.T) {
	resp := complexSampleResp()
	wantCount := map[string]int{
		"code": 1, "total": 1, "author3": 1, "record": 1,
		"topTitles": 6, "wangs": 1, "rows": 20,
	}
	wantText := map[string]string{
		"code": "0", "total": "20", "author3": "王五",
		"wangs": "丙",
	}
	exs := complexExtractExamplesData()
	if len(exs) != 7 {
		t.Fatalf("复杂 JSON 提取例子数 = %d, 期望 7", len(exs))
	}
	extracts := make([]model.Extract, 0, len(exs))
	for _, ex := range exs {
		extracts = append(extracts, model.Extract{Name: ex.varName, Source: "body", Kind: "jsonpath", Path: ex.path})
	}
	items := engine.PreviewExtracts(extracts, map[string]string{}, resp)
	if len(items) != len(exs) {
		t.Fatalf("试算条目数 = %d", len(items))
	}
	for _, item := range items {
		if item.Err != "" {
			t.Fatalf("提取例 %s 出错: %s", item.Name, item.Err)
		}
		if c, ok := wantCount[item.Name]; ok && item.Count != c {
			t.Fatalf("提取例 %s 条数 = %d, 期望 %d（文本 %s）", item.Name, item.Count, c, item.Text)
		}
		if s, ok := wantText[item.Name]; ok && item.Text != s {
			t.Fatalf("提取例 %s 文本 = %q, 期望 %q", item.Name, item.Text, s)
		}
		if item.Name == "record" && !strings.Contains(item.Text, `"id":3`) {
			t.Fatalf("按 id 取整条应保留完整对象: %s", item.Text)
		}
	}
}

// TestComplexJSONExamplesProcess 验证整页 list 取出后，3 条处理例子能直接试算。
func TestComplexJSONExamplesProcess(t *testing.T) {
	resp := complexSampleResp()
	items := engine.PreviewExtracts([]model.Extract{
		{Name: "rows", Source: "body", Kind: "jsonpath", Path: "body.list"},
	}, map[string]string{}, resp)
	if len(items) != 1 || items[0].Count != 20 {
		t.Fatalf("整页 list 提取失败: %+v", items)
	}
	vars := map[string]string{"rows": engine.PreviewValueString(items[0].Value)}

	for _, ex := range complexProcessExamplesData() {
		r := engine.PreviewProcess(model.Step{Op: ex.op, Args: ex.args}, vars)
		if r.Err != "" {
			t.Fatalf("处理例 %s 出错: %s", ex.name, r.Err)
		}
		switch ex.name {
		case "复杂JSON·高分过滤":
			if !strings.Contains(r.AfterT, "6 条") {
				t.Fatalf("score>90 应剩 6 条，实际: %s", r.AfterT)
			}
		case "复杂JSON·按分排序":
			if !strings.Contains(r.AfterT, "酉") || !strings.Contains(r.AfterT, "20 条") {
				t.Fatalf("按分降序应含最高分酉，实际: %s", r.AfterT)
			}
		case "复杂JSON·精简字段":
			if strings.Contains(r.AfterT, "detail") {
				t.Fatalf("pick 应丢掉 detail 字段: %s", r.AfterT)
			}
		}
	}
}
