package ui

import (
	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

// stepTabNames 按步骤类型返回 Postman 风格的细粒度配置标签。
// 每个标签只承载一个配置维度，避免所有字段挤在一屏里、摆布不清。
func stepTabNames(step *model.Step) []string {
	switch step.Type {
	case model.StepRequest:
		// Postman 风格：Method/URL 已提到顶部请求行，子标签不再有「常规」。
		return []string{"Params", "Headers", "Body", "Auth", "提取", "断言", "脚本"}
	case model.StepTCP:
		return []string{"常规", "帧头", "发送", "读取", "提取", "断言", "脚本", "调试"}
	case model.StepLoop:
		return []string{"循环配置", "去重", "健壮性", "循环体"}
	case model.StepBranch:
		return []string{"条件", "高级", "then", "else"}
	case model.StepSwitch:
		return []string{"分支", "默认"}
	case model.StepParallel:
		return []string{"汇合配置", "子任务"}
	case model.StepMerge:
		return []string{"门控", "引用"}
	case model.StepProcess:
		return []string{"处理"}
	case model.StepScript:
		return []string{"脚本"}
	case model.StepAssert:
		return []string{"断言"}
	case model.StepExport:
		return []string{"导出"}
	default:
		return []string{"配置"}
	}
}

// clampStepTab 把 stepTab 限制在当前步骤的标签数内，切换步骤后不会越界。
func clampStepTab(a *App, step *model.Step) int {
	tabs := stepTabNames(step)
	if a.stepTab < 0 || a.stepTab >= len(tabs) {
		a.stepTab = 0
	}
	return a.stepTab
}

// stepTabView 按步骤类型与当前标签，把渲染分发到对应的配置视图。
func (a *App) stepTabView(c *ui.Context, t *ui.Theme, step *model.Step, tab string) {
	switch step.Type {
	case model.StepRequest:
		a.requestTabView(c, t, step, tab)
	case model.StepTCP:
		a.tcpTabView(c, t, step, tab)
	case model.StepLoop:
		a.loopTabView(c, t, step, tab)
	case model.StepBranch:
		a.branchTabView(c, t, step, tab)
	case model.StepSwitch:
		a.switchTabView(c, t, step, tab)
	case model.StepParallel:
		a.parallelTabView(c, t, step, tab)
	case model.StepMerge:
		a.mergeTabView(c, t, step, tab)
	case model.StepProcess:
		a.editorProcess(c, t, step)
	case model.StepScript:
		a.editorScript(c, t, step)
	case model.StepAssert:
		a.editorAssert(c, t, step)
	case model.StepExport:
		a.editorExport(c, t, step)
	}
}

// requestTabView 渲染 HTTP 请求步骤的单个配置标签。
func (a *App) requestTabView(c *ui.Context, t *ui.Theme, step *model.Step, tab string) {
	switch tab {
	case "Params":
		ui.Text(c, "查询参数 Query").FontSize(12).Bold()
		fieldHint(c, t, "每行 key: value，如 page: {{page}}，自动拼到 URL；支持 {{变量名}}。")
		queryText := mapToColonString(step.Query)
		if ui.TextArea(c, &queryText).Height(160).Changed() {
			step.Query = stringToMap(queryText, ":")
		}
	case "Headers":
		ui.Text(c, "请求头 Headers").FontSize(12).Bold()
		fieldHint(c, t, "每行 key: value，可直接粘贴 Chrome 复制的请求头；支持 {{变量}}。")
		headerText := mapToColonString(step.Headers)
		if ui.TextArea(c, &headerText).Height(160).Changed() {
			step.Headers = stringToMap(headerText, ":")
		}
	case "Body":
		ui.Text(c, "请求体 Body").FontSize(12).Bold()
		bodyEditor(c, t, &step.BodyType, &step.Body, &step.FormData,
			&step.GraphQLQuery, &step.GraphQLVariables, &a.bodyHeight, `{"key": "{{value}}"}`, &a.bodyFmtMsg)
	case "Auth":
		ui.Text(c, "认证").FontSize(12).Bold()
		authEditor(c, t, &step.AuthType, &step.AuthToken, &step.AuthUser, &step.AuthPass,
			&step.AuthKeyName, &step.AuthKeyValue, &step.AuthKeyIn,
			&step.AuthTokenURL, &step.AuthClientID, &step.AuthClientSecret, &step.AuthScope, &step.AuthGrantType)
	case "提取":
		a.editorExtract(c, t, step)
	case "断言":
		a.editorAssertList(c, t, step)
	case "脚本":
		a.editorRequestScripts(c, t, step)
	default: // 兼容旧数据：若某处仍传入已移除的「常规」，给出迁移提示。
		ui.Text(c, "请求方法与 URL 已移至上方请求行配置。").FontSize(12).Bold()
		fieldHint(c, t, "相对路径自动拼接 Base URL，完整 http(s):// 则忽略；配置后点请求行右侧【▶ 发送】。")
	}
}
