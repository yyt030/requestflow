package ui

import (
	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

// 内置示例响应：一次典型的「图书列表」接口（JSON）与网页（HTML），
// 与下方默认例子一一对应，载入后可立即看到提取/处理结果。

const sampleJSONResponse = `{
  "code": 0,
  "data": {
    "token": "demo-token-abc123",
    "books": [
      {"title": "活着", "rating": 9.4, "url": "/book/1"},
      {"title": "许三观卖血记", "rating": 9.0, "url": "/book/2"},
      {"title": "平凡的世界", "rating": 8.8, "url": "/book/3"},
      {"title": "测试低分书", "rating": 6.2, "url": "/book/4"}
    ]
  }
}`

const sampleHTMLResponse = `<!doctype html><html><body>
  <div class="item"><h3>活着</h3><a class="link" href="/book/1">详情</a></div>
  <div class="item"><h3>许三观卖血记</h3><a class="link" href="/book/2">详情</a></div>
  <div class="item"><h3>平凡的世界</h3><a class="link" href="/book/3">详情</a></div>
</body></html>`

// extractExamples 提供 3 个可一键填入的提取例子，覆盖最常用方式。
func (a *App) extractExamples(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "默认例子").FontSize(10).TextColor(t.TextMuted)
		if ui.Button(c, previewExampleLabel(previewExtract, "JSON Token")).Clicked() {
			step.Extract = []model.Extract{{Name: "token", Source: "body", Kind: "jsonpath", Path: "data.token"}}
		}
		if ui.Button(c, previewExampleLabel(previewExtract, "JSON里的list")).Clicked() {
			// 把响应 JSON 里的数组（list）提取成变量：jsonpath 填数组所在的路径即可。
			step.Extract = []model.Extract{{Name: "books", Source: "body", Kind: "jsonpath", Path: "data.books"}}
		}
		if ui.Button(c, previewExampleLabel(previewExtract, "CSS 标题+链接")).Clicked() {
			step.Extract = []model.Extract{{
				Name: "books", Source: "body", Kind: "cssRows", Path: "div.item",
				CssFields: []model.CSSField{
					{Name: "title", Path: "h3", CssField: "text"},
					{Name: "link", Path: "a.link", CssField: "attr", CssAttr: "href", Resolve: true},
				},
			}}
		}
	})
	fieldHint(c, t, "想把响应 JSON 里的 list 取出来：方式选 jsonpath，路径填数组所在位置（如 data.books），变量名任取（如 books），下游「数据处理」即可用该变量。例子会替换当前规则、预览实时刷新；JSON 例请先载入示例 JSON，CSS 例请先载入示例 HTML。")
}

// processExample 一个可一键套用的处理例子：点击后同时设置 op 与参数。
type processExample struct {
	name string
	op   string
	args map[string]string
	desc string
}

// processExamplesData 覆盖最常用的处理操作。前 3 个依赖 books 变量，
// 后几个用字面量或 books，载入示例响应后均可直接试算出结果。
func processExamplesData() []processExample {
	return []processExample{
		{"解析JSON里的list", "json_to_table", map[string]string{
			"from": "books", "to": "table"},
			"把响应里取到的 JSON 数组变量 books 解析成表格 table（先在 HTTP 步骤「提取」标签用 jsonpath 取 data.books）"},
		{"过滤评分≥9", "filter", map[string]string{
			"from": "books", "field": "rating", "op": "gte", "value": "9", "to": "top"},
			"保留 rating≥9 的书，结果写入 top"},
		{"取前3条", "limit", map[string]string{
			"from": "books", "count": "3", "to": "top"},
			"截取 books 的前 3 条"},
		{"按评分排序", "sort", map[string]string{
			"from": "books", "field": "rating", "order": "desc", "to": "sorted"},
			"按 rating 从高到低排序，结果写入 sorted"},
		{"只取标题和链接", "pick", map[string]string{
			"from": "books", "fields": "title,url", "to": "picked"},
			"每条只保留 title 和 url 两个字段"},
		{"按标题去重", "dedup", map[string]string{
			"from": "books", "field": "title", "to": "deduped", "stat": "dupCount"},
			"按 title 去重，重复数写入 dupCount"},
		{"拼成一行标题", "join", map[string]string{
			"values": "活着,许三观卖血记,平凡的世界", "sep": " / ", "to": "titleLine"},
			"把多个值用分隔符拼成字符串（不依赖响应）"},
		{"组装结果行", "row", map[string]string{
			"fields": "title,rating", "title": "{{item.title}}", "rating": "{{item.rating}}", "to": "final"},
			"循环内把多字段组装成一行对象（item 用首个数组元素试算）"},
	}
}

// processExamples 渲染一键套用的处理例子。用 Wrap 自动换行，避免按钮过多挤在一行。
func (a *App) processExamples(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Row(c).Gap(6).Wrap().AlignItems(ui.Center).Children(func() {
		ui.Text(c, "默认例子").FontSize(10).TextColor(t.TextMuted)
		for _, ex := range processExamplesData() {
			ex := ex
			if ui.Button(c, previewExampleLabel(previewProcess, ex.name)).Tooltip(ex.desc).Clicked() {
				step.Op = ex.op
				step.Args = ex.args
			}
		}
	})
	fieldHint(c, t, "数据处理只认变量、不直接读响应报文：想解析返回 JSON 里的 list，请先在 HTTP 步骤「提取」标签用 jsonpath（如 data.books）取出成变量 books；本页「解析JSON里的list」会把 books 转成表格。依赖 books 的例子载入示例响应后预览自动回填；join 不依赖响应。点击例子会替换当前 op 与参数。")
}
