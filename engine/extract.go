package engine

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
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
func cleanCSSValue(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

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
func htmlUnescapeBasic(s string) string {
	r := strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", "\"", "&#39;", "'", "&nbsp;", " ")
	return r.Replace(s)
}

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
type AssertDetail struct {
	StepName string `json:"stepName,omitempty"`
	StepID   string `json:"stepId,omitempty"`
	Name     string `json:"name"`
	Source   string `json:"source"`
	Op       string `json:"op"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Passed   bool   `json:"passed"`
	Detail   string `json:"detail"`
}

// applyAsserts 对响应执行断言。全部通过返回 nil（兼容旧调用）。
func applyAsserts(ctx *Context, resp *model.Response, asserts []model.Assert) error {
	_, err := applyAssertsDetailed(ctx, resp, asserts)
	return err
}

// applyAssertsDetailed 执行断言并返回每条的结构化结果。遇到第一条失败即返回。
func applyAssertsDetailed(ctx *Context, resp *model.Response, asserts []model.Assert) ([]AssertDetail, error) {
	details := make([]AssertDetail, 0, len(asserts))
	for _, a := range asserts {
		ok, detail, err := assertOne(ctx, resp, a)
		if err != nil {
			return details, err
		}
		name := a.Name
		if name == "" {
			name = a.Source
		}
		expected := ctx.Render(a.Value)
		details = append(details, AssertDetail{
			Name:     name,
			Source:   a.Source,
			Op:       a.Op,
			Expected: expected,
			Passed:   ok,
			Detail:   detail,
		})
		if !ok {
			// 从 detail 中解析实际值（detail 形如 实际="x", 期望 ...）
			details[len(details)-1].Actual = parseActual(detail)
			return details, fmt.Errorf("断言失败 [%s]: %s", name, detail)
		}
	}
	return details, nil
}

// parseActual 从断言详情文本中提取实际值（尽力而为，仅用于报告展示）。
func parseActual(detail string) string {
	if strings.HasPrefix(detail, "实际=") {
		if i := strings.Index(detail, ", 期望"); i > 0 {
			s := detail[len("实际="):i]
			return strings.Trim(s, `"`)
		}
	}
	return ""
}

func assertOne(ctx *Context, resp *model.Response, a model.Assert) (bool, string, error) {
	source := strings.ToLower(a.Source)
	var actual interface{}
	switch source {
	case "status":
		actual = resp.StatusCode
	case "header":
		for k, v := range resp.Headers {
			if strings.EqualFold(k, a.Path) && len(v) > 0 {
				actual = v[0]
				break
			}
		}
	case "variable":
		actual = ctx.Get(a.Path)
	case "jsonpath", "", "body":
		if !gjson.Valid(resp.Text) {
			actual = resp.Text
		} else if a.Path != "" {
			r := gjson.Get(resp.Text, a.Path)
			if r.Exists() {
				actual = jsonValue(r)
			}
		} else {
			actual = resp.Text
		}
	default:
		actual = resp.Text
	}

	op := strings.ToLower(a.Op)
	if op == "" {
		op = "eq"
	}
	expected := ctx.Render(a.Value)
	actualStr := toStr(actual)

	var ok bool
	switch op {
	case "eq":
		ok = actualStr == expected
	case "ne":
		ok = actualStr != expected
	case "contains":
		ok = strings.Contains(actualStr, expected)
	case "exists":
		ok = actual != nil && actualStr != ""
	case "gt", "gte", "lt", "lte":
		af, aok := toFloat(actual)
		ef, eok := toFloat(expected)
		if !aok || !eok {
			return false, fmt.Sprintf("无法比较数值: %q 与 %q", actualStr, expected), nil
		}
		switch op {
		case "gt":
			ok = af > ef
		case "gte":
			ok = af >= ef
		case "lt":
			ok = af < ef
		case "lte":
			ok = af <= ef
		}
	default:
		return false, fmt.Sprintf("未知断言操作 %q", op), nil
	}
	detail := fmt.Sprintf("实际=%q, 期望 %s %q", actualStr, op, expected)
	return ok, detail, nil
}
