package engine

import (
	"strings"
	"testing"

	"requestflow/model"
)

// TestLoopDedupByField 验证循环结束后对累积数组按字段去重，并把重复数写入统计变量与日志。
func TestLoopDedupByField(t *testing.T) {
	// 每轮向 allRows 追加两行，其中 "link=https://a" 出现两次。
	appendA := model.Step{ID: "a", Name: "累积A", Type: model.StepProcess, Enabled: true,
		Op: "set", Args: map[string]string{"name": "row", "value": `{"link":"https://a","title":"A1"}`}}
	appendB := model.Step{ID: "b", Name: "累积B", Type: model.StepProcess, Enabled: true,
		Op: "set", Args: map[string]string{"name": "row", "value": `{"link":"https://b","title":"B"}`}}
	// 用 script 把 row（JSON 字符串）解析成对象再 append，模拟提取结果。
	push := model.Step{ID: "p", Name: "追加", Type: model.StepScript, Enabled: true, Code: `
		var obj = JSON.parse(vars.row);
		var acc = vars.allRows || [];
		acc.push(obj);
		vars.allRows = acc;
	`}

	loop := model.Step{ID: "loop", Name: "翻页", Type: model.StepLoop, Enabled: true,
		Loop: &model.Loop{
			Mode: "count", Count: 2,
			DedupVar: "allRows", DedupField: "link", DedupStat: "dupCount",
		},
		Children: []model.Step{appendA, push, appendB, push},
	}

	var logs []string
	eng := NewEngine(&model.Scenario{Name: "dedup loop", Steps: []model.Step{loop}},
		func(e Event) {
			if e.Type == "log" && strings.Contains(e.Message, "累积去重") {
				logs = append(logs, e.Message)
			}
		})
	res := eng.Run()
	if !res.OK {
		t.Fatalf("执行失败: %v", res.Err)
	}
	got, ok := res.Vars["allRows"].([]interface{})
	if !ok || len(got) != 2 {
		t.Fatalf("去重后应剩 2 行: %T %v", res.Vars["allRows"], got)
	}
	if n, _ := toFloat(res.Vars["dupCount"]); n != 2 {
		t.Fatalf("dupCount = %v, 期望 2（两轮各有一个 https://a、https://b 重复）", res.Vars["dupCount"])
	}
	if len(logs) == 0 || !strings.Contains(logs[0], "去掉 2 条重复") {
		t.Fatalf("日志未提示重复数: %v", logs)
	}
}
