package ui

import (
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"requestflow/logger"
	"requestflow/version"
)

// ---------- 顶部工具栏（Postman 风格：品牌 + 场景 + 文件/数据/运行分组 + 溢出菜单） ----------
// 布局约定：整条工具栏垂直居中（AlignItems Center），各功能组用等高的工具条容器包裹，
// 组内间距统一、组间用细分隔线，避免纯文字标签与按钮框混排造成的参差观感。
func (a *App) toolbar(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Gap(10).Padding(10, 14).AlignItems(ui.Center).Children(func() {
		// 品牌标识：橙色圆角块 + 名称。
		ui.Box(c).Width(28).Height(28).Radius(7).Background(brandOrange).Center().Children(func() {
			ui.Text(c, "R").FontSize(15).Bold().TextColor(ui.RGB(255, 255, 255))
		})
		ui.Column(c).Gap(0).Children(func() {
			ui.Text(c, "RequestFlow").FontSize(14).Bold()
			ui.Text(c, "request流程自动化").FontSize(9).TextColor(t.TextMuted)
		})
		toolbarDivider(c, t)
		// ── 工作区切换：点击选择场景工作区目录 ──
		wsName := "工作区"
		if a.workspace.dir != "" {
			wsName = filepath.Base(a.workspace.dir)
		}
		if ui.Button(c, "▦ "+truncateRunes(wsName, 12)).Tooltip("切换场景工作区目录").Clicked() {
			a.openWorkspace()
		}
		// ── 环境选择：弹出菜单列出所有环境 + 无环境 ──
		envName := a.scenario.ActiveEnv
		if envName == "" {
			envName = "无环境"
		}
		ui.PopoverMenuButton(c, "◈ "+truncateRunes(envName, 12), func(m *ui.Menu) {
			if m.Item("无环境").Chosen() {
				a.scenario.ActiveEnv = ""
				a.log("已切换到环境：无环境")
			}
			for _, name := range a.scenario.EnvNames() {
				env := name
				if m.Item(env).Chosen() {
					a.scenario.ActiveEnv = env
					a.log("已切换到环境：" + env)
				}
			}
		})
		toolbarDivider(c, t)
		// 当前场景名。
		ui.Text(c, a.scenario.Name).FontSize(12).Bold().SingleLine()
		ui.Spacer(c)

		// ── 全局搜索：过滤左侧导航列表 ──
		ui.SearchField(c, &a.globalSearch).Placeholder("全局搜索…").Width(180)

		// ── 文件组：新建 / 打开 / 保存（统一图标按钮，等高对齐） ──
		toolbarGroup(c, t, func() {
			if iconButton(c, t, "new", iconNew, "新建场景", 28, t.Text).Clicked() {
				a.newScenario()
			}
			if iconButton(c, t, "open", iconOpen, "打开场景", 28, t.Text).Clicked() {
				a.openScenario()
			}
			if iconButton(c, t, "save", iconSave, "保存场景", 28, t.Text).Clicked() {
				a.saveScenario()
			}
		})
		toolbarDivider(c, t)

		// ── 数据组：导入（统一入口）/ 数据驱动 ──
		toolbarGroup(c, t, func() {
			// 导入入口统一到一个菜单，避免 cURL 与 Postman/OpenAPI 分散两处让用户找错。
			ui.PopoverMenuButton(c, "⇩ 导入", func(m *ui.Menu) {
				if m.Item("⇥ 从 cURL 粘贴").Chosen() {
					a.openImport("curl")
				}
				if m.Item("⇩ 导入 Postman / OpenAPI 文件").Chosen() {
					a.openImport("file")
				}
			})
			if ui.Button(c, "⊞ 数据驱动").Tooltip("用 CSV/JSON 数据逐行运行场景").Clicked() {
				a.pickDataFile()
			}
		})
		toolbarDivider(c, t)

		// ── 运行组：停止 / 运行 ──
		toolbarGroup(c, t, func() {
			if ui.Button(c, "■").Tooltip("停止运行").Disabled(!a.running).Clicked() {
				a.stop()
			}
			if ui.PrimaryButton(c, "▶ 运行").Disabled(a.running).Clicked() {
				a.run()
			}
		})
		toolbarDivider(c, t)

		// ── 溢出菜单：次要功能（历史 / 报告 / 导入 API / 关于） ──
		toolbarGroup(c, t, func() {
			ui.PopoverMenuButton(c, "⋯ 更多", func(m *ui.Menu) {
				if m.Item("🕘 请求历史").Chosen() {
					a.openHistory()
				}
				if m.Item("⤓ 导出测试报告").Disabled(a.lastResult == nil && len(a.dataResults) == 0).Chosen() {
					a.exportReport()
				}
				m.Separator()
				if m.Item("ⓘ 关于 RequestFlow " + version.Short()).Chosen() {
					a.showAbout()
				}
			})
		})
	})
}

// toolbarDivider 工具栏分组之间的细竖线，高度与按钮对齐。
func toolbarDivider(c *ui.Context, t *ui.Theme) {
	ui.Box(c).Width(1).Height(22).Background(t.Border.Mix(t.Surface, 0.15))
}

// toolbarGroup 用等高、垂直居中的容器包裹一组工具按钮，组间由调用方插入分隔线。
func toolbarGroup(c *ui.Context, t *ui.Theme, fn func()) {
	ui.Row(c).Gap(4).AlignItems(ui.Center).Children(fn)
}

// confirmDeleteEnv 删除当前环境前二次确认，避免误删整组环境变量。

// confirmDeleteEnv 删除当前环境前二次确认，避免误删整组环境变量。
func (a *App) confirmDeleteEnv() {
	env := a.scenario.ActiveEnv
	if env == "" {
		return
	}
	go func() {
		res, _ := mygo.Dialog.Message(mygo.MessageOptions{
			Parent:  a.win,
			Type:    mygo.MessageWarning,
			Title:   "删除环境",
			Message: "确定删除环境「" + env + "」及其全部变量吗？此操作不可撤销。",
			Buttons: []string{"取消", "删除"},
		})
		if res.Button == 1 {
			delete(a.scenario.Environments, env)
			a.scenario.ActiveEnv = ""
			a.activeEnvBuf = ""
			a.log("已删除环境 " + env)
		}
	}()
}

// openImport 统一导入入口：curl=打开粘贴面板，file=选择 Postman/OpenAPI 文件。

// openImport 统一导入入口：curl=打开粘贴面板，file=选择 Postman/OpenAPI 文件。
func (a *App) openImport(mode string) {
	a.showImport = true
	a.showHistory = false
	a.importMsg = ""
	a.mainTab = 0
	if mode == "file" {
		a.importAPIFile()
	}
}

// showAbout 弹出"关于"对话框展示完整版本信息。
// Dialog.Message 会阻塞，因此放到独立 goroutine 中调用。

// showAbout 弹出"关于"对话框展示完整版本信息。
// Dialog.Message 会阻塞，因此放到独立 goroutine 中调用。
func (a *App) showAbout() {
	go func() {
		detail := version.Detail()
		if p := logger.Path(); p != "" {
			detail += "\n\n日志文件:\n" + p
		}
		if err := logger.Err(); err != nil {
			detail += "\n\n⚠ 日志未启用:\n" + err.Error()
		}
		_, _ = mygo.Dialog.Message(mygo.MessageOptions{
			Parent:  a.win,
			Type:    mygo.MessageInfo,
			Title:   "关于 RequestFlow",
			Message: version.String(),
			Detail:  detail,
			Buttons: []string{"确定"},
		})
	}()
}

// ---------- 左侧：文件树风格场景导航 ----------
