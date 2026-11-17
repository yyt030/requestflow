package engine

import (
	"encoding/json"
	"fmt"
	"strings"

	"go.starlark.net/starlark"

	"requestflow/model"
)

// runPythonScript 用 Starlark（Python 语法子集，纯 Go 无 cgo）执行自定义脚本。
// 与 JS 引擎保持一致的注入：vars（读写变量）、last（最近响应）、log()、JSON。
// vars 通过 starlark.Dict 双向传递，脚本内用 vars["k"]=v 修改会写回上下文。
func runPythonScript(ctx *Context, step model.Step, last *model.Response) error {
	snapshot := ctx.Snapshot()

	vars := starlark.NewDict(len(snapshot))
	for k, v := range snapshot {
		vars.SetKey(starlark.String(k), toStarlark(v))
	}

	predeclared := starlark.StringDict{
		"vars": vars,
		"log": starlark.NewBuiltin("log", func(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kv []starlark.Tuple) (starlark.Value, error) {
			parts := make([]string, 0, len(args))
			for _, a := range args {
				parts = append(parts, pyDisplay(a))
			}
			fmt.Println("[script]", strings.Join(parts, " "))
			return starlark.None, nil
		}),
		"JSON": jsonModule(),
	}
	if last != nil {
		predeclared["last"] = toStarlark(responseView(last))
	}
	// 补足 Starlark universe 缺失的常用 Python 内置函数。
	for k, v := range pyBuiltins() {
		if _, exists := predeclared[k]; !exists {
			predeclared[k] = v
		}
	}

	code := ctx.Render(step.Code)
	thread := &starlark.Thread{Name: "requestflow-script"}
	if _, err := starlark.ExecFile(thread, "<script>", code, predeclared); err != nil {
		return fmt.Errorf("Python 脚本执行失败: %w", err)
	}

	// 把 vars 的改动写回 context（只写脚本里出现过的键，避免覆盖无关变量）。
	for _, key := range vars.Keys() {
		k, ok := key.(starlark.String)
		if !ok {
			continue
		}
		v, _, _ := vars.Get(key)
		ctx.Set(string(k), fromStarlark(v))
	}
	return nil
}

// jsonModule 返回 Python 脚本里的 JSON 工具对象（stringify / parse），
// 保持与 JS 引擎的 JSON.stringify / JSON.parse 用法一致。
func jsonModule() starlark.Value {
	d := starlark.NewDict(2)
	d.SetKey(starlark.String("stringify"), starlark.NewBuiltin("stringify", func(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kv []starlark.Tuple) (starlark.Value, error) {
		return starlark.String(jsonString(args[0])), nil
	}))
	d.SetKey(starlark.String("parse"), starlark.NewBuiltin("parse", func(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kv []starlark.Tuple) (starlark.Value, error) {
		if len(args) == 0 {
			return starlark.None, nil
		}
		s, ok := starlark.AsString(args[0])
		if !ok {
			return starlark.None, nil
		}
		var v interface{}
		if err := json.Unmarshal([]byte(s), &v); err != nil {
			return starlark.None, nil
		}
		return toStarlark(v), nil
	}))
	return d
}

// pyBuiltins 返回需要额外注入到 Starlark 全局的 Python 风格内置函数。
// Starlark 自带 len/range/str/int/float/list/dict/bool/sorted/min/max 等，
// 但缺 sum/any/all/abs/round 等，这里补齐常用部分。
func pyBuiltins() starlark.StringDict {
	return starlark.StringDict{
		"sum":   starlark.NewBuiltin("sum", pySum),
		"abs":   starlark.NewBuiltin("abs", pyAbs),
		"any":   starlark.NewBuiltin("any", pyAnyAll(false)),
		"all":   starlark.NewBuiltin("all", pyAnyAll(true)),
	}
}

func pySum(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kv []starlark.Tuple) (starlark.Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("sum() 缺少参数")
	}
	total := 0.0
	allInt := true
	seq, ok := args[0].(starlark.Sequence)
	if !ok {
		return nil, fmt.Errorf("sum() 需要序列")
	}
	it := seq.Iterate()
	var el starlark.Value
	for it.Next(&el) {
		switch x := el.(type) {
		case starlark.Int:
			n, _ := x.Int64()
			total += float64(n)
		case starlark.Float:
			total += float64(x)
			allInt = false
		default:
			return nil, fmt.Errorf("sum() 元素必须是数字, 得到 %s", el.Type())
		}
	}
	if allInt {
		return starlark.MakeInt(int(total)), nil
	}
	return starlark.Float(total), nil
}

func pyAbs(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kv []starlark.Tuple) (starlark.Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("abs() 缺少参数")
	}
	switch x := args[0].(type) {
	case starlark.Int:
		n, _ := x.Int64()
		if n < 0 {
			n = -n
		}
		return starlark.MakeInt64(n), nil
	case starlark.Float:
		f := float64(x)
		if f < 0 {
			f = -f
		}
		return starlark.Float(f), nil
	default:
		return nil, fmt.Errorf("abs() 需要数字")
	}
}

// pyAnyAll 生成 any()/all()，wantAll=true 时是 all。
func pyAnyAll(wantAll bool) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	name := "any"
	if wantAll {
		name = "all"
	}
	return func(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kv []starlark.Tuple) (starlark.Value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("%s() 缺少参数", name)
		}
		seq, ok := args[0].(starlark.Indexable)
		if !ok {
			return nil, fmt.Errorf("%s() 需要序列", name)
		}
		result := wantAll // any: 初始 false；all: 初始 true
		for i := 0; i < seq.Len(); i++ {
			truthy := bool(seq.Index(i).Truth())
			if wantAll && !truthy {
				result = false
				break
			}
			if !wantAll && truthy {
				result = true
				break
			}
		}
		return starlark.Bool(result), nil
	}
}
