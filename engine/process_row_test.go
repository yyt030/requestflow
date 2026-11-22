package engine

import (
	"testing"

	"requestflow/model"
)

// TestProcessRowBasic 验证 row 把字段表达式组装成对象行并追加到数组。
func TestProcessRowBasic(t *testing.T) {
	ctx := NewContext(nil)
	ctx.Set("item", map[string]interface{}{"title": "算法导论", "rating": "9.3"})
	ctx.Set("bing2", []interface{}{
		map[string]interface{}{"title": "算法导论 官网", "url": "https://a.com"},
		map[string]interface{}{"title": "算法导论 豆瓣", "url": "https://b.com"},
	})

	step := model.Step{Op: "row", Args: map[string]string{
		"fields": "book,rating,title1,link1,title2,link2",
		"book":   "{{item.title}}",
		"rating": "{{item.rating}}",
		"title1": "{{bing2.0.title}}",
		"link1":  "{{bing2.0.url}}",
		"title2": "{{bing2.1.title}}",
		"link2":  "{{bing2.1.url}}",
		"to":     "final",
	}}
	if err := runProcess(ctx, step); err != nil {
		t.Fatalf("row 执行失败: %v", err)
	}

	list, ok := toSlice(ctx.Get("final"))
	if !ok || len(list) != 1 {
		t.Fatalf("final 应为 1 行数组, got %v", ctx.Get("final"))
	}
	row, ok := list[0].(map[string]interface{})
	if !ok {
		t.Fatalf("行应为对象, got %T", list[0])
	}
	want := map[string]string{
		"book": "算法导论", "rating": "9.3",
		"title1": "算法导论 官网", "link1": "https://a.com",
		"title2": "算法导论 豆瓣", "link2": "https://b.com",
	}
	for k, v := range want {
		if got := toStr(row[k]); got != v {
			t.Errorf("字段 %s = %q, 期望 %q", k, got, v)
		}
	}
}

// TestProcessRowAccumulate 验证多次执行 row 会逐行累积（模拟 foreach 多轮）。
func TestProcessRowAccumulate(t *testing.T) {
	ctx := NewContext(nil)
	step := model.Step{Op: "row", Args: map[string]string{
		"fields": "book,rating",
		"book":   "{{item.title}}",
		"rating": "{{item.rating}}",
		"to":     "final",
	}}

	for _, book := range []struct{ title, rating string }{{"A", "9.8"}, {"B", "9.6"}, {"C", "9.4"}} {
		ctx.Set("item", map[string]interface{}{"title": book.title, "rating": book.rating})
		if err := runProcess(ctx, step); err != nil {
			t.Fatalf("row 执行失败: %v", err)
		}
	}

	list, ok := toSlice(ctx.Get("final"))
	if !ok || len(list) != 3 {
		t.Fatalf("final 应为 3 行, got %v", ctx.Get("final"))
	}
	for i, want := range []string{"A", "B", "C"} {
		row := list[i].(map[string]interface{})
		if got := toStr(row["book"]); got != want {
			t.Errorf("第 %d 行 book = %q, 期望 %q", i+1, got, want)
		}
	}
}

// TestProcessRowMissingFields 验证缺少 fields 时返回明确错误。
func TestProcessRowMissingFields(t *testing.T) {
	ctx := NewContext(nil)
	step := model.Step{Op: "row", Args: map[string]string{"book": "{{item.title}}"}}
	if err := runProcess(ctx, step); err == nil {
		t.Error("缺少 fields 应返回错误")
	}
}
