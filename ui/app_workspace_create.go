package ui

import (
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

// ensureWorkspaceDir 保证有一个可用的工作区目录：优先用已设置的目录，
// 否则用当前场景所在目录，再否则创建默认目录（用户主目录下）。
// 这样「新建场景」在任何启动状态下都能落盘，不再因目录为空而静默失败。
func (a *App) ensureWorkspaceDir() string {
	if a.workspace.dir != "" {
		return a.workspace.dir
	}
	dir := ""
	if a.filePath != "" {
		dir = filepath.Dir(a.filePath)
	} else {
		dir = defaultWorkspaceDir()
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ""
	}
	a.workspace.dir = dir
	a.workspace.show = true
	a.scanWorkspace()
	return dir
}

// defaultWorkspaceDir 返回默认场景工作区目录。
func defaultWorkspaceDir() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, "RequestFlow", "scenes")
	}
	if wd, err := os.Getwd(); err == nil {
		return filepath.Join(wd, "scenes")
	}
	return "scenes"
}

// persistNewScenario 把当前内存中的场景落盘为工作区内的新 .json 文件，
// 刷新工作区列表并展开对应节点，保证侧栏树立即出现该场景文件夹。
// 同名文件自动加序号后缀避免覆盖；成功返回路径，失败返回空串。
func (a *App) persistNewScenario(name string) string {
	if a.ensureWorkspaceDir() == "" {
		mygo.Dialog.Error("创建失败", "无法创建工作区目录，请检查磁盘权限")
		return ""
	}
	base := safeFileName(name)
	if base == "" {
		base = "新场景"
	}
	path := filepath.Join(a.workspace.dir, base+".json")
	for n := 2; ; n++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			break
		}
		path = filepath.Join(a.workspace.dir, base+"_"+itoa(n)+".json")
	}
	a.filePath = path
	a.saveTo(path)
	a.scanWorkspace()
	if a.sceneExpanded == nil {
		a.sceneExpanded = map[string]bool{}
	}
	a.sceneExpanded[path] = true
	return path
}
