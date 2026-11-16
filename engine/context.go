// Package engine 实现场景执行引擎：变量上下文、HTTP 请求、提取、断言、
// 内置处理、JS 脚本、导出与调度（含循环翻页）。
package engine

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Context 保存运行时变量，提供 {{expr}} 模板渲染与表达式求值。
// 线程安全，因为脚本/处理可能在子步骤中写变量。
type Context struct {
	mu    sync.RWMutex
	vars  map[string]interface{}
	steps map[string]interface{} // 每步的响应对象，供脚本按步骤名引用
}

func NewContext(initial map[string]string) *Context {
	vars := make(map[string]interface{})
	for k, v := range initial {
		vars[k] = v
	}
	return &Context{vars: vars, steps: make(map[string]interface{})}
}

func (c *Context) Set(name string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.vars[name] = value
}

func (c *Context) Get(name string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.vars[name]
}

// Fork 复制当前上下文为一个独立沙箱，供并行子任务使用。
// 子任务对变量的写入不会回传到父上下文；父上下文已有的变量对子任务可见。
func (c *Context) Fork() *Context {
	c.mu.RLock()
	vars := make(map[string]interface{}, len(c.vars))
	for k, v := range c.vars {
		vars[k] = v
	}
	steps := make(map[string]interface{}, len(c.steps))
	for k, v := range c.steps {
		steps[k] = v
	}
	c.mu.RUnlock()
	return &Context{vars: vars, steps: steps}
}

// SetStepResponse 记录某步骤名对应的响应对象（脚本可按步骤名引用）。
func (c *Context) SetStepResponse(stepName string, view map[string]interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.steps[stepName] = view
}

// Snapshot 返回所有变量的浅拷贝，用于脚本注入与结果展示。
func (c *Context) Snapshot() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]interface{}, len(c.vars)+len(c.steps))
	for k, v := range c.vars {
		out[k] = v
	}
	for k, v := range c.steps {
		out[k] = v
	}
	return out
}

var tokenRE = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)

// Render 把字符串中的 {{expr}} 依次替换为求值结果。
func (c *Context) Render(s string) string {
	if s == "" {
		return ""
	}
	return tokenRE.ReplaceAllStringFunc(s, func(m string) string {
		expr := strings.TrimSpace(m[2 : len(m)-2])
		return toStr(c.Eval(expr))
	})
}

// RenderMap 渲染 map 的所有 key 与 value。
func (c *Context) RenderMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[c.Render(k)] = c.Render(v)
	}
	return out
}

// Eval 求值一个表达式：支持字面量、变量路径、内置函数与比较运算。
func (c *Context) Eval(expr string) interface{} {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return ""
	}
	// 允许 Eval 接收被 {{ }} 包裹的表达式，如 Eval("{{len(rows)}}")
	for strings.HasPrefix(expr, "{{") && strings.HasSuffix(expr, "}}") {
		expr = strings.TrimSpace(expr[2 : len(expr)-2])
	}

	if v, ok := c.evalCompare(expr); ok {
		return v
	}
	if v, ok := c.evalFunc(expr); ok {
		return v
	}
	if v, ok := c.evalLiteral(expr); ok {
		return v
	}
	return c.resolvePath(expr)
}

var compareRE = regexp.MustCompile(`^(.*?)\s*(==|!=|>=|<=|>|<)\s*(.*)$`)

func (c *Context) evalCompare(expr string) (interface{}, bool) {
	m := compareRE.FindStringSubmatch(expr)
	if m == nil {
		return nil, false
	}
	left := c.Eval(strings.TrimSpace(m[1]))
	right := c.Eval(strings.TrimSpace(m[3]))
	op := m[2]
	lf, lok := toFloat(left)
	rf, rok := toFloat(right)
	var result bool
	if lok && rok {
		switch op {
		case "==":
			result = lf == rf
		case "!=":
			result = lf != rf
		case ">=":
			result = lf >= rf
		case "<=":
			result = lf <= rf
		case ">":
			result = lf > rf
		case "<":
			result = lf < rf
		}
	} else {
		ls, rs := toStr(left), toStr(right)
		switch op {
		case "==":
			result = ls == rs
		case "!=":
			result = ls != rs
		case ">":
			result = ls > rs
		case "<":
			result = ls < rs
		case ">=":
			result = ls >= rs
		case "<=":
			result = ls <= rs
		}
	}
	return result, true
}

var funcRE = regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*)\s*\((.*)\)$`)

func (c *Context) evalFunc(expr string) (interface{}, bool) {
	m := funcRE.FindStringSubmatch(expr)
	if m == nil {
		return nil, false
	}
	name := strings.ToLower(m[1])
	argStr := strings.TrimSpace(m[2])
	var args []string
	if argStr != "" {
		args = splitArgs(argStr)
	}
	evalArg := func(i int) interface{} {
		if i < len(args) {
			return c.Eval(strings.TrimSpace(args[i]))
		}
		return nil
	}
	switch name {
	case "len":
		return valueLen(evalArg(0)), true
	case "empty":
		return valueLen(evalArg(0)) == 0, true
	case "tostring":
		return toStr(evalArg(0)), true
	case "toint":
		f, _ := toFloat(evalArg(0))
		return int(f), true
	case "upper":
		return strings.ToUpper(toStr(evalArg(0))), true
	case "lower":
		return strings.ToLower(toStr(evalArg(0))), true
	case "contains":
		return strings.Contains(toStr(evalArg(0)), toStr(evalArg(1))), true
	case "default":
		v := evalArg(0)
		if valueLen(v) == 0 {
			return evalArg(1), true
		}
		return v, true
	case "add":
		a, _ := toFloat(evalArg(0))
		b, _ := toFloat(evalArg(1))
		return a + b, true
	case "sub":
		a, _ := toFloat(evalArg(0))
		b, _ := toFloat(evalArg(1))
		return a - b, true
	case "json":
		var out interface{}
		if err := json.Unmarshal([]byte(toStr(evalArg(0))), &out); err == nil {
			return out, true
		}
		return nil, true
	}
	return nil, false
}

func (c *Context) evalLiteral(expr string) (interface{}, bool) {
	if len(expr) >= 2 && ((expr[0] == '"' && expr[len(expr)-1] == '"') || (expr[0] == '\'' && expr[len(expr)-1] == '\'')) {
		return expr[1 : len(expr)-1], true
	}
	lower := strings.ToLower(expr)
	switch lower {
	case "true":
		return true, true
	case "false":
		return false, true
	case "null", "nil":
		return nil, true
	}
	if f, err := strconv.ParseFloat(expr, 64); err == nil {
		if !math.IsInf(f, 0) && strings.IndexAny(expr, ".eE") < 0 {
			return int(f), true
		}
		return f, true
	}
	return nil, false
}

// resolvePath 解析 "a.b.0.c" 形式的变量路径；支持 []interface{} 与 map。
func (c *Context) resolvePath(path string) interface{} {
	parts := strings.Split(path, ".")
	c.mu.RLock()
	var cur interface{} = c.vars[parts[0]]
	c.mu.RUnlock()
	for _, p := range parts[1:] {
		switch node := cur.(type) {
		case map[string]interface{}:
			cur = node[p]
		case []interface{}:
			i, err := strconv.Atoi(p)
			if err != nil || i < 0 || i >= len(node) {
				return nil
			}
			cur = node[i]
		case string:
			var parsed interface{}
			if json.Unmarshal([]byte(node), &parsed) == nil {
				cur = parsed
				if m, ok := cur.(map[string]interface{}); ok {
					cur = m[p]
				} else {
					return nil
				}
			} else {
				return nil
			}
		default:
			return nil
		}
	}
	return cur
}

func splitArgs(s string) []string {
	var out []string
	depth := 0
	start := 0
	runes := []rune(s)
	for i, r := range runes {
		switch r {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, string(runes[start:i]))
				start = i + 1
			}
		}
	}
	out = append(out, string(runes[start:]))
	return out
}

func valueLen(v interface{}) int {
	switch x := v.(type) {
	case nil:
		return 0
	case string:
		// 若字符串本身是 JSON 数组/对象，则按元素数量计算
		trimmed := strings.TrimSpace(x)
		if len(trimmed) > 0 && (trimmed[0] == '[' || trimmed[0] == '{') {
			var parsed interface{}
			if json.Unmarshal([]byte(trimmed), &parsed) == nil {
				return valueLen(parsed)
			}
		}
		return len([]rune(x))
	case []interface{}:
		return len(x)
	case []string:
		return len(x)
	case map[string]interface{}:
		return len(x)
	case map[string]string:
		return len(x)
	default:
		s := toStr(v)
		if s == "" || s == "<nil>" {
			return 0
		}
		return len([]rune(s))
	}
}

func toFloat(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func toStr(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []byte:
		return string(x)
	case bool:
		return strconv.FormatBool(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return fmt.Sprint(x)
		}
		return string(b)
	}
}
