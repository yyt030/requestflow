package engine

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// processZip 把多个数组按索引合成对象数组，避免手写 JS。
// args: from=逗号分隔的多个数组变量名, fields=逗号分隔的输出字段名（与 from 一一对应）,
// to=目标变量（默认 zipped）。任一数组较短时该字段留空，以最长的为准。
func processZip(ctx *Context, args map[string]string) error {
	get := func(k string) string { return ctx.Render(args[k]) }
	names := strings.Split(get("from"), ",")
	fields := strings.Split(get("fields"), ",")
	out := get("to")
	if out == "" {
		out = "zipped"
	}
	listOfLists := make([][]interface{}, 0, len(names))
	maxLen := 0
	for _, n := range names {
		list, ok := toSlice(ctx.resolveArg(strings.TrimSpace(n)))
		if !ok {
			list = nil
		}
		listOfLists = append(listOfLists, list)
		if len(list) > maxLen {
			maxLen = len(list)
		}
	}
	result := make([]interface{}, 0, maxLen)
	for i := 0; i < maxLen; i++ {
		row := make(map[string]interface{})
		for j, list := range listOfLists {
			key := "col" + fmt.Sprint(j)
			if j < len(fields) && strings.TrimSpace(fields[j]) != "" {
				key = strings.TrimSpace(fields[j])
			}
			if i < len(list) {
				row[key] = list[i]
			} else {
				row[key] = ""
			}
		}
		result = append(result, row)
	}
	ctx.Set(out, result)
	return nil
}

// processLimit 取数组区间：start 起、count 条，或 start..end。避免手写 slice JS。
// args: from, start（默认0）, count（条数）, end（结束下标，含 count 时忽略）, to。

// processLimit 取数组区间：start 起、count 条，或 start..end。避免手写 slice JS。
// args: from, start（默认0）, count（条数）, end（结束下标，含 count 时忽略）, to。
func processLimit(ctx *Context, args map[string]string, op string) error {
	get := func(k string) string { return ctx.Render(args[k]) }
	list, ok := toSlice(ctx.resolveArg(args["from"]))
	if !ok {
		return fmt.Errorf("%s: %s 不是数组", op, get("from"))
	}
	start := 0
	if s := get("start"); s != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && n > 0 {
			start = n
		}
	}
	if start > len(list) {
		start = len(list)
	}
	end := len(list)
	if s := get("end"); s != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && n >= start && n <= len(list) {
			end = n
		}
	} else if s := get("count"); s != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && n >= 0 {
			end = start + n
			if end > len(list) {
				end = len(list)
			}
		}
	}
	out := get("to")
	if out == "" {
		out = "limited"
	}
	ctx.Set(out, list[start:end])
	return nil
}

// processDedup 数组去重：按 field（可选）或整个元素去重，保持原顺序。
// args: from=源数组变量, field=可选去重字段（对象数组按此字段值去重）,
// to=目标变量（默认 deduped）, stat=可选，写入被去掉的重复数量变量名。

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
