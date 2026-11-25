package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo"

	"requestflow/engine"
	"requestflow/history"
	"requestflow/logger"
	"requestflow/model"
)

// doDebugSend 异步发送调试请求，避免阻塞 UI 线程。
func (a *App) doDebugSend() {
	if strings.TrimSpace(a.debugURL) == "" {
		a.debugResult = &engine.DebugResult{OK: false, Err: "URL 不能为空"}
		return
	}
	timeout, _ := strconv.Atoi(strings.TrimSpace(a.debugTimeout))
	vars := map[string]string{"baseUrl": a.scenario.BaseURL}
	for k, v := range a.scenario.Variables {
		vars[k] = v
	}
	req := engine.DebugRequest{
		Method:           a.debugMethod,
		URL:              a.debugURL,
		Headers:          stringToMap(a.debugHeaders, ":"),
		Query:            stringToMap(a.debugQuery, ":"),
		Body:             a.debugBody,
		BodyType:         a.debugBodyType,
		FormData:         a.debugFormData,
		GraphQLQuery:     a.debugGQLQuery,
		GraphQLVariables: a.debugGQLVars,
		AuthType:         a.debugAuthType,
		AuthToken:        a.debugAuthToken,
		AuthUser:         a.debugUser,
		AuthPass:         a.debugPass,
		AuthKeyName:      a.debugKeyName,
		AuthKeyValue:     a.debugKeyValue,
		AuthKeyIn:        a.debugKeyIn,
		AuthTokenURL:     a.debugTokenURL,
		AuthClientID:     a.debugClientID,
		AuthClientSecret: a.debugClientSecret,
		AuthScope:        a.debugScope,
		AuthGrantType:    a.debugGrantType,
		TimeoutSec:       timeout,
		InsecureTLS:      a.debugInsecure,
	}
	a.debugSending = true
	a.debugResult = nil
	go func() {
		res := engine.Debug(req, vars)
		// 记录历史（含请求快照），写文件失败仅在日志提示，不影响调试结果。
		histEntry := history.Entry{
			Method: res.Method, URL: res.URL, Status: res.StatusCode,
			DurationMS: res.DurationMS, Error: res.Err, Source: "debug",
			Headers: req.Headers, BodyType: req.BodyType, Body: req.Body,
		}
		if err := history.Append(histEntry); err != nil {
			logger.Printf("记录请求历史失败: %v", err)
		}
		mygo.RunOnMain(func() {
			a.debugSending = false
			a.debugResult = res
			if res.OK {
				a.log(fmt.Sprintf("[调试] %s %s → %d (%d ms)", res.Method, previewURL(res.URL), res.StatusCode, res.DurationMS))
			} else {
				a.log("[调试] 失败: " + res.Err)
			}
		})
	}()
}

// syncStepToDebug 把 request 步骤的请求配置同步为一次发送的快照。
// 合并后 step 是请求配置的唯一数据源：编辑在步骤里完成，发送前用它刷新 debug 快照，
// 不再维护一份独立的、可编辑的调试配置——从根本上消除"编排/调试割裂"。
func (a *App) syncStepToDebug(step *model.Step) {
	if step == nil {
		return
	}
	// 步骤内联字段即请求真相源，直接同步到调试快照。
	a.debugMethod = step.Method
	if a.debugMethod == "" {
		a.debugMethod = "GET"
	}
	a.debugURL = step.URL
	a.debugHeaders = mapToColonString(step.Headers)
	a.debugQuery = mapToColonString(step.Query)
	a.debugBody = step.Body
	a.debugBodyType = step.BodyType
	if a.debugBodyType == "" {
		a.debugBodyType = "raw"
	}
	a.debugFormData = append([]model.FormField(nil), step.FormData...)
	a.debugGQLQuery = step.GraphQLQuery
	a.debugGQLVars = step.GraphQLVariables
	a.debugAuthType = step.AuthType
	if a.debugAuthType == "" {
		a.debugAuthType = "none"
	}
	a.debugAuthToken = step.AuthToken
	a.debugUser = step.AuthUser
	a.debugPass = step.AuthPass
	a.debugKeyName = step.AuthKeyName
	a.debugKeyValue = step.AuthKeyValue
	a.debugKeyIn = step.AuthKeyIn
	if a.debugKeyIn == "" {
		a.debugKeyIn = "header"
	}
	a.debugTokenURL = step.AuthTokenURL
	a.debugClientID = step.AuthClientID
	a.debugClientSecret = step.AuthClientSecret
	a.debugScope = step.AuthScope
	a.debugGrantType = step.AuthGrantType
	if a.debugGrantType == "" {
		a.debugGrantType = "client_credentials"
	}
	a.debugTimeout = "30"
	a.debugView = 0
}

// loadStepToDebug 兼容旧入口：同步步骤配置为调试快照并清空上次响应，用于双击步骤后查看/发送。
// 不再跳转主标签——编排界面内已内嵌响应区。
func (a *App) loadStepToDebug(step *model.Step) {
	a.syncStepToDebug(step)
	a.debugResult = nil
	a.refreshDebugTabBaseline()
}

// debugStep 把某个 request 步骤载入调试面板并立即发送。

// debugStep 把某个 request 步骤载入调试面板并立即发送。
func (a *App) debugStep(step *model.Step) {
	a.loadStepToDebug(step)
	a.doDebugSend()
}

// importPanel 展示 cURL 粘贴与解析界面。
