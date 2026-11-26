package ui

import (
	"encoding/json"
	"os"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

// newScenario 打开模板向导，让用户从模板或空白场景开始，降低手写 JSON 门槛。
func (a *App) newScenario() {
	a.showTemplateWizard = true
	a.templateIndex = 0
}

// applyTemplate 载入选中的场景模板、落盘到工作区并关闭向导，
// 保证工具栏「新建」入口选模板后侧栏同样出现可展开的场景文件夹节点。
func (a *App) applyTemplate(tpl ScenarioTemplate) {
	a.scenario = tpl.Build()
	a.selectedStep = -1
	a.mainTab = 0
	a.adding = false
	a.resetResult()
	a.showTemplateWizard = false
	path := a.persistNewScenario(a.scenario.Name)
	if path != "" {
		a.log("已从模板新建场景：" + a.scenario.Name + " → " + path)
	}
}

// resetResult 清空上一次运行的可视化结果状态。

// resetResult 清空上一次运行的可视化结果状态。
func (a *App) resetResult() {
	a.lastResult = nil
	a.resultVar = ""
	a.resultState = ui.ListState{}
}

func (a *App) openScenario() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Title:   "打开场景 JSON",
			Filters: []mygo.FileFilter{{Name: "JSON", Extensions: []string{"json"}}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		data, err := os.ReadFile(paths[0])
		if err != nil {
			mygo.Dialog.Error("打开失败", err.Error())
			return
		}
		var sc model.Scenario
		if err := json.Unmarshal(data, &sc); err != nil {
			mygo.Dialog.Error("解析失败", err.Error())
			return
		}
		path := paths[0]
		mygo.RunOnMain(func() { a.applyLoadedScenario(&sc, path) })
	}()
}

// applyLoadedScenario 统一应用外部载入的场景，保证各项状态一致、无半初始化。

// applyLoadedScenario 统一应用外部载入的场景，保证各项状态一致、无半初始化。
func (a *App) applyLoadedScenario(sc *model.Scenario, path string) {
	if sc == nil {
		sc = model.NewScenario("空场景")
	}
	if sc.Variables == nil {
		sc.Variables = map[string]string{}
	}
	if sc.Steps == nil {
		sc.Steps = []model.Step{}
	}
	a.scenario = sc
	a.filePath = path
	a.selectedStep = -1
	a.adding = false
	a.showImport = false
	// 请求字段内联在步骤中即真相源；打开场景不从集合覆盖，也不自动迁移。
	a.resetResult()
	a.refreshTabBaseline()
	a.log("已打开: " + path)
}

func (a *App) saveScenario() {
	if a.filePath == "" {
		go func() {
			path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{DefaultPath: "scenario.json"})
			if err != nil || path == "" {
				return
			}
			mygo.RunOnMain(func() { a.filePath = path; a.saveTo(path) })
		}()
		return
	}
	a.saveTo(a.filePath)
}

func (a *App) saveTo(path string) {
	data, err := json.MarshalIndent(a.scenario, "", "  ")
	if err != nil {
		mygo.Dialog.Error("保存失败", err.Error())
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		mygo.Dialog.Error("保存失败", err.Error())
		return
	}
	a.refreshTabBaseline() // 保存后把当前内容记为基线，清除所有 ● 修改标记
	// 统一树的场景步骤摘要来自缓存，保存后同步最新步骤，避免展开显示旧数据。
	if a.sceneStepCache == nil {
		a.sceneStepCache = map[string][]model.Step{}
	}
	a.sceneStepCache[path] = a.scenario.Steps
	a.log("已保存: " + path)
}

// pickDataFile 选择数据文件后立即启动数据驱动运行。

// pickDataFile 选择数据文件后立即启动数据驱动运行。
func (a *App) pickDataFile() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Title:   "选择数据文件 (CSV/JSON)",
			Filters: []mygo.FileFilter{{Name: "数据文件", Extensions: []string{"csv", "json", "txt"}}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		mygo.RunOnMain(func() {
			a.dataFile = paths[0]
			a.runDataDriven()
		})
	}()
}

// runDataDriven 用选中的数据集逐行运行场景，结果展示在运行结果标签。
