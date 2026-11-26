package engine

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func processFilter(ctx *Context, args map[string]string) error {
	get := func(k string) string { return ctx.Render(args[k]) }
	list, ok := toSlice(ctx.Get(get("from")))
	if !ok {
		return fmt.Errorf("filter: %s 不是数组", get("from"))
	}
	field := get("field")
	op := strings.ToLower(get("op"))
	if op == "" {
		op = "eq"
	}
	want := get("value")
	out := get("to")
	if out == "" {
		out = "filtered"
	}
	result := make([]interface{}, 0)
	for _, item := range list {
		val := toStr(fieldValue(item, field))
		matched := false
		switch op {
		case "eq":
			matched = val == want
		case "ne":
			matched = val != want
		case "contains":
			matched = strings.Contains(val, want)
		case "gt", "gte", "lt", "lte":
			af, aok := toFloat(val)
			ef, eok := toFloat(want)
			if aok && eok {
				switch op {
				case "gt":
					matched = af > ef
				case "gte":
					matched = af >= ef
				case "lt":
					matched = af < ef
				case "lte":
					matched = af <= ef
				}
			}
		}
		if matched {
			result = append(result, item)
		}
	}
	ctx.Set(out, result)
	return nil
}

func processSort(ctx *Context, args map[string]string) error {
	get := func(k string) string { return ctx.Render(args[k]) }
	list, ok := toSlice(ctx.Get(get("from")))
	if !ok {
		return fmt.Errorf("sort: %s 不是数组", get("from"))
	}
	field := get("field")
	desc := strings.EqualFold(get("order"), "desc")
	out := get("to")
	if out == "" {
		out = "sorted"
	}
	sorted := append([]interface{}(nil), list...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := fieldValue(sorted[i], field), fieldValue(sorted[j], field)
		af, aok := toFloat(a)
		bf, bok := toFloat(b)
		var less bool
		if aok && bok {
			less = af < bf
		} else {
			less = toStr(a) < toStr(b)
		}
		if desc {
			return !less
		}
		return less
	})
	ctx.Set(out, sorted)
	return nil
}

func processPick(ctx *Context, args map[string]string) error {
	get := func(k string) string { return ctx.Render(args[k]) }
	list, ok := toSlice(ctx.Get(get("from")))
	if !ok {
		return fmt.Errorf("pick: %s 不是数组", get("from"))
	}
	fields := strings.Split(get("fields"), ",")
	out := get("to")
	if out == "" {
		out = "picked"
	}
	result := make([]interface{}, 0, len(list))
	for _, item := range list {
		row := make(map[string]interface{})
		for _, f := range fields {
			f = strings.TrimSpace(f)
			row[f] = fieldValue(item, f)
		}
		result = append(result, row)
	}
	ctx.Set(out, result)
	return nil
}

// runScript 用 goja 执行自定义 JavaScript，处理 Server 响应。
// 注入：vars（全部变量，可读写）、last（最近一次响应）、log()、http 内置已渲染。

func toSlice(v interface{}) ([]interface{}, bool) {
	switch x := v.(type) {
	case []interface{}:
		return x, true
	case []map[string]interface{}:
		out := make([]interface{}, 0, len(x))
		for _, m := range x {
			out = append(out, m)
		}
		return out, true
	case string:
		var parsed interface{}
		if json.Unmarshal([]byte(x), &parsed) == nil {
			if list, ok := parsed.([]interface{}); ok {
				return list, true
			}
		}
		// 普通字符串包装成单元素数组，便于标量累积
		return []interface{}{x}, true
	default:
		// 标量（数字/布尔等）包装成单元素数组
		if v != nil {
			return []interface{}{v}, true
		}
	}
	return nil, false
}

// flattenDeep 递归展平嵌套数组：[[a,b],[c]] → [a,b,c]。
// 非数组元素（含 map 行对象）原样保留，避免把表格行拆散。
func flattenDeep(list []interface{}) []interface{} {
	out := make([]interface{}, 0, len(list))
	for _, item := range list {
		switch x := item.(type) {
		case []interface{}:
			out = append(out, flattenDeep(x)...)
		case []string:
			for _, s := range x {
				out = append(out, s)
			}
		case []map[string]interface{}:
			for _, m := range x {
				out = append(out, m)
			}
		default:
			out = append(out, item)
		}
	}
	return out
}

func fieldValue(item interface{}, field string) interface{} {
	switch x := item.(type) {
	case map[string]interface{}:
		if v, ok := x[field]; ok {
			return v
		}
	case map[string]string:
		if v, ok := x[field]; ok {
			return v
		}
	}
	return nil
}
