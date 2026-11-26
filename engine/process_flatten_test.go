package engine

import (
	"testing"

	"requestflow/model"
)

// TestProcessFlattenNested 验证 flatten 能把「循环 append 页数组」产生的嵌套数组
// [[row,row],[row,row]] 递归展平为单行数组，且保留 map 行对象不被拆散。
// 这是豆瓣 Top100 抓取（loop 翻页 → cssRows 提取 → append 累积 → flatten）的核心链路。
func TestProcessFlattenNested(t *testing.T) {
	ctx := NewContext(nil)
	page1 := []interface{}{
		map[string]interface{}{"title": "第1名", "rating": "9.9"},
		map[string]interface{}{"title": "第2名", "rating": "9.8"},
	}
	page2 := []interface{}{
		map[string]interface{}{"title": "第3名", "rating": "9.7"},
	}
	ctx.Set("allRows", []interface{}{page1, page2})

	step := model.Step{Op: "flatten", Args: map[string]string{"from": "allRows", "to": "flat"}}
	if err := runProcess(ctx, step); err != nil {
		t.Fatalf("flatten 执行失败: %v", err)
	}
	flat, ok := toSlice(ctx.Get("flat"))
	if !ok {
		t.Fatal("flatten 结果不是数组")
	}
	if len(flat) != 3 {
		t.Fatalf("展平后条数 = %d, 期望 3", len(flat))
	}
	first, ok := flat[0].(map[string]interface{})
	if !ok || first["title"] != "第1名" {
		t.Fatalf("首行应为原始 map 行对象, 实际 %#v", flat[0])
	}
}

// TestProcessFlattenMultipleVars 验证 flatten 仍支持合并多个数组变量（原有语义）。
func TestProcessFlattenMultipleVars(t *testing.T) {
	ctx := NewContext(nil)
	ctx.Set("a", []interface{}{"x", "y"})
	ctx.Set("b", []interface{}{"z"})

	step := model.Step{Op: "flatten", Args: map[string]string{"from": "a,b", "to": "merged"}}
	if err := runProcess(ctx, step); err != nil {
		t.Fatalf("flatten 执行失败: %v", err)
	}
	merged, _ := toSlice(ctx.Get("merged"))
	if len(merged) != 3 || merged[0] != "x" || merged[2] != "z" {
		t.Fatalf("合并结果不符: %#v", merged)
	}
}
