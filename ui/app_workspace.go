package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

// ---------- 场景工作区：一个目录下的多个场景 JSON，点击列表切换编辑 ----------

// workspaceState 工作区的全部可变状态，收进结构体避免 App 主文件触及行数门禁。
type workspaceState struct {
	dir     string
	files   []string // 工作区目录下所有 .json 文件的完整路径
	show    bool     // 工作区分组是否展开
	newBuf  string   // 新建场景名输入缓冲
	newOpen bool     // 新建场景弹窗开关
}

// workspacePanel 侧栏顶部的场景工作区分组：目录名 + 场景文件列表 + 新建。
// 未设置工作区时显示引导，设置后列出目录内所有 .json 场景。
func (a *App) workspacePanel(c *ui.Context, t *ui.Theme) {
	a.treeFolderRow(c, t, "folder-workspace", &a.workspace.show, iconFolder, brandOrange, func() {
		label := "场景工作区"
		if a.workspace.dir != "" {
			label = "工作区: " + filepath.Base(a.workspace.dir)
		}
		ui.Text(c, label).FontSize(11).Bold().SingleLine()
		ui.Text(c, "("+itoa(len(a.workspace.files))+")").FontSize(10).TextColor(t.TextMuted)
	}, func() {
		if treeTextButton(c, t, "ws-open", "⋯", "打开工作区目录", t.TextMuted, 22).Clicked() {
			a.openWorkspace()
		}
	})
	if !a.workspace.show {
		return
	}
	if a.workspace.dir == "" {
		treeLine(c, "ws-empty", nil, nil, func() {
			ui.Text(c, "未设置工作区，点 ⋯ 选择目录").FontSize(10).TextColor(t.TextMuted)
		}, nil)
		return
	}
	// 场景文件列表：点击切换加载。
	for _, path := range a.workspace.files {
		p := path
		a.workspaceFileRow(c, t, p)
	}
	// 新建场景入口。
	treeLine(c, "ws-new", nil, nil, func() {
		add := ui.ButtonBase(c.Key("add")).Grow(1).Padding(2, 0).Radius(4).
			Justify(ui.Start).AlignItems(ui.Center).Cursor(ui.CursorPointer)
		if add.Hovered() {
			add.Background(t.SurfaceHover)
		}
		add.Children(func() {
			ui.Text(c, "＋ 新建场景").FontSize(11).TextColor(t.TextMuted)
		})
		if add.Clicked() {
			a.openWorkspaceNew()
		}
	}, nil)
}

// workspaceFileRow 单个场景文件节点：选中项高亮，点击切换加载该场景。
func (a *App) workspaceFileRow(c *ui.Context, t *ui.Theme, path string) {
	active := path == a.filePath
	name := scenarioNameFromFile(path)
	treeLine(c, "ws-file-"+path, nil,
		func() { ui.Icon(c, iconFile).Size(13, 13).TextColor(t.TextMuted) },
		func() {
			row := ui.ButtonBase(c.Key("name")).Grow(1).Padding(2, 0).Radius(4).
				Justify(ui.Start).AlignItems(ui.Center).Cursor(ui.CursorPointer).
				Tooltip("点击切换到此场景：" + path)
			if active {
				row.Background(t.Accent.Mix(t.Surface, 0.86))
			} else if row.Hovered() {
				row.Background(t.SurfaceHover)
			}
			row.Children(func() {
				nb := ui.Text(c, name).FontSize(11).SingleLine()
				if active {
					nb.Bold().TextColor(t.Accent)
				}
			})
			if row.Clicked() && !active {
				a.switchWorkspaceScene(path)
			}
		}, nil)
}

// openWorkspace 弹出目录选择对话框，选中后扫描其中的场景 JSON。
func (a *App) openWorkspace() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Title:     "选择场景工作区目录",
			Directory: true,
		})
		if err != nil || len(paths) == 0 {
			return
		}
		dir := paths[0]
		mygo.RunOnMain(func() { a.setWorkspace(dir) })
	}()
}

// setWorkspace 设置工作区目录并扫描其中的 .json 场景文件。
func (a *App) setWorkspace(dir string) {
	a.workspace.dir = dir
	a.workspace.show = true
	a.scanWorkspace()
	a.log("工作区已打开: " + dir + "（" + itoa(len(a.workspace.files)) + " 个场景）")
}

// scanWorkspace 扫描工作区目录下的 .json 文件，按文件名排序。
func (a *App) scanWorkspace() {
	a.workspace.files = nil
	if a.workspace.dir == "" {
		return
	}
	entries, err := os.ReadDir(a.workspace.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".json") {
			continue
		}
		a.workspace.files = append(a.workspace.files, filepath.Join(a.workspace.dir, e.Name()))
	}
	sort.Slice(a.workspace.files, func(i, j int) bool {
		return strings.ToLower(a.workspace.files[i]) < strings.ToLower(a.workspace.files[j])
	})
}

// switchWorkspaceScene 切换工作区场景：先保存当前场景，再从目标文件加载。
func (a *App) switchWorkspaceScene(path string) {
	if a.filePath != "" {
		a.saveTo(a.filePath)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		mygo.Dialog.Error("打开场景失败", err.Error())
		return
	}
	var sc model.Scenario
	if err := json.Unmarshal(data, &sc); err != nil {
		mygo.Dialog.Error("解析场景失败", err.Error())
		return
	}
	a.applyLoadedScenario(&sc, path)
	a.log("已切换场景: " + scenarioNameFromFile(path))
}

// openWorkspaceNew 打开「新建场景」弹窗，预填默认名。
func (a *App) openWorkspaceNew() {
	a.workspace.newBuf = "新场景"
	a.workspace.newOpen = true
}

// workspaceNewModal 在工作区目录下新建场景 JSON，并立即切换过去。
func (a *App) workspaceNewModal(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Width(420).Padding(20).Gap(12).Background(t.Surface).Radius(12).Children(func() {
		ui.Text(c, "在工作区新建场景").FontSize(16).Bold()
		fieldHint(c, t, "将在工作区目录创建同名 .json 文件，创建后立即打开编辑。")
		ui.TextInput(c, &a.workspace.newBuf).Placeholder("场景名称").Label("名称")
		ui.Row(c).Gap(8).Children(func() {
			ui.Spacer(c)
			if ui.Button(c, "取消").Clicked() {
				a.workspace.newOpen = false
			}
			if ui.PrimaryButton(c, "创建").Clicked() {
				a.createWorkspaceScene()
			}
		})
	})
}

// createWorkspaceScene 真正创建场景文件并切换。
func (a *App) createWorkspaceScene() {
	name := strings.TrimSpace(a.workspace.newBuf)
	if name == "" || a.workspace.dir == "" {
		return
	}
	path := filepath.Join(a.workspace.dir, safeFileName(name)+".json")
	if _, err := os.Stat(path); err == nil {
		mygo.Dialog.Error("创建失败", "同名场景文件已存在: "+path)
		return
	}
	if a.filePath != "" {
		a.saveTo(a.filePath)
	}
	sc := model.NewScenario(name)
	a.scenario = sc
	a.filePath = path
	a.selectedStep = -1
	a.resetResult()
	a.saveTo(path)
	a.scanWorkspace()
	a.workspace.newOpen = false
	a.log("已新建场景: " + name)
}

// scenarioNameFromFile 从文件名派生场景显示名（去掉 .json 扩展名）。
func scenarioNameFromFile(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// safeFileName 去掉文件名中不合法的路径字符，避免创建失败或越界。
func safeFileName(name string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return strings.TrimSpace(replacer.Replace(name))
}

// itoa 把整数转字符串（工作区计数显示用，避免引入 strconv 依赖样板）。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
