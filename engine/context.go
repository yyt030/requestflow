// Package engine 实现场景执行引擎：变量上下文、HTTP 请求、提取、断言、
// 内置处理、JS 脚本、导出与调度（含循环翻页）。
package engine

import (
	"regexp"
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

// SetStepResponse 记录某步骤名对应的响应对象（脚本可按步骤名引用）。
func (c *Context) SetStepResponse(stepName string, view map[string]interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.steps[stepName] = view
}

// Snapshot 返回所有变量的浅拷贝，用于脚本注入与结果展示。

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

var varPathRE = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_.]*$`)

// Render 把字符串中的 {{expr}} 依次替换为求值结果。

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

// resolveArg 解析一个参数表达式，返回「原始值」（可能是数组/对象）：
//   - {{var}} 形式 → 取变量原始值
//   - 纯变量名/点路径（如 rows、data.list）→ 变量存在时取原始值
//   - 其它文本（含 JSON 字面量、含 {{}} 的拼接串）→ 渲染为字符串
//
// 供 process 步骤区分「数组输入」与「文本输入」。

// resolveArg 解析一个参数表达式，返回「原始值」（可能是数组/对象）：
//   - {{var}} 形式 → 取变量原始值
//   - 纯变量名/点路径（如 rows、data.list）→ 变量存在时取原始值
//   - 其它文本（含 JSON 字面量、含 {{}} 的拼接串）→ 渲染为字符串
//
// 供 process 步骤区分「数组输入」与「文本输入」。
func (c *Context) resolveArg(expr string) interface{} {
	s := strings.TrimSpace(expr)
	if s == "" {
		return ""
	}
	if m := tokenRE.FindStringSubmatch(s); len(m) == 2 && strings.TrimSpace(s[2:len(s)-2]) == m[1] {
		return c.Eval(m[1])
	}
	if varPathRE.MatchString(s) {
		head := strings.SplitN(s, ".", 2)[0]
		c.mu.RLock()
		_, exists := c.vars[head]
		c.mu.RUnlock()
		if exists {
			return c.Eval(s)
		}
	}
	return c.Render(expr)
}

// RenderMap 渲染 map 的所有 key 与 value。

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

// Eval 求值一个表达式：支持字面量、变量路径、内置函数、比较运算与逻辑组合(&& ||)。
