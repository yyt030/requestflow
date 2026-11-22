package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

// ---------- 左侧：VSCode 资源管理器风格的紧凑树 ----------
// 场景/步骤/变量都是一行行树节点（图标 + 文字），没有常驻的大输入框；
// 需要编辑名字或 key=value 时点击节点弹出 Modal，编辑完即关闭。
func (a *App) sidebar(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Fill().Children(func() {
		// 面板标题栏。
		ui.Row(c).Padding(12, 12, 8, 12).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconFolder).Size(16, 16).TextColor(brandOrange)
			ui.Text(c, "资源管理器").FontSize(13).Bold()
			ui.Spacer(c)
			ui.Text(c, fmt.Sprintf("%d 项", len(a.scenario.Steps))).FontSize(10).TextColor(t.TextMuted)
		})
		// 搜索框：过滤步骤（VSCode 资源管理器的过滤栏）。
		ui.Row(c).Padding(0, 12, 10, 12).Children(func() {
			ui.SearchField(c, &a.stepSearch).Placeholder("搜索步骤…")
		})
		// 树区域。
		ui.Scroll(c).Grow(1).Padding(2, 8, 10, 8).Children(func() {
			a.fileTree(c, t)
		})
	})
}

// fileTree 两棵可折叠根节点：场景（步骤）与变量（环境 + 全局变量）。
func (a *App) fileTree(c *ui.Context, t *ui.Theme) {
	// 根节点：场景名（点击弹出重命名 Modal）+ 步骤子节点。
	a.treeFolderRow(c, t, "folder-scene", &a.showSteps, iconFolder, brandOrange, func() {
		a.sceneNameNode(c, t)
	}, func() {
		if treeTextButton(c, t, "rename", "✎", "重命名场景", t.TextMuted, 22).Clicked() {
			a.openNameEdit()
		}
	})
	if a.showSteps {
		a.stepList(c, t)
	}

	ui.Box(c).Height(8)

	// 根节点：变量（环境选择器 + 全局变量 + 当前环境变量）。
	varCount := len(a.scenario.Variables)
	if env := a.scenario.EnvVars(a.scenario.ActiveEnv); env != nil {
		varCount += len(env)
	}
	a.treeFolderRow(c, t, "folder-vars", &a.showVars, iconFolder, t.TextMuted, func() {
		ui.Text(c, "变量").FontSize(11).Bold()
		ui.Text(c, fmt.Sprintf("(%d)", varCount)).FontSize(10).TextColor(t.TextMuted)
	}, nil)
	if a.showVars {
		a.environmentEditor(c, t)
	}
}

// treeLine 统一的树行布局：固定「箭头槽16 + 图标槽16 + 文字区(Grow)」+ 可选尾部。
// 所有层级（根节点/步骤/变量）都用它，文字起点恒为 4+16+4+16+4=44px，严格左对齐。
// 子节点没有折叠箭头时传 nil，箭头槽留空但占位，保证文字不错位。
func treeLine(c *ui.Context, key string, arrow, icon, body, tail func()) {
	ui.Row(c.Key(key)).Gap(4).Padding(3, 4).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Width(16).Height(18).Center().Children(func() {
			if arrow != nil {
				arrow()
			}
		})
		ui.Box(c).Width(16).Height(18).Center().Children(func() {
			if icon != nil {
				icon()
			}
		})
		ui.Row(c).Grow(1).AlignItems(ui.Center).Children(func() {
			if body != nil {
				body()
			}
		})
		if tail != nil {
			tail()
		}
	})
}

// treeFolderRow 文件夹根节点行：折叠箭头 + 文件夹图标 + 标题 + 尾部操作。
func (a *App) treeFolderRow(c *ui.Context, t *ui.Theme, key string, open *bool, icon *ui.SVG, iconColor ui.Color, label func(), tail func()) {
	treeLine(c, key,
		func() {
			arrow := map[bool]string{true: "▾", false: "▸"}[*open]
			tog := ui.ButtonBase(c.Key("tog")).Width(16).Height(18).Radius(4).
				Justify(ui.Center).AlignItems(ui.Center).Cursor(ui.CursorPointer).Tooltip("展开/折叠")
			if tog.Hovered() {
				tog.Background(t.SurfaceHover)
			}
			tog.Children(func() {
				ui.Text(c, arrow).FontSize(10).TextColor(t.TextMuted)
			})
			if tog.Clicked() {
				*open = !*open
			}
		},
		func() { ui.Icon(c, icon).Size(14, 14).TextColor(iconColor) },
		label, tail)
}

// treeTextButton 树节点内的轻量文字按钮：默认透明无边框，仅 hover 时浅色高亮。
// 用于侧栏的重命名/删除/新建等操作，避免标准 Button 的灰底方块破坏树的纯文本观感。
func treeTextButton(c *ui.Context, t *ui.Theme, key, label, tip string, col ui.Color, width float32) ui.Element {
	b := ui.ButtonBase(c.Key(key)).Radius(4).Padding(2, 4).
		Label(tip).Tooltip(tip).Cursor(ui.CursorPointer).Justify(ui.Center).AlignItems(ui.Center)
	if width > 0 {
		b.Width(width)
	}
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Text(c, label).FontSize(11).TextColor(col)
	})
	return b
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
