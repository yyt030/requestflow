package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"requestflow/engine"
	"requestflow/history"
	"requestflow/importer"
)

// importPanel 展示 cURL 粘贴与解析界面。
func (a *App) importPanel(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Children(func() {
		ui.Scroll(c).Grow(1).Padding(16).Gap(10).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "从 Chrome 导入 cURL").FontSize(15).Bold()
				ui.Spacer(c)
				if ui.Button(c, "✕ 关闭").Clicked() {
					a.showImport = false
					a.importMsg = ""
				}
			})
			ui.Text(c, "在 Chrome 开发者工具 Network 面板中右键请求 → Copy → Copy as cURL，粘贴到下方，点击解析即可生成请求步骤。").FontSize(11).TextColor(t.TextMuted)
			ui.Text(c, "支持 bash / cmd / PowerShell 三种复制格式；自动识别 Bearer Token、Cookie、查询参数与请求体。").FontSize(11).TextColor(t.TextMuted)
			ui.Divider(c)
			ui.Text(c, "或导入整个 API 定义").FontSize(12).Bold()
			fieldHint(c, t, "支持 Postman Collection v2.1 / OpenAPI 3.x / Swagger 2.0；自动转成多步骤场景（文件夹/路径展开为步骤，变量/认证/Body 保留）。")
			if ui.Button(c, "📂 导入 Postman / OpenAPI 文件").Clicked() {
				a.importAPIFile()
			}
			ui.Divider(c)
			resizableTextArea(c, t, &a.importText, &a.importHeight,
				"curl 'https://api.example.com/orders?page=1' \\\n  -H 'authorization: Bearer xxx' \\\n  -H 'cookie: session=abc'", 80, 900)
			if a.importMsg != "" {
				msgColor := t.Accent
				if strings.HasPrefix(a.importMsg, "✗") {
					msgColor = t.Danger
				}
				ui.Text(c, a.importMsg).FontSize(11).TextColor(msgColor)
			}
		})
		ui.Row(c).Gap(8).Padding(16, 12).AlignItems(ui.Center).Children(func() {
			if ui.PrimaryButton(c, "⇥ 解析并添加为步骤").Clicked() {
				a.doImportCurl()
			}
			if ui.Button(c, "清空").Clicked() {
				a.importText = ""
				a.importMsg = ""
			}
		})
	})
}

// openHistory 加载历史并切换到历史面板。

// openHistory 加载历史并切换到历史面板。
func (a *App) openHistory() {
	entries, err := history.Load()
	if err != nil {
		a.historyMsg = "✗ 读取历史失败: " + err.Error()
	} else {
		a.historyMsg = ""
	}
	a.historyList = entries
	a.showHistory = true
	a.showImport = false
	a.mainTab = 0
}

// historyPanel 请求历史面板：列表回看、重新载入调试、清空。

// historyPanel 请求历史面板：列表回看、重新载入调试、清空。
func (a *App) historyPanel(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Gap(10).Padding(14, 16).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "请求历史").FontSize(15).Bold()
			ui.Text(c, fmt.Sprintf("共 %d 条（exe 同目录 requestflow-history.jsonl）", len(a.historyList))).FontSize(10).TextColor(t.TextMuted)
			ui.Spacer(c)
			if ui.Button(c, "↻ 刷新").Clicked() {
				a.openHistory()
			}
			if ui.Button(c, "🗑 清空").Clicked() {
				if err := history.Clear(); err != nil {
					a.historyMsg = "✗ 清空失败: " + err.Error()
				} else {
					a.historyList = []history.Entry{}
					a.historyMsg = "已清空历史"
				}
			}
			if ui.Button(c, "✕ 关闭").Clicked() {
				a.showHistory = false
				a.historyMsg = ""
			}
		})
		if a.historyMsg != "" {
			ui.Row(c).Padding(0, 16, 6, 16).Children(func() {
				ui.Text(c, a.historyMsg).FontSize(11).TextColor(t.Accent)
			})
		}
		ui.Scroll(c).Grow(1).Padding(6, 16, 16, 16).Gap(4).Children(func() {
			if len(a.historyList) == 0 {
				ui.Text(c, "暂无历史。发送一次调试请求后会自动记录。").FontSize(11).TextColor(t.TextMuted)
				return
			}
			for _, e := range a.historyList {
				ent := e
				ui.Row(c).Gap(8).Padding(8, 10).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Width(5).Height(24).Radius(2).Background(methodColor(ent.Method))
					ui.Column(c).Grow(1).Gap(2).Children(func() {
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Text(c, ent.Method).FontSize(11).Bold().Width(52)
							ui.Text(c, previewURL(ent.URL)).FontSize(11)
						})
						status := ""
						if ent.Error != "" {
							status = "失败: " + ent.Error
						} else {
							status = fmt.Sprintf("%d · %d ms · %s", ent.Status, ent.DurationMS, ent.Source)
						}
						ui.Text(c, ent.Time+"  "+status).FontSize(10).TextColor(t.TextMuted)
					})
					if ui.Button(c, "载入调试").Clicked() {
						a.loadHistoryToDebug(ent)
					}
				})
			}
		})
	})
}

// loadHistoryToDebug 把一条历史记录的请求快照载回调试面板。

// loadHistoryToDebug 把一条历史记录的请求快照载回调试面板。
func (a *App) loadHistoryToDebug(e history.Entry) {
	a.debugMethod = e.Method
	a.debugURL = e.URL
	a.debugHeaders = mapToString(e.Headers)
	a.debugBodyType = e.BodyType
	if e.BodyType == "" {
		a.debugBodyType = "raw"
	}
	a.debugBody = e.Body
	a.showHistory = false
	a.mainTab = 1
	a.debugResult = nil
	a.refreshDebugTabBaseline() // 载入的历史请求作为新的干净基线
	a.log("已载入历史请求，可编辑后重新发送")
}

// importAPIFile 选择 Postman Collection / OpenAPI / Swagger 文件并导入为场景。

// importAPIFile 选择 Postman Collection / OpenAPI / Swagger 文件并导入为场景。
func (a *App) importAPIFile() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Title: "选择 API 定义文件（Postman / OpenAPI / Swagger）",
			Filters: []mygo.FileFilter{
				{Name: "API 定义", Extensions: []string{"json"}},
				{Name: "所有文件", Extensions: []string{"*"}},
			},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		data, err := os.ReadFile(paths[0])
		if err != nil {
			mygo.RunOnMain(func() { a.importMsg = "✗ 读取文件失败: " + err.Error() })
			return
		}
		sc, impErr := importer.Import(data)
		if impErr != nil {
			mygo.RunOnMain(func() { a.importMsg = "✗ 导入失败: " + impErr.Error() })
			return
		}
		base := filepath.Base(paths[0])
		mygo.RunOnMain(func() {
			a.applyLoadedScenario(sc, "")
			a.showImport = false
			a.importMsg = ""
			a.log(fmt.Sprintf("已导入 %s：%d 个步骤，%d 个全局变量", base, len(sc.Steps), len(sc.Variables)))
		})
	}()
}

// doImportCurl 解析粘贴的 cURL，成功则生成一个 request 步骤追加到场景末尾。

// doImportCurl 解析粘贴的 cURL，成功则生成一个 request 步骤追加到场景末尾。
func (a *App) doImportCurl() {
	text := strings.TrimSpace(a.importText)
	if text == "" {
		a.importMsg = "✗ 请先粘贴 cURL 命令"
		return
	}
	req, err := engine.ParseCurl(text)
	if err != nil {
		a.importMsg = "✗ 解析失败: " + err.Error()
		return
	}
	step := req.ToStep("", a.scenario.BaseURL)
	a.scenario.Steps = append(a.scenario.Steps, step)
	a.selectedStep = len(a.scenario.Steps) - 1
	a.showImport = false
	a.importText = ""
	a.importMsg = ""
	a.log("已导入 cURL 请求: " + step.Method + " " + previewURL(step.URL))
}

// clampHeight 把拖拽后的高度限制在 [min, max] 内。
// emptyState 渲染居中的空状态占位：主提示 + 可选副提示，统一各面板无数据时的视觉。
