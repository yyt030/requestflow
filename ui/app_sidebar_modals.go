package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// nameEditModal 场景名重命名弹窗。
func (a *App) nameEditModal(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Width(420).Padding(20).Gap(12).Background(t.Surface).Radius(12).Children(func() {
		ui.Text(c, "重命名场景").FontSize(16).Bold()
		ui.TextInput(c, &a.nameEditBuf).Placeholder("场景名称").Label("名称")
		ui.Row(c).Gap(8).Children(func() {
			ui.Spacer(c)
			if ui.Button(c, "取消").Clicked() {
				a.nameEditOpen = false
			}
			if ui.PrimaryButton(c, "保存").Clicked() {
				if n := strings.TrimSpace(a.nameEditBuf); n != "" {
					a.scenario.Name = n
				}
				a.nameEditOpen = false
			}
		})
	})
}

// envEditModal 环境切换/新建/删除弹窗。
func (a *App) envEditModal(c *ui.Context, t *ui.Theme) {
	envs := a.scenario.EnvNames()
	ui.Column(c).Width(460).Padding(20).Gap(10).Background(t.Surface).Radius(12).Children(func() {
		ui.Text(c, "环境管理").FontSize(16).Bold()
		ui.Text(c, "当前环境变量覆盖同名全局变量；运行时优先级：环境变量 > 全局变量。").
			FontSize(10).TextColor(t.TextMuted)
		ui.Divider(c)
		// 已有环境列表：点击即切换到该环境。
		if len(envs) == 0 {
			ui.Text(c, "（暂无环境）").FontSize(11).TextColor(t.TextMuted)
		}
		for _, name := range envs {
			nm := name
			label := "○ " + nm
			if nm == a.scenario.ActiveEnv {
				label = "● " + nm + "（当前）"
			}
			if ui.Button(c, label).Grow(1).Clicked() {
				a.scenario.ActiveEnv = nm
				a.activeEnvBuf = nm
			}
		}
		if ui.Button(c, "切换到(无环境)").Grow(1).Clicked() {
			a.scenario.ActiveEnv = ""
			a.activeEnvBuf = ""
		}
		ui.Divider(c)
		// 新建环境：输入名 + 新建按钮。
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.TextInput(c, &a.envEditBuf).Placeholder("新环境名").Grow(1)
			if ui.Button(c, "新建").Clicked() {
				n := strings.TrimSpace(a.envEditBuf)
				if n != "" {
					if a.scenario.Environments == nil {
						a.scenario.Environments = map[string]map[string]string{}
					}
					if _, exists := a.scenario.Environments[n]; !exists {
						a.scenario.Environments[n] = map[string]string{}
					}
					a.scenario.ActiveEnv = n
					a.activeEnvBuf = n
					a.envEditBuf = ""
				}
			}
			if a.scenario.ActiveEnv != "" {
				if ui.Button(c, "删除当前").TextColor(t.Danger).Clicked() {
					a.confirmDeleteEnv()
				}
			}
		})
		ui.Row(c).Children(func() {
			ui.Spacer(c)
			if ui.Button(c, "完成").Clicked() {
				a.envEditOpen = false
			}
		})
	})
}

// varEditModal 变量 key=value 编辑弹窗：origKey 为空表示新建，否则可改名/改值/删除。
func (a *App) varEditModal(c *ui.Context, t *ui.Theme) {
	title := "新建变量"
	if a.varEditOrigKey != "" {
		title = "编辑变量：" + a.varEditOrigKey
	}
	ui.Column(c).Width(460).Padding(20).Gap(10).Background(t.Surface).Radius(12).Children(func() {
		ui.Text(c, title).FontSize(16).Bold()
		if a.varEditTarget != "global" {
			ui.Text(c, "环境："+a.varEditTarget).FontSize(10).TextColor(t.Accent)
		}
		ui.TextInput(c, &a.varEditKey).Placeholder("变量名（key）").Label("变量名")
		ui.TextArea(c, &a.varEditValue).Placeholder("变量值（value）").Height(90)
		fieldHint(c, t, "在步骤中用 {{变量名}} 引用；环境变量覆盖同名全局变量。")
		ui.Row(c).Gap(8).Children(func() {
			ui.Spacer(c)
			if a.varEditOrigKey != "" {
				if ui.Button(c, "删除").TextColor(t.Danger).Clicked() {
					delete(a.varsFor(a.varEditTarget), a.varEditOrigKey)
					a.varEditOpen = false
				}
			}
			if ui.Button(c, "取消").Clicked() {
				a.varEditOpen = false
			}
			if ui.PrimaryButton(c, "保存").Clicked() {
				a.commitVarEdit()
			}
		})
	})
}

// commitVarEdit 把弹窗缓冲写回目标 map，处理改名（删旧键）与去掉空 key。
func (a *App) commitVarEdit() {
	k := strings.TrimSpace(a.varEditKey)
	if k == "" {
		return
	}
	vars := a.varsFor(a.varEditTarget)
	if a.varEditOrigKey != "" && a.varEditOrigKey != k {
		delete(vars, a.varEditOrigKey)
	}
	vars[k] = a.varEditValue
	a.varEditOpen = false
}
