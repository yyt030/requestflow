package engine

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"requestflow/model"
)

// extractCSSRows 结构化行提取：按 Path 选中所有行，每行内按 CssFields 提取子字段，
// 直接产出 []map[string]interface{}。从机制上保证同一行的字段来自同一元素，杜绝多选择器 zip 错位。
func extractCSSRows(ctx *Context, resp *model.Response, ex model.Extract) (interface{}, error) {
	selector := ctx.Render(ex.Path)
	if strings.TrimSpace(selector) == "" {
		return nil, fmt.Errorf("cssRows 提取需要提供行选择器（path）")
	}
	if len(ex.CssFields) == 0 {
		return nil, fmt.Errorf("cssRows 提取需要提供 cssFields 子字段配置")
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(resp.Text))
	if err != nil {
		return nil, fmt.Errorf("cssRows 解析 HTML 失败: %w", err)
	}
	base := urlBaseFor(resp, ctx)
	out := make([]interface{}, 0)
	var fldErr error
	doc.Find(selector).EachWithBreak(func(_ int, row *goquery.Selection) bool {
		obj := make(map[string]interface{})
		for _, f := range ex.CssFields {
			if f.Name == "" {
				continue
			}
			val, ok := cssFieldValue(row, f)
			if fldErr != nil {
				return false
			}
			if !ok || val == "" {
				obj[f.Name] = ""
				continue
			}
			if f.Resolve {
				val = resolveURLOne(val, base)
			}
			obj[f.Name] = val
		}
		out = append(out, obj)
		return true
	})
	if fldErr != nil {
		return nil, fldErr
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// cssFieldValue 在行元素内提取单个子字段。f.Path 为空时取行元素自身。

// cssFieldValue 在行元素内提取单个子字段。f.Path 为空时取行元素自身。
func cssFieldValue(row *goquery.Selection, f model.CSSField) (string, bool) {
	sel := row
	path := strings.TrimSpace(f.Path)
	if path != "" {
		sel = row.Find(path)
		if sel.Length() == 0 {
			return "", false
		}
	}
	field := strings.ToLower(strings.TrimSpace(f.CssField))
	if field == "" {
		field = "text"
	}
	var val string
	switch field {
	case "html":
		v, err := sel.First().Html()
		if err != nil {
			return "", false
		}
		val = v
	case "attr":
		if strings.TrimSpace(f.CssAttr) == "" {
			return "", false
		}
		v, ok := sel.First().Attr(f.CssAttr)
		if !ok {
			return "", false
		}
		val = v
	default:
		val = sel.First().Text()
	}
	return cleanCSSValue(val), true
}

// urlBaseFor 计算 URL 绝对化的基准：优先 ctx 变量 baseUrl，其次响应里的 <base href>，都没有返回空。

// urlBaseFor 计算 URL 绝对化的基准：优先 ctx 变量 baseUrl，其次响应里的 <base href>，都没有返回空。
func urlBaseFor(resp *model.Response, ctx *Context) string {
	if v := ctx.Get("baseUrl"); v != nil && toStr(v) != "" {
		return toStr(v)
	}
	if resp != nil && resp.Text != "" {
		if m := regexp.MustCompile(`(?i)<base\s+[^>]*href\s*=\s*["']([^"']+)["']`).FindStringSubmatch(resp.Text); len(m) == 2 {
			return m[1]
		}
	}
	return ""
}

// resolveURLValue 对提取值做 URL 处理：字符串数组逐项、标量单项。
// 处理内容：HTML 实体已在 cleanCSSValue/goquery 阶段解码；这里做相对地址绝对化与跳转链接解码。

// resolveURLValue 对提取值做 URL 处理：字符串数组逐项、标量单项。
// 处理内容：HTML 实体已在 cleanCSSValue/goquery 阶段解码；这里做相对地址绝对化与跳转链接解码。
func resolveURLValue(v interface{}, base string) interface{} {
	switch x := v.(type) {
	case []interface{}:
		out := make([]interface{}, 0, len(x))
		for _, it := range x {
			out = append(out, resolveURLOne(toStr(it), base))
		}
		return out
	case []string:
		out := make([]interface{}, 0, len(x))
		for _, it := range x {
			out = append(out, resolveURLOne(it, base))
		}
		return out
	default:
		return resolveURLOne(toStr(v), base)
	}
}

// resolveURLOne 单个 URL：先解码常见跳转链接，再按 base 绝对化相对地址。

// resolveURLOne 单个 URL：先解码常见跳转链接，再按 base 绝对化相对地址。
var (
	baiduLinkRE = regexp.MustCompile(`(?:^|https?://(?:www\.)?baidu\.com)/link\?url=([^&#]+)`)
	bingLinkRE  = regexp.MustCompile(`bing\.com/ck/a\?[^#]*?u=a1([A-Za-z0-9_\-]+)`)
)

func resolveURLOne(raw, base string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return s
	}
	// 解码 HTML 实体残留（goquery 已解一次，这里兜底 &amp; 等）。
	s = htmlUnescapeBasic(s)
	if m := baiduLinkRE.FindStringSubmatch(s); len(m) == 2 {
		if dec, err := base64.RawURLEncoding.DecodeString(m[1]); err == nil && strings.HasPrefix(string(dec), "http") {
			return string(dec)
		}
	}
	if m := bingLinkRE.FindStringSubmatch(s); len(m) == 2 {
		if dec, err := base64.RawURLEncoding.DecodeString(m[1]); err == nil {
			if u := strings.TrimPrefix(string(dec), "\x00"); strings.HasPrefix(u, "http") {
				return u
			}
		}
	}
	if base != "" && !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		if abs, err := url.Parse(base); err == nil {
			if rel, err := url.Parse(s); err == nil {
				return abs.ResolveReference(rel).String()
			}
		}
	}
	return s
}

// htmlUnescapeBasic 兜底解码少量 HTML 实体（goquery 通常已处理）。

// htmlUnescapeBasic 兜底解码少量 HTML 实体（goquery 通常已处理）。
func htmlUnescapeBasic(s string) string {
	r := strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", "\"", "&#39;", "'", "&nbsp;", " ")
	return r.Replace(s)
}

// extractedCount 统计提取结果的条数，供 expectMin 校验。标量按 1 计（nil 为 0）。

// extractedCount 统计提取结果的条数，供 expectMin 校验。标量按 1 计（nil 为 0）。
func extractedCount(v interface{}) int {
	if v == nil {
		return 0
	}
	if list, ok := rawSlice(v); ok {
		return len(list)
	}
	return 1
}
