package engine

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/dop251/goja"

	"requestflow/model"
)

// runProcess 执行内置数据处理逻辑。
// 数据以变量形式在步骤间流转，输出写回变量，供后续步骤或导出使用。
func runProcess(ctx *Context, step model.Step) error {
	op := strings.ToLower(step.Op)
	args := step.Args
	get := func(k string) string { return ctx.Render(args[k]) }

	switch op {
	case "set": // 直接设置一个变量：args[name]=value
		ctx.Set(get("name"), get("value"))
		return nil

	case "json_to_table": // 把 JSON 数组变量解析为表格 []map
		src := get("from")
		out := get("to")
		if out == "" {
			out = "table"
		}
		var rows interface{} = ctx.Get(src)
		if s, ok := rows.(string); ok {
			if err := json.Unmarshal([]byte(s), &rows); err != nil {
				return fmt.Errorf("json_to_table 解析 %s 失败: %w", src, err)
			}
		}
		list, ok := toSlice(rows)
		if !ok {
			return fmt.Errorf("json_to_table: %s 不是数组", src)
		}
		ctx.Set(out, list)
		return nil

	case "filter": // 过滤数组：args[from], args[field], args[op], args[value], args[to]
		return processFilter(ctx, args)

	case "sort": // 数组排序
		return processSort(ctx, args)

	case "pick": // 提取对象的某些字段组成新数组/对象
		return processPick(ctx, args)

	case "append": // 把 from 变量的值追加到 to 数组变量（常用于循环内累积）
		to := get("to")
		if to == "" {
			to = "rows"
		}
		val := ctx.Get(get("from"))
		var acc []interface{}
		if existing, ok := toSlice(ctx.Get(to)); ok {
			acc = existing
		}
		acc = append(acc, val)
		ctx.Set(to, acc)
		return nil

	case "flatten": // 将多个数组变量合并为一个
		names := strings.Split(get("from"), ",")
		out := get("to")
		if out == "" {
			out = "table"
		}
		var merged []interface{}
		for _, n := range names {
			list, ok := toSlice(ctx.Get(strings.TrimSpace(n)))
			if ok {
				merged = append(merged, list...)
			}
		}
		ctx.Set(out, merged)
		return nil

	case "format_time": // 时间戳格式化：args[from], args[to], args[layout], args[in] = unix|unixms
		v := ctx.Get(get("from"))
		unit := strings.ToLower(get("in"))
		var t time.Time
		switch n := v.(type) {
		case float64:
			t = timeFromNumber(n, unit)
		case int:
			t = timeFromNumber(float64(n), unit)
		case int64:
			t = timeFromNumber(float64(n), unit)
		case string:
			f, ok := toFloat(n)
			if ok {
				t = timeFromNumber(f, unit)
			} else if parsed, err := time.Parse(time.RFC3339, n); err == nil {
				t = parsed
			}
		}
		layout := get("layout")
		if layout == "" {
			layout = "2006-01-02 15:04:05"
		}
		out := get("to")
		if out == "" {
			out = get("from")
		}
		ctx.Set(out, t.Format(layout))
		return nil

	case "dedup": // 数组去重：args[from], args[field]（可选）, args[to], args[stat]（可选）
		return processDedup(ctx, args)

	case "base64encode":
		outName := get("to")
		if outName == "" {
			outName = "encoded"
		}
		v := ctx.resolveArg(args["from"])
		if list, ok := rawSlice(v); ok {
			out := make([]interface{}, 0, len(list))
			for _, it := range list {
				out = append(out, base64.StdEncoding.EncodeToString([]byte(toStr(it))))
			}
			ctx.Set(outName, out)
		} else {
			ctx.Set(outName, base64.StdEncoding.EncodeToString([]byte(toStr(v))))
		}
		return nil
	case "base64decode":
		outName := get("to")
		if outName == "" {
			outName = "decoded"
		}
		v := ctx.resolveArg(args["from"])
		if list, ok := rawSlice(v); ok {
			out := make([]interface{}, 0, len(list))
			for _, it := range list {
				s, err := decodeBase64Flexible(it)
				if err != nil {
					return err
				}
				out = append(out, s)
			}
			ctx.Set(outName, out)
		} else {
			s, err := decodeBase64Flexible(v)
			if err != nil {
				return err
			}
			ctx.Set(outName, s)
		}
		return nil
	case "urlencode":
		ctx.Set(get("to"), url.QueryEscape(get("from")))
		return nil
	case "urldecode":
		s, err := url.QueryUnescape(get("from"))
		if err != nil {
			return err
		}
		ctx.Set(get("to"), s)
		return nil
	case "join": // 拼接字符串，args[values]="a,b,c", args[sep]
		parts := strings.Split(get("values"), ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		ctx.Set(get("to"), strings.Join(parts, get("sep")))
		return nil
	}
	return fmt.Errorf("未知的内置处理 op: %q", step.Op)
}

// processDedup 数组去重：按 field（可选）或整个元素去重，保持原顺序。
// args: from=源数组变量, field=可选去重字段（对象数组按此字段值去重）,
// to=目标变量（默认 deduped）, stat=可选，写入被去掉的重复数量变量名。
func processDedup(ctx *Context, args map[string]string) error {
	get := func(k string) string { return ctx.Render(args[k]) }
	list, ok := toSlice(ctx.resolveArg(args["from"]))
	if !ok {
		return fmt.Errorf("dedup: %s 不是数组", get("from"))
	}
	field := get("field")
	out := get("to")
	if out == "" {
		out = "deduped"
	}
	deduped, removed := dedupByField(list, field)
	ctx.Set(out, deduped)
	if stat := get("stat"); stat != "" {
		ctx.Set(stat, removed)
	}
	return nil
}

// dedupByField 保持顺序去重；field 为空时用元素文本作为键，否则用元素该字段的值。
// 返回去重后的数组与被去掉的重复元素数量。
func dedupByField(list []interface{}, field string) ([]interface{}, int) {
	seen := make(map[string]struct{}, len(list))
	out := make([]interface{}, 0, len(list))
	removed := 0
	for _, item := range list {
		v := item
		if field != "" {
			v = fieldValue(item, field)
		}
		key := dedupKey(v)
		if _, exists := seen[key]; exists {
			removed++
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out, removed
}

// dedupKey 把任意值转成稳定的去重键。map/切片用 JSON 序列化保证字段顺序稳定。
func dedupKey(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return "<nil>"
	case string:
		return "s:" + x
	case map[string]interface{}, []interface{}:
		b, err := json.Marshal(x)
		if err == nil {
			return "j:" + string(b)
		}
	}
	return fmt.Sprintf("%T:%v", v, v)
}

// decodeBase64Flexible 宽容解码 Base64：依次尝试标准（带 padding）、标准无 padding、
// URL-safe（带/无 padding）。兼容 Bing 跳转链接等无 padding 的 base64url 场景。
func decodeBase64Flexible(v interface{}) (string, error) {
	s := strings.TrimSpace(toStr(v))
	if s == "" {
		return "", nil
	}
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	var lastErr error
	for _, enc := range encodings {
		if b, err := enc.DecodeString(s); err == nil {
			return string(b), nil
		} else {
			lastErr = err
		}
	}
	return "", lastErr
}

// rawSlice 把「本身就是数组」的值转为 []interface{}；标量与普通字符串返回 false。
// 与 toSlice 不同：不会把标量或 JSON 字符串包装成单元素数组，用于区分「数组输入」与「标量输入」。
func rawSlice(v interface{}) ([]interface{}, bool) {
	switch x := v.(type) {
	case []interface{}:
		return x, true
	case []string:
		out := make([]interface{}, 0, len(x))
		for _, s := range x {
			out = append(out, s)
		}
		return out, true
	case []map[string]interface{}:
		out := make([]interface{}, 0, len(x))
		for _, m := range x {
			out = append(out, m)
		}
		return out, true
	case string:
		// 变量里常存的是 JSON 数组字符串（如 set 的结果），也视为数组输入。
		var parsed interface{}
		if json.Unmarshal([]byte(strings.TrimSpace(x)), &parsed) == nil {
			if list, ok := parsed.([]interface{}); ok {
				return list, true
			}
		}
	}
	return nil, false
}

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
func runScript(ctx *Context, step model.Step, last *model.Response) error {
	vm := goja.New()
	snapshot := ctx.Snapshot()
	vm.Set("vars", snapshot)
	if last != nil {
		vm.Set("last", responseView(last))
	}
	vm.Set("log", func(args ...interface{}) {
		parts := make([]string, 0, len(args))
		for _, a := range args {
			parts = append(parts, toStr(a))
		}
		fmt.Println("[script]", strings.Join(parts, " "))
	})
	vm.Set("atob", func(s string) (string, error) {
		return decodeBase64Flexible(s)
	})
	vm.Set("btoa", func(s string) string {
		return base64.StdEncoding.EncodeToString([]byte(s))
	})
	vm.Set("JSON", map[string]interface{}{
		"stringify": func(v interface{}) string {
			b, _ := json.Marshal(v)
			return string(b)
		},
		"parse": func(s string) interface{} {
			var v interface{}
			_ = json.Unmarshal([]byte(s), &v)
			return v
		},
	})

	code := ctx.Render(step.Code)
	if _, err := vm.RunString(code); err != nil {
		return fmt.Errorf("脚本执行失败: %w", err)
	}
	// 将 vars 的改动写回 context
	for k, v := range snapshot {
		ctx.Set(k, v)
	}
	return nil
}

func timeFromNumber(n float64, unit string) time.Time {
	if unit == "unixms" {
		return time.UnixMilli(int64(n))
	}
	return time.Unix(int64(n), 0)
}

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
