package ui

import (
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
)

// copyDebugResponse 按当前响应标签复制对应内容到剪贴板。
// Body 复制格式化后的原文，Headers/Cookies 拼成可读文本。
func (a *App) copyDebugResponse() {
	res := a.debugResult
	if res == nil {
		return
	}
	var text string
	var label string
	switch a.debugView {
	case 1: // Headers
		label = "响应头"
		var b strings.Builder
		for _, kv := range res.Headers {
			b.WriteString(kv.Key)
			b.WriteString(": ")
			b.WriteString(kv.Value)
			b.WriteByte('\n')
		}
		text = strings.TrimRight(b.String(), "\n")
	case 2: // Cookies
		label = "Cookie"
		var b strings.Builder
		for _, kv := range res.Cookies {
			b.WriteString(kv.Key)
			b.WriteString(" = ")
			b.WriteString(kv.Value)
			b.WriteByte('\n')
		}
		text = strings.TrimRight(b.String(), "\n")
	default: // Body：格式化后与展示一致
		label = "响应体"
		text = res.Formatted
	}
	if text == "" {
		a.log(label + "为空，无可复制内容")
		return
	}
	_ = mygo.Clipboard.Write(transfer.New(transfer.NewItem(
		transfer.Bytes(transfer.Text, []byte(text)),
	)))
	a.log(label + "已复制到剪贴板（" + itoa(len([]rune(text))) + " 字）")
}
