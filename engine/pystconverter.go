package engine

import (
	"encoding/json"
	"fmt"

	"go.starlark.net/starlark"
)

// toStarlark 把 Go 值转换为 Starlark 值（脚本注入用）。
func toStarlark(v interface{}) starlark.Value {
	switch x := v.(type) {
	case nil:
		return starlark.None
	case string:
		return starlark.String(x)
	case bool:
		return starlark.Bool(x)
	case int:
		return starlark.MakeInt(x)
	case int64:
		return starlark.MakeInt64(x)
	case float64:
		return starlark.Float(x)
	case float32:
		return starlark.Float(float64(x))
	case []interface{}:
		items := make([]starlark.Value, 0, len(x))
		for _, e := range x {
			items = append(items, toStarlark(e))
		}
		return starlark.NewList(items)
	case []string:
		items := make([]starlark.Value, 0, len(x))
		for _, e := range x {
			items = append(items, starlark.String(e))
		}
		return starlark.NewList(items)
	case map[string]interface{}:
		d := starlark.NewDict(len(x))
		for k, e := range x {
			d.SetKey(starlark.String(k), toStarlark(e))
		}
		return d
	case map[string]string:
		d := starlark.NewDict(len(x))
		for k, e := range x {
			d.SetKey(starlark.String(k), starlark.String(e))
		}
		return d
	default:
		// 其它类型（如自定义结构）先 JSON 化再转，尽量不丢数据。
		b, err := json.Marshal(x)
		if err != nil {
			return starlark.String(fmt.Sprint(x))
		}
		var generic interface{}
		if err := json.Unmarshal(b, &generic); err != nil {
			return starlark.String(string(b))
		}
		return toStarlark(generic)
	}
}

// fromStarlark 把 Starlark 值转换回 Go 值（脚本结果写回用）。
func fromStarlark(v starlark.Value) interface{} {
	switch x := v.(type) {
	case starlark.NoneType:
		return nil
	case starlark.String:
		return string(x)
	case starlark.Bool:
		return bool(x)
	case starlark.Int:
		if n, ok := x.Int64(); ok {
			return n
		}
		return x.String()
	case starlark.Float:
		return float64(x)
	case *starlark.List:
		out := make([]interface{}, 0, x.Len())
		it := x.Iterate()
		var el starlark.Value
		for it.Next(&el) {
			out = append(out, fromStarlark(el))
		}
		return out
	case starlark.Tuple:
		out := make([]interface{}, 0, len(x))
		for _, el := range x {
			out = append(out, fromStarlark(el))
		}
		return out
	case *starlark.Dict:
		out := make(map[string]interface{}, x.Len())
		for _, key := range x.Keys() {
			val, _, _ := x.Get(key)
			k := key.String()
			if sk, ok := key.(starlark.String); ok {
				k = string(sk)
			}
			out[k] = fromStarlark(val)
		}
		return out
	case *starlark.Set:
		out := make([]interface{}, 0, x.Len())
		it := x.Iterate()
		var el starlark.Value
		for it.Next(&el) {
			out = append(out, fromStarlark(el))
		}
		return out
	default:
		return v.String()
	}
}

// pyDisplay 让 log() 打印 Python 风格：字符串不带引号，容器可读。
func pyDisplay(v starlark.Value) string {
	if s, ok := v.(starlark.String); ok {
		return string(s)
	}
	return v.String()
}

func jsonString(v starlark.Value) string {
	b, err := json.Marshal(fromStarlark(v))
	if err != nil {
		return ""
	}
	return string(b)
}
