package engine

import (
	"fmt"
	"strings"
)

// processRow 在循环的一轮迭代内把多个字段组装成一个对象行，并追加到目标数组。
// 用于替代「多个 set + 多个 append + 循环外 zip」的冗长写法，把跨站采集的
// 「逐项补充字段 → 合成宽表」从十几步压缩为一步。
//
// args:
//   - fields: 逗号分隔的输出字段名，如 "book,rating,title1,link1"
//   - 每个字段名作为独立 key，值为取值表达式，如 book="{{item.title}}"
//     （支持 {{...}} 表达式、点路径、字面量；用 resolveArg 求值，保留原始类型）
//   - to: 累积到的数组变量名（默认 rows）
//
// 每执行一次，追加一行；循环结束后 to 即为对象数组，可直接 export。
func processRow(ctx *Context, args map[string]string) error {
	get := func(k string) string { return ctx.Render(args[k]) }

	fieldsStr := get("fields")
	if strings.TrimSpace(fieldsStr) == "" {
		return fmt.Errorf("row: 缺少 fields（逗号分隔的字段名）")
	}
	out := get("to")
	if out == "" {
		out = "rows"
	}

	row := make(map[string]interface{})
	for _, raw := range strings.Split(fieldsStr, ",") {
		field := strings.TrimSpace(raw)
		if field == "" || field == "fields" || field == "to" {
			continue
		}
		row[field] = ctx.resolveArg(args[field])
	}

	var acc []interface{}
	if existing, ok := toSlice(ctx.Get(out)); ok {
		acc = existing
	}
	acc = append(acc, row)
	ctx.Set(out, acc)
	return nil
}
