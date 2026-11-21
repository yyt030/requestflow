package ui

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"requestflow/logger"
	"requestflow/version"
)

// ---------- 顶部工具栏（Postman 风格：品牌 + 场景 + 文件/数据/运行分组 + 溢出菜单） ----------
func (a *App) toolbar(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Gap(8).Padding(10, 14).AlignItems(ui.Center).Children(func() {
		// 品牌标识：橙色圆角块 + 名称。
		ui.Box(c).Width(26).Height(26).Radius(7).Background(brandOrange).Center().Children(func() {
			ui.Text(c, "R").FontSize(14).Bold().TextColor(ui.RGB(255, 255, 255))
		})
		ui.Column(c).Gap(0).Children(func() {
			ui.Text(c, "RequestFlow").FontSize(14).Bold()
			ui.Text(c, "request流程自动化").FontSize(9).TextColor(t.TextMuted)
		})
		ui.Divider(c).Height(24)
		// 当前场景名（工作区概念）。
		ui.Icon(c, iconFolder).Size(14, 14).TextColor(t.TextMuted)
		ui.Text(c, a.scenario.Name).FontSize(12).Bold()
		if a.filePath != "" {
			ui.Badge(c, "已保存").TextColor(t.TextMuted)
		} else {
			ui.Badge(c, "未保存").TextColor(t.TextMuted)
		}
		ui.Spacer(c)

		// ── 文件组：新建 / 打开 / 保存 ──
		ui.Text(c, "文件").FontSize(9).TextColor(t.TextMuted)
		if iconButton(c, t, "new", iconNew, "新建场景", 28, t.Text).Clicked() {
			a.newScenario()
		}
		if iconButton(c, t, "open", iconOpen, "打开场景", 28, t.Text).Clicked() {
			a.openScenario()
		}
		if iconButton(c, t, "save", iconSave, "保存场景", 28, t.Text).Clicked() {
			a.saveScenario()
		}
		ui.Divider(c).Height(20)

		// ── 数据组：导入（统一入口）/ 数据驱动 ──
		ui.Text(c, "数据").FontSize(9).TextColor(t.TextMuted)
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
		ui.Divider(c).Height(20)

		// ── 运行组：停止 / 运行 ──
		ui.Text(c, "运行").FontSize(9).TextColor(t.TextMuted)
		if ui.Button(c, "■").Tooltip("停止运行").Disabled(!a.running).Clicked() {
			a.stop()
		}
		if ui.PrimaryButton(c, "▶ 运行").Disabled(a.running).Clicked() {
			a.run()
		}
		ui.Divider(c).Height(20)

		// ── 溢出菜单：次要功能（历史 / 报告 / 导入 API / 关于） ──
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
