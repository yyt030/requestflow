package engine

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/tidwall/gjson"

	"requestflow/model"
)

// applyExtracts 从响应中提取变量写入 context。
func applyExtracts(ctx *Context, resp *model.Response, extracts []model.Extract) error {
	for _, ex := range extracts {
		if ex.Name == "" {
			continue
		}
		value, err := extractOne(ctx, resp, ex)
		if err != nil {
			return fmt.Errorf("提取变量 %s: %w", ex.Name, err)
		}
		if value == nil || toStr(value) == "" {
			if ex.Default != "" {
				value = ex.Default
			}
		}
		// expectMin：提取条数不足即报错，避免反爬页/抓空页被当成成功。
		if ex.ExpectMin > 0 {
			n := extractedCount(value)
			if n < ex.ExpectMin {
				return fmt.Errorf("提取变量 %s: 仅得到 %d 条，低于 expectMin=%d（可能是反爬页或抓空页）", ex.Name, n, ex.ExpectMin)
			}
		}
		ctx.Set(ex.Name, value)
	}
	return nil
}

func extractOne(ctx *Context, resp *model.Response, ex model.Extract) (interface{}, error) {
	source := strings.ToLower(ex.Source)
	kind := strings.ToLower(ex.Kind)
	switch source {
	case "status":
		return resp.StatusCode, nil
	case "header":
		for k, v := range resp.Headers {
			if strings.EqualFold(k, ex.Path) && len(v) > 0 {
				return v[0], nil
			}
		}
		return nil, nil
	case "cookie":
		for _, c := range resp.Cookies {
			if c.Name == ex.Path {
				return c.Value, nil
			}
		}
		return nil, nil
	default: // body
		switch kind {
		case "whole":
			return resp.Text, nil
		case "regex":
			re, err := regexp.Compile(ex.Path)
			if err != nil {
				return nil, err
			}
			m := re.FindStringSubmatch(resp.Text)
			if m == nil {
				return nil, nil
			}
			idx := ex.Group
			if idx < 0 || idx >= len(m) {
				idx = 0
			}
			return m[idx], nil
		case "css":
			v, err := extractCSS(ctx.Render(ex.Path), resp.Text, ex.CssField, ex.CssAttr)
			if err != nil || v == nil {
				return v, err
			}
			if ex.ResolveURL {
				v = resolveURLValue(v, urlBaseFor(resp, ctx))
			}
			return v, nil
		case "cssrows":
			return extractCSSRows(ctx, resp, ex)
		default: // jsonpath
			path := ctx.Render(ex.Path)
			if path == "" {
				path = ex.Name
			}
			if !gjson.Valid(resp.Text) {
				return nil, nil
			}
			r := gjson.Get(resp.Text, path)
			if !r.Exists() {
				return nil, nil
			}
			return jsonValue(r), nil
		}
	}
}

// extractCSS 用 CSS 选择器从 HTML 提取。多元素统一返回 []interface{}，无匹配返回 nil。
// field：text（默认，元素文本）/ html（内部 HTML）/ attr（属性值，由 attrName 指定）。
// goquery 基于 x/net/html，会自动解码 HTML 实体（如 &nbsp;、&amp;）。

// extractCSS 用 CSS 选择器从 HTML 提取。多元素统一返回 []interface{}，无匹配返回 nil。
// field：text（默认，元素文本）/ html（内部 HTML）/ attr（属性值，由 attrName 指定）。
// goquery 基于 x/net/html，会自动解码 HTML 实体（如 &nbsp;、&amp;）。
func extractCSS(selector, html, field, attrName string) (interface{}, error) {
	if strings.TrimSpace(selector) == "" {
		return nil, fmt.Errorf("CSS 提取需要提供选择器")
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("CSS 提取解析 HTML 失败: %w", err)
	}
	field = strings.ToLower(strings.TrimSpace(field))
	if field == "" {
		field = "text"
	}
	out := make([]interface{}, 0)
	var matchErr error
	doc.Find(selector).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		var val string
		switch field {
		case "html":
			val, matchErr = s.Html()
		case "attr":
			if strings.TrimSpace(attrName) == "" {
				matchErr = fmt.Errorf("CSS 提取 field=attr 时需填写属性名 cssAttr")
				return false
			}
			v, ok := s.Attr(attrName)
			if !ok {
				return true // 该元素无此属性，跳过
			}
			val = v
		default: // text
			val = s.Text()
		}
		if matchErr != nil {
			return false
		}
		val = cleanCSSValue(val)
		if val == "" {
			return true // 跳过空值，避免空元素污染数组
		}
		out = append(out, val)
		return true
	})
	if matchErr != nil {
		return nil, matchErr
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// cleanCSSValue 收敛空白：合并连续空白（含 &nbsp; 解码后的 \u00a0）并去除首尾空白。

// cleanCSSValue 收敛空白：合并连续空白（含 &nbsp; 解码后的 \u00a0）并去除首尾空白。
func cleanCSSValue(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// extractCSSRows 结构化行提取：按 Path 选中所有行，每行内按 CssFields 提取子字段，
// 直接产出 []map[string]interface{}。从机制上保证同一行的字段来自同一元素，杜绝多选择器 zip 错位。

func jsonValue(r gjson.Result) interface{} {
	switch r.Type {
	case gjson.Number:
		return r.Float()
	case gjson.True:
		return true
	case gjson.False:
		return false
	case gjson.Null:
		return nil
	case gjson.JSON:
		var v interface{}
		if json.Unmarshal([]byte(r.Raw), &v) == nil {
			return v
		}
		return r.Raw
	default:
		return r.String()
	}
}

// AssertDetail 单个断言的结构化结果，供测试报告使用。
