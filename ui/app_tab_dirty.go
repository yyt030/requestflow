package ui

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

// canonicalFingerprint 把字段快照序列化为"语义指纹"：空的 map/slice/对象统一归一为 null。
// 目的：让"从没填过"(nil) 与"填了又清空"(空容器) 得到相同指纹，撤销编辑后圆圈能自动消失。
// 用 json.Number 保留数字原始文本，避免 float64 精度问题。
func canonicalFingerprint(v any) string {
	raw, _ := json.Marshal(v)
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var node any
	_ = dec.Decode(&node)
	out, _ := json.Marshal(normalizeEmpty(node))
	return string(out)
}

// normalizeEmpty 递归把空 map / 空数组归一为 nil，非空容器保留并递归处理子元素。
func normalizeEmpty(n any) any {
	switch x := n.(type) {
	case map[string]any:
		if len(x) == 0 {
			return nil
		}
		for k, v := range x {
			x[k] = normalizeEmpty(v)
		}
		return x
	case []any:
		if len(x) == 0 {
			return nil
		}
		for i, v := range x {
			x[i] = normalizeEmpty(v)
		}
		return x
	default:
		return n
	}
}

// 标签修改标记：为每个步骤的每个配置标签维护一份"已确认内容指纹"。
// 渲染标签栏时，把当前内容指纹与基线对比，不同则在标签名后加 ● 标记，
// 让用户一眼看出哪些标签改过。基线在场景加载/新建/保存时刷新。

// tabKey 生成基线的 key：步骤 ID + 标签名。ID 为空时用指针地址兜底。
func tabKey(step *model.Step, tab string) string {
	id := step.ID
	if id == "" {
		id = fmt.Sprintf("%p", step)
	}
	return id + "|" + tab
}

// tabFingerprint 把某个标签涉及的字段序列化为指纹字符串。
// 字段按标签维度选取，改了哪块的字段，哪块的指纹就变。
func tabFingerprint(step *model.Step, tab string) string {
	return canonicalFingerprint(tabFields(step, tab))
}

// tabFields 返回标签对应的字段快照。用 []any 保序，json.Marshal 稳定。
func tabFields(step *model.Step, tab string) []any {
	switch step.Type {
	case model.StepRequest:
		return requestFields(step, tab)
	case model.StepTCP:
		return tcpFields(step, tab)
	case model.StepLoop:
		return loopFields(step, tab)
	case model.StepBranch:
		return branchFields(step, tab)
	case model.StepSwitch:
		return switchFields(step, tab)
	case model.StepParallel:
		return parallelFields(step, tab)
	case model.StepMerge:
		return []any{step.Merge}
	case model.StepProcess:
		return []any{step.Op, step.Args}
	case model.StepScript:
		return []any{step.ScriptLang, step.Code}
	case model.StepAssert:
		return []any{step.Assert}
	case model.StepExport:
		return []any{step.From, step.Format, step.Path}
	default:
		return nil
	}
}

func requestFields(step *model.Step, tab string) []any {
	switch tab {
	case "常规":
		return []any{step.Method, step.URL}
	case "Params":
		return []any{step.Query}
	case "Headers":
		return []any{step.Headers}
	case "Body":
		return []any{step.BodyType, step.Body, step.FormData, step.GraphQLQuery, step.GraphQLVariables}
	case "Auth":
		return []any{step.AuthType, step.AuthToken, step.AuthUser, step.AuthPass,
			step.AuthKeyName, step.AuthKeyValue, step.AuthKeyIn,
			step.AuthTokenURL, step.AuthClientID, step.AuthClientSecret, step.AuthScope, step.AuthGrantType}
	case "提取":
		return []any{step.Extract}
	case "断言":
		return []any{step.Assert}
	case "脚本":
		return []any{step.PreScript, step.PostScript}
	}
	return nil
}

func tcpFields(step *model.Step, tab string) []any {
	switch tab {
	case "常规":
		return []any{step.TCPHost, step.TCPPort, step.TCPConnMode, step.TCPConnKey,
			step.TCPClose, step.TCPKeepAlive, step.TCPTLS, step.TCPTimeout}
	case "帧头":
		return []any{step.TCPFrameType, step.TCPFrameHead, step.TCPFrameEndian}
	case "发送":
		return []any{step.TCPSend, step.TCPSendMode}
	case "读取":
		return []any{step.TCPReadMode, step.TCPReadNum, step.TCPUntil}
	case "提取":
		return []any{step.Extract}
	case "断言":
		return []any{step.Assert}
	case "脚本":
		return []any{step.PreScript, step.PostScript}
	}
	return nil // 调试标签是动作，无持久配置字段
}

func loopFields(step *model.Step, tab string) []any {
	lp := step.Loop
	switch tab {
	case "循环配置":
		if lp == nil {
			return nil
		}
		return []any{lp.Mode, lp.Count, lp.ListVar, lp.ItemVar, lp.IndexVar, lp.BreakWhen,
			lp.PageVar, lp.PageStart, lp.PageStep, lp.Max}
	case "去重":
		if lp == nil {
			return nil
		}
		return []any{lp.DedupVar, lp.DedupField, lp.DedupStat}
	case "健壮性":
		if lp == nil {
			return nil
		}
		return []any{lp.DelayMs, lp.MinRowsVar, lp.MinRows, lp.RetryOnEmpty}
	case "循环体":
		return []any{step.Children}
	}
	return nil
}

func branchFields(step *model.Step, tab string) []any {
	br := step.Branch
	if br == nil {
		return nil
	}
	switch tab {
	case "条件":
		return []any{br.Logic, br.Conditions}
	case "高级":
		return []any{br.When}
	case "then":
		return []any{br.Then}
	case "else":
		return []any{br.Else}
	}
	return nil
}

func switchFields(step *model.Step, tab string) []any {
	sw := step.Switch
	if sw == nil {
		return nil
	}
	switch tab {
	case "分支":
		return []any{sw.Cases}
	case "默认":
		return []any{sw.Default}
	}
	return nil
}

func parallelFields(step *model.Step, tab string) []any {
	p := step.Parallel
	if p == nil {
		return nil
	}
	switch tab {
	case "汇合配置":
		return []any{p.Var, p.Mode, p.Separator, p.FailFast, p.StatusVar}
	case "子任务":
		return []any{p.Tasks}
	}
	return nil
}

// tabIsDirty 判断标签相对基线是否被修改；首次见到的标签记录基线并视为干净。
func (a *App) tabIsDirty(step *model.Step, tab string) bool {
	key := tabKey(step, tab)
	cur := tabFingerprint(step, tab)
	base, ok := a.tabDirty[key]
	if !ok {
		a.tabDirty[key] = cur
		return false
	}
	return base != cur
}

// tabDirtyFlags 返回每个标签是否被修改，以及被修改的标签数量。
func (a *App) tabDirtyFlags(step *model.Step) ([]bool, int) {
	tabs := stepTabNames(step)
	flags := make([]bool, len(tabs))
	dirty := 0
	for i, name := range tabs {
		if a.tabIsDirty(step, name) {
			flags[i] = true
			dirty++
		}
	}
	return flags, dirty
}

// refreshTabBaseline 把当前所有步骤的所有标签指纹记为基线（表示"已确认"）。
func (a *App) refreshTabBaseline() {
	if a.tabDirty == nil {
		a.tabDirty = map[string]string{}
	}
	for i := range a.scenario.Steps {
		step := &a.scenario.Steps[i]
		for _, tab := range stepTabNames(step) {
			a.tabDirty[tabKey(step, tab)] = tabFingerprint(step, tab)
		}
	}
}

// dirtyDotView 在 dirty 标签名后画一个绿色圆点，标记该标签内容被改过。
func dirtyDotView(c *ui.Context, t *ui.Theme) {
	ui.Text(c, "•").FontSize(12).TextColor(t.Success).Tooltip("此标签内容已修改，未保存")
}

// tabBarWithDots 用 TabsBase 自定义渲染标签栏：选中态保留主题外观，
// 每个标签名后按需跟一个绿色小圆点。返回被修改的标签数量。
func (a *App) tabBarWithDots(c *ui.Context, t *ui.Theme, step *model.Step) int {
	tabs := stepTabNames(step)
	flags, dirtyCount := a.tabDirtyFlags(step)
	parts := ui.TabsBase(c, &a.stepTab, len(tabs))
	parts.List.Gap(2).Children(func() {
		for i, name := range tabs {
			i := i
			tab := parts.Tab(i).Padding(6, 12).Radius(6)
			if i == a.stepTab {
				tab.Background(t.Surface)
			}
			tab.Children(func() {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					label := ui.Text(c, name).FontSize(12)
					if i == a.stepTab {
						label.Bold().TextColor(t.Accent)
					}
					if flags[i] {
						dirtyDotView(c, t)
					}
				})
			})
		}
	})
	return dirtyCount
}
