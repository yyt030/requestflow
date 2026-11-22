package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

// ---------- 左侧：文件树风格场景导航 ----------
func (a *App) sidebar(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Children(func() {
		// 面板标题栏。
		ui.Row(c).Padding(12, 12, 8, 12).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconFolder).Size(16, 16).TextColor(brandOrange)
			ui.Text(c, "资源管理器").FontSize(13).Bold()
			ui.Spacer(c)
			ui.Text(c, fmt.Sprintf("%d 项", len(a.scenario.Steps))).FontSize(10).TextColor(t.TextMuted)
		})
		// 搜索框：过滤步骤。
		ui.Row(c).Padding(0, 12, 10, 12).Children(func() {
			ui.SearchField(c, &a.stepSearch).Placeholder("搜索步骤…")
		})
		// 文件树根节点区。
		ui.Scroll(c).Grow(1).Padding(2, 8, 10, 8).Children(func() {
			a.fileTree(c, t)
		})
	})
}

// fileTree 文件树：场景（步骤）与环境变量两棵可折叠文件夹节点。

// fileTree 文件树：场景（步骤）与环境变量两棵可折叠文件夹节点。
func (a *App) fileTree(c *ui.Context, t *ui.Theme) {
	// 根文件夹：场景 —— 子节点是各个步骤。
	a.treeFolderRow(c, t, "folder-scene", &a.showSteps, iconFolder, brandOrange, func() {
		ui.TextInput(c, &a.scenario.Name).FontSize(13).Bold().Grow(1)
	}, nil)
	if a.showSteps {
		treeGuide(c, t, func() { a.stepList(c, t) })
	}

	ui.Box(c).Height(8)

	// 根文件夹：环境变量 —— 子节点是环境选择器 + 全局变量 + 当前环境变量。
	varCount := len(a.scenario.Variables)
	if env := a.scenario.EnvVars(a.scenario.ActiveEnv); env != nil {
		varCount += len(env)
	}
	a.treeFolderRow(c, t, "folder-vars", &a.showVars, iconFolder, t.TextMuted, func() {
		ui.Text(c, "变量").FontSize(12).Bold()
		ui.Text(c, fmt.Sprintf("(%d)", varCount)).FontSize(10).TextColor(t.TextMuted)
	}, nil)
	if a.showVars {
		treeGuide(c, t, func() { a.environmentEditor(c, t) })
	}
}

// environmentEditor 环境变量分层编辑：环境选择器 + 全局变量 + 当前环境变量。
// 运行时"当前环境变量"覆盖同名"全局变量"，可在多套环境间快速切换。

// environmentEditor 环境变量分层编辑：环境选择器 + 全局变量 + 当前环境变量。
// 运行时"当前环境变量"覆盖同名"全局变量"，可在多套环境间快速切换。
func (a *App) environmentEditor(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Gap(6).Children(func() {
		// 环境选择器：无 / 各环境名。切换时同步到场景与本地缓冲。
		envs := append([]string{"(无)"}, a.scenario.EnvNames()...)
		cur := "(无)"
		if a.scenario.ActiveEnv != "" {
			cur = a.scenario.ActiveEnv
		}
		sel := cur
		if ui.Select(c, &sel, envs).Label("当前环境").Changed() {
			if sel == "(无)" {
				a.scenario.ActiveEnv = ""
			} else {
				a.scenario.ActiveEnv = sel
			}
			a.activeEnvBuf = a.scenario.ActiveEnv
		}
		fieldHint(c, t, "当前环境变量覆盖同名全局变量；运行时优先级：环境变量 > 全局变量。")

		// 新建环境
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			ui.TextInput(c, &a.newEnvName).Placeholder("新环境名").Width(90)
			if ui.Button(c, "+").Width(26).Tooltip("新建环境").Clicked() {
				n := strings.TrimSpace(a.newEnvName)
				if n != "" {
					if a.scenario.Environments == nil {
						a.scenario.Environments = map[string]map[string]string{}
					}
					if _, exists := a.scenario.Environments[n]; !exists {
						a.scenario.Environments[n] = map[string]string{}
						a.scenario.ActiveEnv = n
						a.activeEnvBuf = n
					}
					a.newEnvName = ""
				}
			}
			if a.scenario.ActiveEnv != "" {
				if ui.Button(c, "✕").Width(26).Tooltip("删除当前环境").Clicked() {
					a.confirmDeleteEnv()
				}
			}
		})

		// 全局变量区
		ui.Text(c, "全局变量").FontSize(11).Bold().TextColor(t.TextMuted)
		a.variablesEditor(c, t)

		// 当前环境变量区
		if a.scenario.ActiveEnv != "" {
			ui.Divider(c)
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "环境 ["+a.scenario.ActiveEnv+"] 变量").FontSize(11).Bold().TextColor(t.Accent)
			})
			envVars := a.scenario.Environments[a.scenario.ActiveEnv]
			a.envVariablesEditor(c, t, envVars)
		}
	})
}

// envVariablesEditor 编辑指定环境的变量集（key=value），结构同全局变量编辑器。

// envVariablesEditor 编辑指定环境的变量集（key=value），结构同全局变量编辑器。
func (a *App) envVariablesEditor(c *ui.Context, t *ui.Theme, vars map[string]string) {
	if vars == nil {
		return
	}
	keyValueRows(c, t, vars, &a.newVarKey, &a.newVarValue,
		"如 baseUrl: https://api.prod.com；切换环境即整组生效", 78)
}

// treeFolderRow 文件夹节点行：展开箭头 + 文件夹图标 + 标题内容 + 尾部操作。

// treeFolderRow 文件夹节点行：展开箭头 + 文件夹图标 + 标题内容 + 尾部操作。
func (a *App) treeFolderRow(c *ui.Context, t *ui.Theme, key string, open *bool, icon *ui.SVG, iconColor ui.Color, label func(), tail func()) {
	ui.Row(c.Key(key)).Gap(4).Padding(5, 4).AlignItems(ui.Center).Children(func() {
		arrow := map[bool]string{true: "▾", false: "▸"}[*open]
		if ui.Button(c.Key("tog"), arrow).Width(16).FontSize(10).Clicked() {
			*open = !*open
		}
		ui.Icon(c, icon).Size(14, 14).TextColor(iconColor)
		label()
		ui.Spacer(c)
		if tail != nil {
			tail()
		}
	})
}

// treeGuide 在子节点左侧画一条贯穿高度的竖线，形成文件树的层级引导线。

// treeGuide 在子节点左侧画一条贯穿高度的竖线，形成文件树的层级引导线。
func treeGuide(c *ui.Context, t *ui.Theme, fn func()) {
	color := t.Border.Mix(t.Surface, 0.2)
	ui.Row(c).AlignItems(ui.Stretch).Children(func() {
		ui.Box(c).Width(14)                  // 缩进占位（透明）
		ui.Box(c).Width(1).Background(color) // 竖线：被 Stretch 拉满整棵子树高度
		ui.Column(c).Grow(1).Children(fn)
	})
}

// treeBranch 子节点行首的横折线（与 guide 竖线相接，形成 ├─ 效果）。

// treeBranch 子节点行首的横折线（与 guide 竖线相接，形成 ├─ 效果）。
func treeBranch(c *ui.Context, t *ui.Theme) {
	ui.Box(c).Width(10).Height(1).Background(t.Border.Mix(t.Surface, 0.2))
}

// keyValueRows 渲染一个通用的 key=value 映射编辑器：逐行展示已有键值对（可编辑、删除），
// 底部提供新增行。newKey/newValue 为「新行」输入缓冲，由调用方持有以跨帧保留。
// 供全局变量、环境变量等复用，消除重复的增删改样板。

// variablesEditor 可视化编辑全局变量（key=value），直接读写 scenario.Variables。
func (a *App) variablesEditor(c *ui.Context, t *ui.Theme) {
	if a.scenario.Variables == nil {
		a.scenario.Variables = map[string]string{}
	}
	keyValueRows(c, t, a.scenario.Variables, &a.newVarKey, &a.newVarValue,
		"用 {{key}} 在步骤中引用", 78)
	// 可用变量速查：聚合全局变量 + 各步骤提取变量，帮助定位可引用名。
	fieldHint(c, t, varSummary(collectVarNames(a.scenario)))
}

func (a *App) addMenu(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Padding(8, 12).Gap(4).Background(t.Surface).Radius(8).Children(func() {
		ui.Text(c, "选择步骤类型").FontSize(10).TextColor(t.TextMuted)
		types := []struct {
			label string
			typ   model.StepType
		}{
			{"HTTP 请求", model.StepRequest},
			{"TCP Socket", model.StepTCP},
			{"条件分支", model.StepBranch},
			{"多分支 switch", model.StepSwitch},
			{"循环 / 翻页", model.StepLoop},
			{"并行扇出", model.StepParallel},
			{"扇入合并(门控)", model.StepMerge},
			{"内置处理", model.StepProcess},
			{"自定义脚本", model.StepScript},
			{"独立断言", model.StepAssert},
			{"导出结果", model.StepExport},
		}
		for _, item := range types {
			it := item
			if ui.Button(c, "+ "+it.label).Clicked() {
				a.appendStep(it.typ)
				a.adding = false
			}
		}
	})
}
