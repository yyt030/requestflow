package engine

import (
	"fmt"
	"strings"

	"requestflow/model"
)

// runStepScript 按脚本语言执行一段请求级脚本（Pre-request / Tests）。
// 语言取自 step.ScriptLang，空值按 js 处理，与独立 script 步骤保持一致。
func (e *Engine) runStepScript(code string, step model.Step, last *model.Response, phase string) error {
	if strings.TrimSpace(code) == "" {
		return nil
	}
	wrap := step
	wrap.Code = code
	var err error
	if strings.EqualFold(step.ScriptLang, "python") || strings.EqualFold(step.ScriptLang, "py") {
		err = runPythonScript(e.ctx, wrap, last)
	} else {
		err = runScript(e.ctx, wrap, last)
	}
	if err != nil {
		return fmt.Errorf("%s 脚本失败: %w", phase, err)
	}
	return nil
}

func (e *Engine) runRequest(step model.Step) error {
	// Pre-request Script：在渲染请求参数之前执行，脚本对变量的改写能影响本次请求。
	if err := e.runStepScript(step.PreScript, step, e.lastResp, "Pre-request"); err != nil {
		return err
	}
	method := e.ctx.Render(step.Method)
	rawURL := e.ctx.Render(step.URL)
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		base := e.ctx.Render(e.scenario.BaseURL)
		if base != "" {
			rawURL = strings.TrimRight(base, "/") + "/" + strings.TrimLeft(rawURL, "/")
		}
	}
	headers := e.ctx.RenderMap(step.Headers)
	query := e.ctx.RenderMap(step.Query)
	body := e.ctx.Render(step.Body)
	// 构造渲染后的完整请求 step：携带 Body 类型 / 表单 / GraphQL / 认证全部字段。
	reqStep := model.Step{
		BodyType:         e.ctx.Render(step.BodyType),
		Body:             body,
		GraphQLQuery:     e.ctx.Render(step.GraphQLQuery),
		GraphQLVariables: e.ctx.Render(step.GraphQLVariables),
		AuthType:         e.ctx.Render(step.AuthType),
		AuthToken:        e.ctx.Render(step.AuthToken),
		AuthUser:         e.ctx.Render(step.AuthUser),
		AuthPass:         e.ctx.Render(step.AuthPass),
		AuthKeyName:      e.ctx.Render(step.AuthKeyName),
		AuthKeyValue:     e.ctx.Render(step.AuthKeyValue),
		AuthKeyIn:        e.ctx.Render(step.AuthKeyIn),
		AuthTokenURL:     e.ctx.Render(step.AuthTokenURL),
		AuthClientID:     e.ctx.Render(step.AuthClientID),
		AuthClientSecret: e.ctx.Render(step.AuthClientSecret),
		AuthScope:        e.ctx.Render(step.AuthScope),
		AuthGrantType:    e.ctx.Render(step.AuthGrantType),
	}
	for _, f := range step.FormData {
		reqStep.FormData = append(reqStep.FormData, model.FormField{
			Key:   e.ctx.Render(f.Key),
			Type:  f.Type,
			Value: e.ctx.Render(f.Value),
		})
	}

	e.emit("log", step.ID, step.Name, fmt.Sprintf("%s %s", method, rawURL))
	resp, err := e.http.DoRequest(&reqStep, method, rawURL, headers, query)
	if err != nil {
		return err
	}
	e.lastResp = resp
	e.emit("log", step.ID, step.Name, fmt.Sprintf("响应 %d, %d 字节", resp.StatusCode, len(resp.Body)))

	if err := applyExtracts(e.ctx, resp, step.Extract); err != nil {
		return err
	}
	details, detErr := applyAssertsDetailed(e.ctx, resp, step.Assert)
	e.trace.addAssertions(step.Name, step.ID, details)
	if detErr != nil {
		return detErr
	}
	e.ctx.SetStepResponse(step.Name, responseView(resp))
	// Tests（PostScript）：在响应提取与断言之后执行，last 指向本次响应。
	if err := e.runStepScript(step.PostScript, step, resp, "Tests"); err != nil {
		return err
	}
	return nil
}

// runBranch 条件分支：when 成立执行 then 子步骤，否则执行 else 子步骤。
// 支持结构化 Conditions（按 Logic 组合）与原始 When 表达式（优先）。
