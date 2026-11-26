package ui

import (
	"bytes"
	"io"
	"strings"

	"golang.org/x/net/html"
)

// htmlVoidElements 无结束标签的 HTML 元素，不增加缩进层级。
var htmlVoidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}

// htmlImplicitClose 可省略结束标签的元素：遇到同名开始标签时先闭合前一个（如 <li>a<li>b）。
var htmlImplicitClose = map[string]bool{
	"li": true, "dt": true, "dd": true, "p": true, "tr": true,
	"td": true, "th": true, "option": true, "thead": true, "tbody": true,
	"tfoot": true, "colgroup": true,
}

// htmlTagHints 判断文本更可能是 HTML 而非严格 XML 的标签特征。
var htmlTagHints = []string{
	"html", "head", "body", "div", "span", "p", "a", "ul", "ol", "li",
	"table", "thead", "tbody", "tr", "td", "th", "form", "input", "button",
	"img", "br", "hr", "meta", "link", "script", "style", "title",
	"section", "article", "nav", "header", "footer", "main", "aside",
	"label", "select", "option", "textarea", "iframe", "video", "audio", "source",
	"h1", "h2", "h3", "h4", "h5", "h6",
}

// looksLikeHTML 依据 DOCTYPE / 常见 HTML 标签判断是否为 HTML（否则交给严格 XML 解析器）。
func looksLikeHTML(s string) bool {
	lower := strings.ToLower(s)
	if strings.Contains(lower, "<!doctype") || strings.Contains(lower, "<script") || strings.Contains(lower, "<style") {
		return true
	}
	for _, tag := range htmlTagHints {
		if strings.Contains(lower, "<"+tag) || strings.Contains(lower, "</"+tag) {
			return true
		}
	}
	return false
}

// indentHTML 用 x/net/html 的容错 tokenizer 美化 HTML：仅插入换行与缩进，
// 不改写标签大小写、属性写法与实体（&nbsp; 等原样保留），对未闭合标签容错。
func indentHTML(s string) (string, bool) {
	if !strings.HasPrefix(strings.TrimSpace(s), "<") {
		return "", false
	}
	z := html.NewTokenizer(strings.NewReader(s))
	var out bytes.Buffer
	stack := make([]string, 0, 16)
	sawMarkup := false

	// writeLine 把一段原始 token 按当前缩进层级写入，空段跳过。
	writeLine := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		if out.Len() > 0 {
			out.WriteByte('\n')
			out.WriteString(strings.Repeat("  ", len(stack)))
		}
		out.WriteString(raw)
	}

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if z.Err() != io.EOF {
				return "", false // 非正常结束：视为无法格式化
			}
			goto done
		case html.DoctypeToken, html.CommentToken, html.TextToken:
			sawMarkup = sawMarkup || tt != html.TextToken
			writeLine(string(z.Raw())) // Raw 保留实体与原文写法
		case html.StartTagToken, html.SelfClosingTagToken, html.EndTagToken:
			nameB, _ := z.TagName()
			name := strings.ToLower(string(nameB))
			sawMarkup = true
			if tt == html.StartTagToken && htmlImplicitClose[name] && len(stack) > 0 && stack[len(stack)-1] == name {
				stack = stack[:len(stack)-1]
				writeLine("</" + name + ">")
			}
			switch tt {
			case html.StartTagToken, html.SelfClosingTagToken:
				writeLine(string(z.Raw()))
				if tt == html.StartTagToken && !htmlVoidElements[name] {
					stack = append(stack, name)
				}
			case html.EndTagToken:
				// 先弹出栈中到匹配标签为止，并为被跳过的中间标签补结束标签，
				// 使结束标签与其开始标签处于同一缩进层级。
				idx := -1
				for i := len(stack) - 1; i >= 0; i-- {
					if stack[i] == name {
						idx = i
						break
					}
				}
				if idx >= 0 {
					for len(stack)-1 > idx {
						top := stack[len(stack)-1]
						stack = stack[:len(stack)-1]
						writeLine("</" + top + ">")
					}
					stack = stack[:idx]
				}
				writeLine(string(z.Raw()))
			}
		}
	}
done:
	if !sawMarkup || out.Len() == 0 {
		return "", false
	}
	out.WriteByte('\n')
	result := out.String()
	if result == s {
		return s, true
	}
	return result, true
}
