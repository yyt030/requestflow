package ui

import (
	"os"
	"path/filepath"
	"testing"

	"requestflow/model"
)

// TestPersistNewScenario 验证新建场景必定落盘到工作区、加入 files 列表并展开节点，
// 且即使 workspace.dir 为空（首次启动）也能自动创建默认工作区目录。
func TestPersistNewScenario(t *testing.T) {
	dir := t.TempDir()
	a := &App{
		scenario:      model.NewScenario("新场景"),
		workspace:     workspaceState{dir: dir},
		sceneExpanded: map[string]bool{},
	}
	path := a.persistNewScenario("订单接口")
	if path == "" {
		t.Fatal("persistNewScenario 返回空路径，落盘失败")
	}
	want := filepath.Join(dir, "订单接口.json")
	if path != want {
		t.Fatalf("落盘路径 = %q, 期望 %q", path, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("场景文件未写入磁盘: %v", err)
	}
	if len(a.workspace.files) != 1 || a.workspace.files[0] != want {
		t.Fatalf("工作区文件列表 = %v, 期望包含 %q", a.workspace.files, want)
	}
	if !a.sceneExpanded[want] {
		t.Fatal("新场景节点未展开，侧栏不会出现文件夹内容")
	}
	if a.filePath != want {
		t.Fatalf("filePath = %q, 期望 %q", a.filePath, want)
	}
}

// TestPersistNewScenarioNoWorkspaceDir 验证 workspace.dir 为空时自动创建默认目录，
// 不再静默失败（这正是「新建场景功能无效」的回归测试）。
func TestPersistNewScenarioNoWorkspaceDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	a := &App{
		scenario:      model.NewScenario("新场景"),
		sceneExpanded: map[string]bool{},
	}
	if a.workspace.dir != "" {
		t.Fatal("前置条件：workspace.dir 应为空")
	}
	path := a.persistNewScenario("空白场景")
	if path == "" {
		t.Fatal("workspace.dir 为空时新建场景静默失败")
	}
	if a.workspace.dir == "" {
		t.Fatal("未创建默认工作区目录")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("默认工作区中场景文件未写入: %v", err)
	}
	if len(a.workspace.files) == 0 {
		t.Fatal("工作区文件列表为空，侧栏不会出现场景文件夹")
	}
}

// TestPersistNewScenarioDuplicateName 验证同名场景自动加序号，不覆盖已有文件。
func TestPersistNewScenarioDuplicateName(t *testing.T) {
	dir := t.TempDir()
	a := &App{
		scenario:      model.NewScenario("新场景"),
		workspace:     workspaceState{dir: dir},
		sceneExpanded: map[string]bool{},
	}
	p1 := a.persistNewScenario("同名")
	p2 := a.persistNewScenario("同名")
	if p1 == p2 {
		t.Fatalf("同名场景路径重复，会覆盖文件: %q", p1)
	}
	if filepath.Base(p2) != "同名_2.json" {
		t.Fatalf("第二个场景文件名 = %q, 期望 同名_2.json", filepath.Base(p2))
	}
	if len(a.workspace.files) != 2 {
		t.Fatalf("工作区文件数 = %d, 期望 2", len(a.workspace.files))
	}
}
