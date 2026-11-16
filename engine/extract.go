package engine

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

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

// applyAsserts 对响应执行断言。全部通过返回 nil。
func applyAsserts(ctx *Context, resp *model.Response, asserts []model.Assert) error {
	for _, a := range asserts {
		ok, detail, err := assertOne(ctx, resp, a)
		if err != nil {
			return err
		}
		if !ok {
			name := a.Name
			if name == "" {
				name = a.Source
			}
			return fmt.Errorf("断言失败 [%s]: %s", name, detail)
		}
	}
	return nil
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
