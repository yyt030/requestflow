package ui

import (
	"testing"
)

func TestProcessExamplesDataValid(t *testing.T) {
	exs := processExamplesData()
	if len(exs) < 7 {
		t.Fatalf("例子数量应至少 7 个覆盖高频操作，实际 %d", len(exs))
	}
	ops := map[string]bool{}
	for _, ex := range exs {
		if ex.op == "" || ex.args == nil {
			t.Fatalf("例子 %q 缺少 op 或 args", ex.name)
		}
		if ops[ex.op] {
			t.Fatalf("op %q 被多个例子重复占用，应聚焦不同操作", ex.op)
		}
		ops[ex.op] = true
		// 例子必须能在 processOpDefs 中找到，保证界面可渲染对应表单。
		if _, ok := processOpDefs()[ex.op]; !ok {
			t.Fatalf("例子 %q 的 op %q 无对应表单定义", ex.name, ex.op)
		}
		// 每个例子至少有一个参数，避免点了之后空配置。
		if len(ex.args) == 0 {
			t.Fatalf("例子 %q 参数为空", ex.name)
		}
	}
}

func TestProcessExamplesRequiredKeys(t *testing.T) {
	// 关键高频 op 的例子必须带 from（join 除外，它用 values）。
	needFrom := map[string]bool{"filter": true, "limit": true, "sort": true, "pick": true, "dedup": true}
	for _, ex := range processExamplesData() {
		if needFrom[ex.op] && ex.args["from"] == "" {
			t.Fatalf("例子 %q (op=%s) 缺少 from 参数", ex.name, ex.op)
		}
		if ex.op == "join" && ex.args["values"] == "" {
			t.Fatalf("join 例子缺少 values 参数")
		}
	}
}
