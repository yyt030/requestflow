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
		if ui.Button(c, previewExampleLabel(previewExtract, "JSON 数组")).Clicked() {
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
	fieldHint(c, t, "例子会替换当前提取规则；可继续编辑变量名/表达式，预览实时刷新。JSON 例请先载入示例 JSON 响应，CSS 例请先载入示例 HTML 响应。")
}

// processExamples 提供与示例响应配套的处理例子：filter 过滤评分、row 组装结果行。
func (a *App) processExamples(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "默认例子").FontSize(10).TextColor(t.TextMuted)
		if ui.Button(c, previewExampleLabel(previewProcess, "过滤评分≥9")).Clicked() {
			step.Op = "filter"
			step.Args = map[string]string{
				"from": "books", "field": "rating", "op": "gte", "value": "9", "to": "top",
			}
		}
		if ui.Button(c, previewExampleLabel(previewProcess, "取前3条")).Clicked() {
			step.Op = "limit"
			step.Args = map[string]string{"from": "books", "count": "3", "to": "top"}
		}
		if ui.Button(c, previewExampleLabel(previewProcess, "组装结果行")).Clicked() {
			step.Op = "row"
			step.Args = map[string]string{
				"fields": "title,rating", "title": "{{item.title}}", "rating": "{{item.rating}}", "to": "final",
			}
		}
	})
	fieldHint(c, t, "处理例依赖来源变量 books：请先用「提取例 · JSON 数组」提取 books，或载入示例响应后预览会自动把提取结果回填为变量。")
}
