package ui

import (
	"fmt"
	"os"
	"requestflow/model"
	"requestflow/version"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Run 启动 GUI。
func Run() {
	app := &App{
		scenario:         demoScenario(),
		treeTopExpanded:  map[string]bool{},
		sceneExpanded:    map[string]bool{},
		sceneStepCache:   map[string][]model.Step{},
		splitFlowSteps:   300,
		selectedStep:     -1,
		splitSide:        280,
		splitSidePrev:    280,
		splitLog:         220,
		splitDebug:       50,
		splitEditorResp:  50,
		debugMethod:      "GET",
		debugAuthType:    "none",
		debugBodyType:    "raw",
		debugKeyIn:       "header",
		debugGrantType:   "client_credentials",
		debugTimeout:     "30",
		headersHeight:    56,
		queryHeight:      56,
		bodyHeight:       72,
		importHeight:     200,
		gqlVarsHeight:    90,
		reqQueryHeight:   160,
		reqHeadersHeight: 160,
		scriptCodeHeight: 240,
		preScriptHeight:  72,
		postScriptHeight: 72,
		tcpSendHeight:    56,
		snippetLang:      "python",
		sidebarTab:       0,
		showSteps:        true,
		varExpanded:      map[string]bool{},
		tabDirty:         map[string]string{},
		debugTabDirty:    map[string]string{},
		sidebarSection:   0,
		drawerWidth:      300,
		tabDragFrom:      -1,
		ctxMenuOpens:     map[string]*bool{},
		respBodyMode:     1,
	}
	// 请求字段内联在步骤中，是唯一真相源。
	app.refreshDebugTabBaseline() // 记录初始调试请求配置为基线
	app.logs = append(app.logs, "就绪。打开或新建场景，配置步骤后点击【一键运行】。")
	mygo.App.WhenReady(func() {
		app.win = mygo.NewWindow(mygo.WindowOptions{
			Title:   "RequestFlow " + version.Short() + " · request流程自动化",
			Width:   1440,
			Height:  900,
			Content: ui.View(app.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func (a *App) view(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Children(func() {
		a.toolbar(c, t)
		// 左侧栏（可折叠）+ 中央工作区。辅助面板由顶栏按钮的下拉气泡承载
		// （见 app_toolbar.go），不占布局宽度，点外部 / Esc 自动收起。
		// 侧栏折叠：记住宽度并压到 44；展开时恢复到合理值。
		if a.sidebarCollapsed {
			if a.splitSide > 100 {
				a.splitSidePrev = a.splitSide
			}
			a.splitSide = 44
		} else if a.splitSide < 100 && a.splitSidePrev > 100 {
			a.splitSide = a.splitSidePrev
		}
		ui.Split(c, &a.splitSide, func() { a.sidebar(c, t) }, func() {
			a.mainArea(c, t)
		}).Grow(1)
		// 新建场景向导：以 Modal 覆盖窗口，选择模板或空白场景。
		ui.Modal(c, &a.showTemplateWizard, func() {
			a.templateWizard(c, t)
		})
		a.sidebarModals(c, t)
	})
}

// templateWizard 渲染新建场景的模板选择向导。
func (a *App) templateWizard(c *ui.Context, t *ui.Theme) {
	templates := scenarioTemplates()
	if a.templateIndex >= len(templates) {
		a.templateIndex = 0
	}
	ui.Column(c).Width(560).Padding(20).Gap(10).Background(t.Surface).Radius(12).Children(func() {
		ui.Text(c, "新建场景").FontSize(18).Bold()
		ui.Text(c, "选择模板快速开始，之后可自由修改步骤与参数。").FontSize(11).TextColor(t.TextMuted)
		ui.Divider(c)
		ui.Scroll(c).Height(320).Gap(6).Children(func() {
			for i, tpl := range templates {
				i, tpl := i, tpl
				selected := i == a.templateIndex
				row := ui.ButtonBase(c.Key(fmt.Sprintf("tpl-%d", i))).Padding(10).Radius(8).
					AlignItems(ui.Center).Cursor(ui.CursorPointer)
				if selected {
					row.Background(t.Accent.Mix(t.Surface, 0.88))
				} else if row.Hovered() {
					row.Background(t.SurfaceHover)
				}
				row.Children(func() {
					ui.Column(c).Gap(3).Children(func() {
						ui.Text(c, tpl.Name).FontSize(13).Bold().TextColor(func() ui.Color {
							if selected {
								return t.Accent
							}
							return t.Text
						}())
						ui.Text(c, tpl.Desc).FontSize(10).TextColor(t.TextMuted)
					})
				})
				if row.Clicked() {
					a.templateIndex = i
				}
				if row.DoubleClicked() {
					a.applyTemplate(tpl)
				}
			}
		})
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Spacer(c)
			if ui.Button(c, "取消").Clicked() {
				a.showTemplateWizard = false
			}
			if ui.PrimaryButton(c, "使用模板").Clicked() {
				a.applyTemplate(templates[a.templateIndex])
			}
		})
	})
}

// consoleOnly 把运行结果（日志/结果表/变量）作为独立标签页铺满主工作区。
func (a *App) consoleOnly(c *ui.Context, t *ui.Theme) {
	a.console(c, t)
}

// ---------- 顶部工具栏（Postman 风格：品牌 + 场景 + 文件/数据/运行分组 + 溢出菜单） ----------
