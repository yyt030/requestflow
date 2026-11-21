package engine

import (
	"reflect"
	"testing"
)

const cssSampleHTML = `<!DOCTYPE html><html><body>
<ul class="grid">
  <li class="item"><a href="/s/1" title="回放">肖申克的救赎</a><span class="rating">9.7</span><span class="desc">经典 &nbsp; 电影 &amp; 必看</span></li>
  <li class="item"><a href="/s/2" title="热门">霸王别姬</a><span class="rating">9.6</span></li>
  <li class="other">肖申克的救赎（别名）</li>
</ul>
<div id="empty"></div>
</body></html>`

// TestExtractCSSText 多元素文本提取为数组，且只匹配选择器范围内的元素。
func TestExtractCSSText(t *testing.T) {
	got, err := extractCSS("li.item a", cssSampleHTML, "text", "")
	if err != nil {
		t.Fatal(err)
	}
	want := []interface{}{"肖申克的救赎", "霸王别姬"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CSS text = %#v, want %#v", got, want)
	}
}

// TestExtractCSSAttr 提取属性值数组，并跳过无该属性的元素。
func TestExtractCSSAttr(t *testing.T) {
	got, err := extractCSS("li.item a", cssSampleHTML, "attr", "href")
	if err != nil {
		t.Fatal(err)
	}
	want := []interface{}{"/s/1", "/s/2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CSS attr(href) = %#v, want %#v", got, want)
	}
	// 不存在的属性：所有元素都无此属性 → nil
	got, err = extractCSS("li.item a", cssSampleHTML, "attr", "data-missing")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("CSS attr(不存在) = %#v, want nil", got)
	}
	// field=attr 但未指定属性名 → 报错
	if _, err := extractCSS("li.item a", cssSampleHTML, "attr", ""); err == nil {
		t.Fatal("CSS attr 未指定属性名应报错")
	}
}

// TestExtractCSSHtml 提取内部 HTML。
func TestExtractCSSHtml(t *testing.T) {
	got, err := extractCSS("li.item a", cssSampleHTML, "html", "")
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := got.([]interface{})
	if !ok || len(arr) != 2 {
		t.Fatalf("CSS html 应返回 2 元素数组, got %#v", got)
	}
	if s, _ := arr[0].(string); s != "肖申克的救赎" {
		t.Fatalf("CSS html 第一项异常: %q", s)
	}
}

// TestExtractCSSEntityAndWhitespace 验证 HTML 实体解码与空白收敛（&nbsp; / &amp;）。
func TestExtractCSSEntityAndWhitespace(t *testing.T) {
	got, err := extractCSS("span.desc", cssSampleHTML, "text", "")
	if err != nil {
		t.Fatal(err)
	}
	want := []interface{}{"经典 电影 & 必看"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CSS 实体/空白处理 = %#v, want %#v", got, want)
	}
}

// TestExtractCSSEmpty 无匹配元素返回 nil；空选择器报错。
func TestExtractCSSEmpty(t *testing.T) {
	got, err := extractCSS("#empty", cssSampleHTML, "text", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("空元素应返回 nil, got %#v", got)
	}
	got, err = extractCSS(".not-exist", cssSampleHTML, "text", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("无匹配应返回 nil, got %#v", got)
	}
	if _, err := extractCSS("  ", cssSampleHTML, "text", ""); err == nil {
		t.Fatal("空选择器应报错")
	}
}
