package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"requestflow/engine"
	"requestflow/model"
)

// tcpDebugState 保存 TCP 步骤「试发送」的异步状态与结果。
type tcpDebugState struct {
	sending bool
	result  *engine.DebugResult
	stepRef *model.Step // 结果归属的步骤指针，切步骤时不复用旧结果
}

// debugTCPStep 用当前 TCP 步骤配置发起一次独立调试，不写回场景、不走运行引擎状态。
func (a *App) debugTCPStep(step *model.Step) {
	vars := map[string]string{"baseUrl": a.scenario.BaseURL}
	for k, v := range a.scenario.Variables {
		vars[k] = v
	}
	a.tcpDebug.sending = true
	a.tcpDebug.result = nil
	a.tcpDebug.stepRef = step
	go func() {
		res := engine.DebugTCPStep(step, vars)
		mygo.RunOnMain(func() {
			a.tcpDebug.sending = false
			a.tcpDebug.result = res
			if res.OK {
				a.log(fmt.Sprintf("[TCP 调试] %s → %d 字节 (%d ms)", res.URL, res.Size, res.DurationMS))
			} else {
				a.log("[TCP 调试] 失败: " + res.Err)
			}
		})
	}()
}

// tcpDebugResultView 在 TCP 编辑器内就地展示调试结果：概览 + 发送预览 + 响应体。
func (a *App) tcpDebugResultView(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Divider(c)
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "调试结果").FontSize(12).Bold()
		ui.Spacer(c)
		if ui.Button(c, "⧉ 复制响应").Clicked() {
			a.copyTCPDebugResult()
		}
	})
	if a.tcpDebug.sending {
		ui.Text(c, "连接发送中…").FontSize(11).TextColor(t.Accent)
		return
	}
	res := a.tcpDebug.result
	if res == nil || a.tcpDebug.stepRef != step {
		ui.Text(c, "点击上方【⚡ 试发送此步骤】，收发结果将显示在这里。").FontSize(11).TextColor(t.TextMuted)
		return
	}
	if !res.OK {
		ui.Text(c, "✗ "+res.Err).FontSize(11).Bold().TextColor(t.Danger)
		return
	}
	// 概览：状态 / 目标 / 耗时 / 字节数。
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Badge(c, "TCP 200 OK").TextColor(t.Success)
		ui.Text(c, res.URL).FontSize(11)
		ui.Text(c, fmt.Sprintf("%d ms", res.DurationMS)).FontSize(11).TextColor(t.TextMuted)
		ui.Text(c, fmt.Sprintf("收 %d B", res.Size)).FontSize(11).TextColor(t.TextMuted)
	})
	// 发送预览：十六进制展示实际发出的字节。
	if len(res.Sent) > 0 {
		ui.Text(c, fmt.Sprintf("已发送 %d 字节：", len(res.Sent))).FontSize(10).TextColor(t.TextMuted)
		sentPrev := hexPreview(res.Sent, 64)
		ui.Text(c, sentPrev).FontSize(10).TextColor(t.TextMuted).Selectable()
	}
	// 响应体：可选中复制；二进制为 hex dump，JSON 为美化文本。
	label := "响应内容"
	if res.IsBinary {
		label = "响应内容（二进制 · 十六进制）"
	} else if res.IsJSON {
		label = "响应内容（JSON 已格式化）"
	}
	ui.Text(c, label).FontSize(11).Bold()
	ui.Scroll(c).Grow(1).Height(200).Padding(0, 0, 32, 0).Children(func() {
		ui.Text(c, res.Formatted).FontSize(11).Selectable()
	})
}

// copyTCPDebugResult 复制 TCP 调试响应到剪贴板。
func (a *App) copyTCPDebugResult() {
	res := a.tcpDebug.result
	if res == nil || !res.OK {
		a.log("暂无可复制的 TCP 调试响应")
		return
	}
	text := res.Formatted
	if text == "" {
		a.log("响应为空，无可复制内容")
		return
	}
	_ = mygo.Clipboard.Write(transfer.New(transfer.NewItem(
		transfer.Bytes(transfer.Text, []byte(text)),
	)))
	a.log("TCP 调试响应已复制到剪贴板（" + itoa(len([]rune(text))) + " 字）")
}

// hexPreview 把字节序列渲染为紧凑的十六进制预览，超出上限以省略号结尾。
func hexPreview(data []byte, maxBytes int) string {
	limit := len(data)
	if limit > maxBytes {
		limit = maxBytes
	}
	var b strings.Builder
	for _, v := range data[:limit] {
		fmt.Fprintf(&b, "%02X ", v)
	}
	s := strings.TrimSpace(b.String())
	if len(data) > maxBytes {
		s += " …"
	}
	return s
}
