package ui

import (
	"github.com/egoist/mygo/ui"

	"requestflow/engine"
	"requestflow/model"
)

// 本文件固化「复杂 JSON 报文定位具体记录」案例：header + body + 20 条 list，
// 提供一键载入的示例响应，以及覆盖下标/条件/嵌套/整条/多字段的提取例子。

// sampleComplexJSONResponse 模拟真实接口报文：外层 header，业务体 body 内嵌 20 条 list。
// 与 complexExtractExamplesData 中的路径一一对应，载入后可立即看到每条例子的试算结果。
const sampleComplexJSONResponse = `{
  "header": {"code": 0, "traceId": "rf-20241124-001", "server": "api.example.com"},
  "body": {
    "page": 1, "pageSize": 20, "total": 137,
    "list": [
      {"id": 1,  "title": "甲", "score": 88, "detail": {"author": "张三"},   "tags": ["a", "b"]},
      {"id": 2,  "title": "乙", "score": 95, "detail": {"author": "李四"},   "tags": ["c"]},
      {"id": 3,  "title": "丙", "score": 72, "detail": {"author": "王五"},   "tags": ["a", "d"]},
      {"id": 4,  "title": "丁", "score": 91, "detail": {"author": "赵六"},   "tags": ["b"]},
      {"id": 5,  "title": "戊", "score": 60, "detail": {"author": "钱七"},   "tags": ["a"]},
      {"id": 6,  "title": "己", "score": 83, "detail": {"author": "孙八"},   "tags": ["b", "c"]},
      {"id": 7,  "title": "庚", "score": 77, "detail": {"author": "周九"},   "tags": ["a"]},
      {"id": 8,  "title": "辛", "score": 92, "detail": {"author": "吴十"},   "tags": ["c", "d"]},
      {"id": 9,  "title": "壬", "score": 68, "detail": {"author": "郑十一"}, "tags": ["b"]},
      {"id": 10, "title": "癸", "score": 85, "detail": {"author": "冯十二"}, "tags": ["a", "d"]},
      {"id": 11, "title": "子", "score": 90, "detail": {"author": "陈十三"}, "tags": ["c"]},
      {"id": 12, "title": "丑", "score": 74, "detail": {"author": "褚十四"}, "tags": ["b", "d"]},
      {"id": 13, "title": "寅", "score": 96, "detail": {"author": "卫十五"}, "tags": ["a", "c"]},
      {"id": 14, "title": "卯", "score": 81, "detail": {"author": "蒋十六"}, "tags": ["b"]},
      {"id": 15, "title": "辰", "score": 66, "detail": {"author": "沈十七"}, "tags": ["a", "d"]},
      {"id": 16, "title": "巳", "score": 93, "detail": {"author": "韩十八"}, "tags": ["c"]},
      {"id": 17, "title": "午", "score": 79, "detail": {"author": "杨十九"}, "tags": ["b", "c"]},
      {"id": 18, "title": "未", "score": 87, "detail": {"author": "朱二十"}, "tags": ["a"]},
      {"id": 19, "title": "申", "score": 62, "detail": {"author": "秦廿一"}, "tags": ["d"]},
      {"id": 20, "title": "酉", "score": 98, "detail": {"author": "尤廿二"}, "tags": ["a", "b"]}
    ]
  }
}`

// complexExtractExample 一条一键提取例子：点击后填入变量名 + jsonpath。
type complexExtractExample struct {
	name    string // 按钮名
	varName string // 输出变量名
	path    string // jsonpath 表达式
	desc    string // 悬浮说明
}

// complexExtractExamplesData 覆盖从复杂 JSON 定位记录的 7 种常用写法。
func complexExtractExamplesData() []complexExtractExample {
	return []complexExtractExample{
		{"头部字段", "code", "header.code", "取 JSON 外层 header 里的字段，如错误码 traceId"},
		{"记录总数", "total", "body.list.#", "数组末尾的 # 取元素个数（本例 20）"},
		{"第3条作者", "author3", "body.list.2.detail.author", "下标从 0 开始，逐层点进嵌套对象取字段"},
		{"按id取整条", "record", `body.list.#(id==3)#`, "条件定位整条记录：末尾 # 表示保留整个对象"},
		{"高分全部标题", "topTitles", `body.list.#(score>90)#.title`, "取所有匹配记录的某字段；末尾 # 不能省，否则只出第一条"},
		{"嵌套字段条件", "wangs", `body.list.#(detail.author=="王五").title`, "按嵌套字段筛选，嵌套路径可直接写在条件里"},
		{"整页list", "rows", "body.list", "取全部 20 条存入变量，供下游 filter/sort/pick 使用"},
	}
}

// complexProcessExamplesData 针对取出的 rows（整页 list）给出处理链路例子。
// 与 processExamplesData 的 op 可重复，但各自独立渲染、独立测试，互不干扰。
func complexProcessExamplesData() []processExample {
	return []processExample{
		{"复杂JSON·高分过滤", "filter", map[string]string{
			"from": "rows", "field": "score", "op": "gt", "value": "90", "to": "top"},
			"先用「整页list」提取例拿到 rows，再保留 score>90 的记录"},
		{"复杂JSON·按分排序", "sort", map[string]string{
			"from": "rows", "field": "score", "order": "desc", "to": "sorted"},
			"按 score 从高到低排序，结果写入 sorted"},
		{"复杂JSON·精简字段", "pick", map[string]string{
			"from": "rows", "fields": "id,title,score", "to": "picked"},
			"每条只保留 id/title/score 三个字段，丢掉嵌套 detail 与 tags"},
	}
}

// complexExtractExamples 渲染复杂 JSON 的一键提取例子，按钮自动换行避免挤在一行。
func (a *App) complexExtractExamples(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Row(c).Gap(6).Wrap().AlignItems(ui.Center).Children(func() {
		ui.Text(c, "复杂JSON例").FontSize(10).TextColor(t.TextMuted)
		if ui.Button(c, "载入20条报文").Tooltip("把 header+body+20 条 list 的示例响应放进调试结果，下方例子立即出结果").Clicked() {
			a.loadComplexSampleDebugResponse()
		}
		for _, ex := range complexExtractExamplesData() {
			ex := ex
			if ui.Button(c, previewExampleLabel(previewExtract, ex.name)).Tooltip(ex.desc).Clicked() {
				step.Extract = []model.Extract{{Name: ex.varName, Source: "body", Kind: "jsonpath", Path: ex.path}}
			}
		}
	})
}

// loadComplexSampleDebugResponse 把复杂 JSON 示例写进调试结果，供预览立即使用。
func (a *App) loadComplexSampleDebugResponse() {
	body := sampleComplexJSONResponse
	a.debugResult = &engine.DebugResult{
		OK: true, StatusCode: 200, StatusText: "OK",
		Method: "GET", URL: "sample://preview-complex",
		Body: body, Formatted: body, IsJSON: true, Size: len(body),
		Headers: []engine.KV{{Key: "Content-Type", Value: "application/json"}},
	}
}
