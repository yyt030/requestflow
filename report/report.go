// Package report 把执行结果导出为 HTML / JSON 测试报告。
package report

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"

	"requestflow/engine"
)

// Data 报告根数据。
type Data struct {
	Title      string          `json:"title"`
	Scenario   string          `json:"scenario"`
	Generated  time.Time       `json:"generated"`
	Started    time.Time       `json:"started"`
	Ended      time.Time       `json:"ended"`
	Total      int             `json:"total"`
	Passed     int             `json:"passed"`
	Failed     int             `json:"failed"`
	DurationMS int64           `json:"durationMs"`
	Iterations []IterationData `json:"iterations"`
}

// IterationData 单次数据迭代（非数据驱动时只有一轮，Data 为空）。
type IterationData struct {
	Index      int                    `json:"index"`
	Data       map[string]string      `json:"data,omitempty"`
	OK         bool                   `json:"ok"`
	Err        string                 `json:"error,omitempty"`
	Started    time.Time              `json:"started"`
	Ended      time.Time              `json:"ended"`
	DurationMS int64                  `json:"durationMs"`
	Steps      []engine.StepTrace     `json:"steps"`
	Assertions []engine.AssertDetail  `json:"assertions"`
	Vars       map[string]interface{} `json:"vars,omitempty"`
}

// NewFromResult 用单次结果构造报告数据（非数据驱动）。
func NewFromResult(res *engine.Result) *Data {
	d := &Data{Generated: time.Now()}
	if res == nil {
		return d
	}
	d.Scenario = res.Scenario
	d.Title = "RequestFlow 测试报告 · " + res.Scenario
	d.Started, d.Ended = res.Started, res.Ended
	if !res.Ended.IsZero() && !res.Started.IsZero() {
		d.DurationMS = res.Ended.Sub(res.Started).Milliseconds()
	}
	iter := IterationData{
		Index: 1, OK: res.OK, Started: res.Started, Ended: res.Ended,
		DurationMS: d.DurationMS, Steps: res.Steps, Assertions: res.Assertions, Vars: res.Vars,
	}
	if res.Err != nil {
		iter.Err = res.Err.Error()
	}
	d.Iterations = []IterationData{iter}
	d.recalc()
	return d
}

// NewFromDataset 用数据驱动的多轮结果构造报告数据。
func NewFromDataset(results []engine.DatasetResult) *Data {
	d := &Data{Generated: time.Now(), Title: "RequestFlow 数据驱动测试报告"}
	d.Iterations = make([]IterationData, 0, len(results))
	for _, r := range results {
		iter := IterationData{
			Index: r.Index, Data: r.Data, OK: r.Result != nil && r.Result.OK,
			Steps: r.Result.Steps, Assertions: r.Result.Assertions, Vars: r.Vars,
		}
		if r.Result != nil {
			iter.Started, iter.Ended = r.Result.Started, r.Result.Ended
			iter.DurationMS = r.Result.Ended.Sub(r.Result.Started).Milliseconds()
			if r.Result.Err != nil {
				iter.Err = r.Result.Err.Error()
			}
			if d.Scenario == "" {
				d.Scenario = r.Result.Scenario
			}
			if d.Started.IsZero() || r.Result.Started.Before(d.Started) {
				d.Started = r.Result.Started
			}
			if r.Result.Ended.After(d.Ended) {
				d.Ended = r.Result.Ended
			}
		}
		d.Iterations = append(d.Iterations, iter)
	}
	if d.Scenario != "" {
		d.Title = "RequestFlow 数据驱动测试报告 · " + d.Scenario
	}
	d.DurationMS = d.Ended.Sub(d.Started).Milliseconds()
	d.recalc()
	return d
}

func (d *Data) recalc() {
	d.Total = len(d.Iterations)
	for _, it := range d.Iterations {
		if it.OK {
			d.Passed++
		} else {
			d.Failed++
		}
	}
}

// Save 根据扩展名保存报告（.json → JSON，其余 → HTML）。
func Save(data *Data, path string) error {
	if data == nil {
		return fmt.Errorf("报告数据为空")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".json" {
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(path, b, 0644)
	}
	// HTML
	var b strings.Builder
	if err := htmlTmpl.Execute(&b, data); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0644)
}

var htmlTmpl = template.Must(template.New("report").Funcs(template.FuncMap{
	"statusClass": func(ok bool) string {
		if ok {
			return "pass"
		}
		return "fail"
	},
	"statusText": func(ok bool) string {
		if ok {
			return "通过"
		}
		return "失败"
	},
	"passRate": func(pass, total int) string {
		if total == 0 {
			return "0%"
		}
		return fmt.Sprintf("%.0f%%", float64(pass)/float64(total)*100)
	},
}).Parse(htmlTemplate))

const htmlTemplate = `<!DOCTYPE html>
<html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<style>
:root{--pass:#16a34a;--fail:#dc2626;--muted:#6b7280;--bd:#e5e7eb;--bg:#f9fafb}
*{box-sizing:border-box}body{margin:0;font:14px/1.6 -apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,"PingFang SC","Microsoft YaHei",sans-serif;color:#111827;background:var(--bg)}
.wrap{max-width:1080px;margin:0 auto;padding:32px 20px}
h1{font-size:24px;margin:0 0 4px}.sub{color:var(--muted);font-size:13px;margin-bottom:24px}
.cards{display:flex;gap:12px;flex-wrap:wrap;margin-bottom:24px}
.card{background:#fff;border:1px solid var(--bd);border-radius:10px;padding:14px 20px;min-width:120px}
.card .n{font-size:26px;font-weight:700}.card .l{color:var(--muted);font-size:12px}
.card.pass .n{color:var(--pass)}.card.fail .n{color:var(--fail)}
.iter{background:#fff;border:1px solid var(--bd);border-radius:10px;margin-bottom:18px;overflow:hidden}
.iter h2{font-size:15px;margin:0;padding:12px 16px;background:#f3f4f6;border-bottom:1px solid var(--bd);display:flex;justify-content:space-between;align-items:center}
.badge{font-size:12px;font-weight:600;padding:2px 10px;border-radius:12px;color:#fff}
.badge.pass{background:var(--pass)}.badge.fail{background:var(--fail)}
table{width:100%;border-collapse:collapse}th,td{text-align:left;padding:8px 12px;border-bottom:1px solid var(--bd);font-size:13px;vertical-align:top}
th{background:#fafafa;color:var(--muted);font-weight:600}td.s-pass{color:var(--pass);font-weight:600}td.s-fail{color:var(--fail);font-weight:600}
.err{font-family:ui-monospace,Menlo,Consolas,monospace;font-size:12px;color:var(--fail)}
.kv{font-family:ui-monospace,Menlo,Consolas,monospace;font-size:12px;color:var(--muted)}
.empty{padding:16px;color:var(--muted)}
</style></head><body><div class="wrap">
<h1>{{.Title}}</h1>
<div class="sub">生成时间 {{.Generated.Format "2006-01-02 15:04:05"}} ｜ 耗时 {{.DurationMS}} ms</div>
<div class="cards">
<div class="card"><div class="n">{{.Total}}</div><div class="l">总迭代</div></div>
<div class="card pass"><div class="n">{{.Passed}}</div><div class="l">通过</div></div>
<div class="card fail"><div class="n">{{.Failed}}</div><div class="l">失败</div></div>
<div class="card"><div class="n">{{passRate .Passed .Total}}</div><div class="l">通过率</div></div>
</div>
{{range .Iterations}}
<div class="iter">
<h2><span>迭代 #{{.Index}}{{if .Data}} <span class="kv">{{range $k,$v := .Data}}{{$k}}={{$v}} {{end}}</span>{{end}}</span>
<span class="badge {{statusClass .OK}}">{{statusText .OK}} · {{.DurationMS}} ms</span></h2>
{{if .Err}}<div class="empty err">{{.Err}}</div>{{end}}
{{if .Steps}}
<table><thead><tr><th>步骤</th><th>类型</th><th>状态</th><th>耗时</th><th>说明</th></tr></thead><tbody>
{{range .Steps}}<tr><td>{{.Name}}</td><td>{{.Type}}</td>
<td class="s-{{.Status}}">{{if eq .Status "pass"}}✓ 通过{{else}}✗ 失败{{end}}</td>
<td>{{.DurationMS}} ms</td><td>{{if .Err}}<span class="err">{{.Err}}</span>{{else}}—{{end}}</td></tr>{{end}}
</tbody></table>{{end}}
{{if .Assertions}}
<table><thead><tr><th>断言</th><th>来源</th><th>期望</th><th>结果</th></tr></thead><tbody>
{{range .Assertions}}<tr><td>{{.Name}}</td><td>{{.Source}}</td><td>{{.Op}} {{.Expected}}</td>
<td class="{{if .Passed}}s-pass{{else}}s-fail{{end}}">{{if .Passed}}✓{{else}}✗ {{.Detail}}{{end}}</td></tr>{{end}}
</tbody></table>{{end}}
{{if and (not .Steps) (not .Assertions) (not .Err)}}<div class="empty">（无步骤记录）</div>{{end}}
</div>{{end}}
</div></body></html>`
