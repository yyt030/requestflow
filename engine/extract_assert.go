package engine

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"

	"requestflow/model"
)

// AssertDetail 单个断言的结构化结果，供测试报告使用。
type AssertDetail struct {
	StepName string `json:"stepName,omitempty"`
	StepID   string `json:"stepId,omitempty"`
	Name     string `json:"name"`
	Source   string `json:"source"`
	Op       string `json:"op"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Passed   bool   `json:"passed"`
	Detail   string `json:"detail"`
}

// applyAsserts 对响应执行断言。全部通过返回 nil（兼容旧调用）。

// applyAsserts 对响应执行断言。全部通过返回 nil（兼容旧调用）。
func applyAsserts(ctx *Context, resp *model.Response, asserts []model.Assert) error {
	_, err := applyAssertsDetailed(ctx, resp, asserts)
	return err
}

// applyAssertsDetailed 执行断言并返回每条的结构化结果。遇到第一条失败即返回。

// applyAssertsDetailed 执行断言并返回每条的结构化结果。遇到第一条失败即返回。
func applyAssertsDetailed(ctx *Context, resp *model.Response, asserts []model.Assert) ([]AssertDetail, error) {
	details := make([]AssertDetail, 0, len(asserts))
	for _, a := range asserts {
		ok, detail, err := assertOne(ctx, resp, a)
		if err != nil {
			return details, err
		}
		name := a.Name
		if name == "" {
			name = a.Source
		}
		expected := ctx.Render(a.Value)
		details = append(details, AssertDetail{
			Name:     name,
			Source:   a.Source,
			Op:       a.Op,
			Expected: expected,
			Passed:   ok,
			Detail:   detail,
		})
		if !ok {
			// 从 detail 中解析实际值（detail 形如 实际="x", 期望 ...）
			details[len(details)-1].Actual = parseActual(detail)
			return details, fmt.Errorf("断言失败 [%s]: %s", name, detail)
		}
	}
	return details, nil
}

// parseActual 从断言详情文本中提取实际值（尽力而为，仅用于报告展示）。

// parseActual 从断言详情文本中提取实际值（尽力而为，仅用于报告展示）。
func parseActual(detail string) string {
	if strings.HasPrefix(detail, "实际=") {
		if i := strings.Index(detail, ", 期望"); i > 0 {
			s := detail[len("实际="):i]
			return strings.Trim(s, `"`)
		}
	}
	return ""
}

func assertOne(ctx *Context, resp *model.Response, a model.Assert) (bool, string, error) {
	source := strings.ToLower(a.Source)
	var actual interface{}
	switch source {
	case "status":
		actual = resp.StatusCode
	case "header":
		for k, v := range resp.Headers {
			if strings.EqualFold(k, a.Path) && len(v) > 0 {
				actual = v[0]
				break
			}
		}
	case "variable":
		actual = ctx.Get(a.Path)
	case "jsonpath", "", "body":
		if !gjson.Valid(resp.Text) {
			actual = resp.Text
		} else if a.Path != "" {
			r := gjson.Get(resp.Text, a.Path)
			if r.Exists() {
				actual = jsonValue(r)
			}
		} else {
			actual = resp.Text
		}
	default:
		actual = resp.Text
	}

	op := strings.ToLower(a.Op)
	if op == "" {
		op = "eq"
	}
	expected := ctx.Render(a.Value)
	actualStr := toStr(actual)

	var ok bool
	switch op {
	case "eq":
		ok = actualStr == expected
	case "ne":
		ok = actualStr != expected
	case "contains":
		ok = strings.Contains(actualStr, expected)
	case "exists":
		ok = actual != nil && actualStr != ""
	case "gt", "gte", "lt", "lte":
		af, aok := toFloat(actual)
		ef, eok := toFloat(expected)
		if !aok || !eok {
			return false, fmt.Sprintf("无法比较数值: %q 与 %q", actualStr, expected), nil
		}
		switch op {
		case "gt":
			ok = af > ef
		case "gte":
			ok = af >= ef
		case "lt":
			ok = af < ef
		case "lte":
			ok = af <= ef
		}
	default:
		return false, fmt.Sprintf("未知断言操作 %q", op), nil
	}
	detail := fmt.Sprintf("实际=%q, 期望 %s %q", actualStr, op, expected)
	return ok, detail, nil
}
