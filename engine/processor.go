package engine

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
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

	case "zip": // 多数组按索引合成对象数组：args[from]="titles,links", args[fields]="title,url", args[to]
		return processZip(ctx, args)

	case "row": // 循环内把多字段组装成一行对象并追加到数组：args[fields], args[字段名]=表达式, args[to]
		return processRow(ctx, args)

	case "limit", "take", "slice": // 取数组区间：args[from], args[start]（默认0）, args[count]（N）, args[end]（可选）, args[to]
		return processLimit(ctx, args, op)

	case "resolve_url": // URL 绝对化 + 跳转链接解码：args[from], args[base]（可选）, args[to]
		v := ctx.resolveArg(args["from"])
		base := get("base")
		ctx.Set(get("to"), resolveURLValue(v, base))
		return nil

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

// processZip 把多个数组按索引合成对象数组，避免手写 JS。
// args: from=逗号分隔的多个数组变量名, fields=逗号分隔的输出字段名（与 from 一一对应）,
// to=目标变量（默认 zipped）。任一数组较短时该字段留空，以最长的为准。

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
