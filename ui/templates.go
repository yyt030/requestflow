package ui

import (
	"requestflow/model"
)

// ScenarioTemplate 是一个可一键载入的场景模板，用于降低手写 JSON 门槛。
type ScenarioTemplate struct {
	Name  string                 // 模板名称
	Desc  string                 // 一句话说明
	Build func() *model.Scenario // 构建场景
}

// scenarioTemplates 返回内置模板列表。点击「新建场景」时展示，选中后加载到编辑器。
func scenarioTemplates() []ScenarioTemplate {
	return []ScenarioTemplate{
		{
			Name:  "空白场景",
			Desc:  "只有场景名，从零开始添加步骤",
			Build: func() *model.Scenario { return model.NewScenario("新场景") },
		},
		{
			Name:  "HTTP 登录取 Token",
			Desc:  "登录 → 从响应提取 token → 带 Bearer 查询个人信息并断言",
			Build: tmplLoginToken,
		},
		{
			Name:  "翻页采集（CSS + 去重 + 导出）",
			Desc:  "循环翻页 → CSS 提取多元素数组 → 累积按字段去重 → 导出 CSV",
			Build: tmplCrawlPages,
		},
		{
			Name:  "TCP 长连接 + 长度头帧",
			Desc:  "复用同一 socket 收发，8 位字符串长度头，含保活配置",
			Build: tmplTCPFrame,
		},
		{
			Name:  "搜索结果采集（含 base64 解码去重）",
			Desc:  "提取跳转链接 → base64decode 数组 → dedup 去重，演示 P0 新能力",
			Build: tmplSearchCollect,
		},
		{
			Name:  "搜索引擎采集（cssRows 结构化行）",
			Desc:  "行选择器一次提取标题+真实链接，配对零错位 → 按 URL 去重 → 取前 N 条 → 导出",
			Build: tmplSearchRows,
		},
	}
}

// tmplSearchRows 演示 cssRows 结构化行提取：标题与链接来自同一行，杜绝多选择器 zip 错位。
// 默认按百度结果结构预设（div.c-container[tpl=www_index] + mu 属性），可改为 Bing/其他站点。
func tmplSearchRows() *model.Scenario {
	sc := model.NewScenario("示例：搜索引擎采集（结构化行）")
	sc.BaseURL = "https://www.baidu.com"
	sc.Variables["keyword"] = "golang"
	search := model.Step{
		ID: newID(), Name: "搜索", Type: model.StepRequest, Enabled: true,
		Method: "GET", URL: "/s",
		Query: map[string]string{"wd": "{{keyword}}"},
		Extract: []model.Extract{
			{
				Name: "rows", Kind: "cssrows", Path: `div.c-container[tpl="www_index"]`,
				ExpectMin: 1,
				CssFields: []model.CSSField{
					{Name: "title", Path: "h3", CssField: "text"},
					{Name: "url", CssField: "attr", CssAttr: "mu", Resolve: true},
				},
			},
		},
	}
	dedup := model.Step{
		ID: newID(), Name: "按 URL 去重", Type: model.StepProcess, Enabled: true,
		Op: "dedup", Args: map[string]string{"from": "rows", "field": "url", "to": "rows", "stat": "dupCount"},
	}
	limit := model.Step{
		ID: newID(), Name: "取前 30 条", Type: model.StepProcess, Enabled: true,
		Op: "limit", Args: map[string]string{"from": "rows", "count": "30", "to": "top30"},
	}
	exp := model.Step{
		ID: newID(), Name: "导出 JSON", Type: model.StepExport, Enabled: true,
		From: "top30", Format: "json", Path: "search_result.json",
	}
	sc.Steps = []model.Step{search, dedup, limit, exp}
	return sc
}

func tmplLoginToken() *model.Scenario {
	sc := model.NewScenario("示例：登录取 Token")
	sc.BaseURL = "https://httpbin.org"
	login := model.Step{
		ID: newID(), Name: "登录", Type: model.StepRequest, Enabled: true,
		Method: "POST", URL: "/post",
		Body: `{"user":"demo","pwd":"123456"}`,
		Extract: []model.Extract{
			{Name: "token", Source: "body", Kind: "jsonpath", Path: "json.user"},
		},
		Assert: []model.Assert{{Source: "status", Op: "eq", Value: "200"}},
	}
	profile := model.Step{
		ID: newID(), Name: "个人信息", Type: model.StepRequest, Enabled: true,
		Method: "GET", URL: "/bearer",
		AuthType: "bearer", AuthToken: "{{token}}",
		Assert: []model.Assert{{Source: "status", Op: "eq", Value: "200"}},
	}
	sc.Steps = []model.Step{login, profile}
	return sc
}

func tmplCrawlPages() *model.Scenario {
	sc := model.NewScenario("示例：翻页采集 + 去重导出")
	sc.BaseURL = "https://example.com"
	sc.Variables["listUrl"] = "/list?page={{page}}"
	page := model.Step{
		ID: newID(), Name: "采集本页", Type: model.StepRequest, Enabled: true,
		Method: "GET", URL: "{{listUrl}}",
		Extract: []model.Extract{
			{Name: "items", Source: "body", Kind: "css", Path: "li.item", CssField: "text"},
		},
	}
	accumulate := model.Step{
		ID: newID(), Name: "累积到 allRows", Type: model.StepProcess, Enabled: true,
		Op: "flatten", Args: map[string]string{"from": "allRows,items", "to": "allRows"},
	}
	loop := model.Step{
		ID: newID(), Name: "翻页 1-5", Type: model.StepLoop, Enabled: true,
		Loop: &model.Loop{
			Mode: "count", Count: 5, PageVar: "page", PageStart: 1, PageStep: 1, Max: 20,
			DedupVar: "allRows", DedupField: "", DedupStat: "dupCount",
		},
		Children: []model.Step{page, accumulate},
	}
	exp := model.Step{
		ID: newID(), Name: "导出 CSV", Type: model.StepExport, Enabled: true,
		From: "allRows", Format: "csv", Path: "result.csv",
	}
	sc.Steps = []model.Step{loop, exp}
	return sc
}

func tmplTCPFrame() *model.Scenario {
	sc := model.NewScenario("示例：TCP 长连接 + 长度头帧")
	sc.Variables["host"] = "127.0.0.1"
	sc.Variables["port"] = "9000"
	login := model.Step{
		ID: newID(), Name: "登录", Type: model.StepTCP, Enabled: true,
		TCPHost: "{{host}}", TCPPort: "{{port}}", TCPTimeout: "5000",
		TCPConnMode: "long", TCPConnKey: "conn1", TCPKeepAlive: "30",
		TCPFrameType: "ascii", TCPFrameHead: "8",
		TCPSendMode: "frame", TCPSend: "LOGIN",
		TCPReadMode: "frame",
		Extract: []model.Extract{
			{Name: "session", Source: "body", Kind: "regex", Path: "SESSION=([A-Za-z0-9]+)", Group: 1},
		},
	}
	query := model.Step{
		ID: newID(), Name: "查询", Type: model.StepTCP, Enabled: true,
		TCPHost: "{{host}}", TCPPort: "{{port}}", TCPTimeout: "5000",
		TCPConnMode: "long", TCPConnKey: "conn1", TCPKeepAlive: "30",
		TCPFrameType: "ascii", TCPFrameHead: "8",
		TCPSendMode: "frame", TCPSend: "QUERY {{session}}",
		TCPReadMode: "frame",
	}
	closeStep := model.Step{
		ID: newID(), Name: "关闭连接", Type: model.StepTCP, Enabled: true,
		TCPHost: "{{host}}", TCPPort: "{{port}}",
		TCPConnMode: "long", TCPConnKey: "conn1", TCPClose: true,
	}
	sc.Steps = []model.Step{login, query, closeStep}
	return sc
}

func tmplSearchCollect() *model.Scenario {
	sc := model.NewScenario("示例：搜索结果采集 + 解码去重")
	sc.BaseURL = "https://www.bing.com"
	sc.Variables["keyword"] = "黄仁勋"
	search := model.Step{
		ID: newID(), Name: "搜索", Type: model.StepRequest, Enabled: true,
		Method: "GET", URL: "/search",
		Query: map[string]string{"q": "{{keyword}}"},
		Extract: []model.Extract{
			{Name: "rawLinks", Source: "body", Kind: "css", Path: "li.b_algo h2 a", CssField: "attr", CssAttr: "href"},
			{Name: "titles", Source: "body", Kind: "css", Path: "li.b_algo h2", CssField: "text"},
		},
	}
	decode := model.Step{
		ID: newID(), Name: "解码跳转链接", Type: model.StepProcess, Enabled: true,
		Op: "base64decode", Args: map[string]string{"from": "rawLinks", "to": "links"},
	}
	dedup := model.Step{
		ID: newID(), Name: "去重", Type: model.StepProcess, Enabled: true,
		Op: "dedup", Args: map[string]string{"from": "links", "to": "uniqueLinks", "stat": "dupCount"},
	}
	sc.Steps = []model.Step{search, decode, dedup}
	return sc
}
