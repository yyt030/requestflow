package engine

import (
	"regexp"
	"strings"
)

// Eval 求值一个表达式：支持字面量、变量路径、内置函数、比较运算与逻辑组合(&& ||)。
func (c *Context) Eval(expr string) interface{} {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return ""
	}
	// 允许 Eval 接收被 {{ }} 包裹的表达式，如 Eval("{{len(rows)}}")
	for strings.HasPrefix(expr, "{{") && strings.HasSuffix(expr, "}}") {
		expr = strings.TrimSpace(expr[2 : len(expr)-2])
	}

	// 逻辑运算：|| 优先级低于 &&，均在引号/括号之外按顶层切分。
	// 短路求值：|| 任一为真即真；&& 任一为假即假。
	if parts := splitOutside(expr, "||"); len(parts) > 1 {
		for _, p := range parts {
			if truthy(c.Eval(p)) {
				return true
			}
		}
		return false
	}
	if parts := splitOutside(expr, "&&"); len(parts) > 1 {
		for _, p := range parts {
			if !truthy(c.Eval(p)) {
				return false
			}
		}
		return true
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

// splitOutside 按分隔符在引号与成对括号之外做顶层切分。
// 忽略引号内、括号/方括号/花括号内的分隔符，用于安全解析逻辑表达式。

// splitOutside 按分隔符在引号与成对括号之外做顶层切分。
// 忽略引号内、括号/方括号/花括号内的分隔符，用于安全解析逻辑表达式。
func splitOutside(s, sep string) []string {
	var parts []string
	var quote rune
	depth := 0
	start := 0
	runes := []rune(s)
	for i := 0; i < len(runes); {
		r := runes[i]
		if quote != 0 {
			if r == quote && (i == 0 || runes[i-1] != '\\') {
				quote = 0
			}
			i++
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && strings.HasPrefix(string(runes[i:]), sep) {
				parts = append(parts, strings.TrimSpace(string(runes[start:i])))
				i += len([]rune(sep))
				start = i
				continue
			}
		}
		i++
	}
	if len(parts) > 0 {
		parts = append(parts, strings.TrimSpace(string(runes[start:])))
	}
	return parts
}

// truthy 把求值结果转为布尔：false/0/空值/空串为假，其余为真。

// truthy 把求值结果转为布尔：false/0/空值/空串为假，其余为真。
func truthy(v interface{}) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		switch strings.ToLower(strings.TrimSpace(x)) {
		case "", "false", "0", "null", "nil", "<nil>":
			return false
		}
		return true
	}
	if f, ok := toFloat(v); ok {
		return f != 0
	}
	return valueLen(v) > 0
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
