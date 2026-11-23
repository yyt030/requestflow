package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

// tcpFrameTypeLabels 是帧头类型下拉的中文展示项；存盘值由 tcpFrameTypeValue 映射为英文。
var tcpFrameTypeLabels = []string{"不启用", "字符串头(位数自定义,左补零)", "1字节二进制头(≤255)", "2字节二进制头(64K)"}

func tcpFrameTypeLabel(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "ascii", "ascii8", "string", "string8":
		return "字符串头(位数自定义,左补零)"
	case "bin1", "binary1", "byte":
		return "1字节二进制头(≤255)"
	case "bin2", "binary2", "binary":
		return "2字节二进制头(64K)"
	default:
		return "不启用"
	}
}

func tcpFrameTypeValue(label string) string {
	switch label {
	case "字符串头(位数自定义,左补零)":
		return "ascii"
	case "1字节二进制头(≤255)":
		return "bin1"
	case "2字节二进制头(64K)":
		return "bin2"
	default:
		return ""
	}
}

// tcpTabView 按 Postman 风格标签渲染 TCP 步骤的单个配置维度。
func (a *App) tcpTabView(c *ui.Context, t *ui.Theme, step *model.Step, tab string) {
	switch tab {
	case "常规":
		a.tcpConnView(c, t, step)
	case "帧头":
		a.tcpFrameView(c, t, step)
	case "发送":
		a.tcpSendView(c, t, step)
	case "读取":
		a.tcpReadView(c, t, step)
	case "提取":
		a.editorExtract(c, t, step)
	case "断言":
		a.editorAssertList(c, t, step)
	case "脚本":
		a.editorRequestScripts(c, t, step)
	case "调试":
		ui.Text(c, "单步调试").FontSize(12).Bold()
		fieldHint(c, t, "独立于场景运行，按当前配置连一次、发一次、收一次并展示结果，不影响场景状态。")
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if ui.Button(c, "⚡ 试发送此步骤").Clicked() {
				a.debugTCPStep(step)
			}
		})
		a.tcpDebugResultView(c, t, step)
	}
}

func (a *App) tcpConnView(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Text(c, "连接目标").FontSize(12).Bold()
	fieldHint(c, t, "连接 TCP 服务端。主机与端口支持 {{变量}}；如需 TLS 加密勾选 TLS。keepalive 填探测周期秒数（如 30），0=关闭（默认）。")
	ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
		ui.TextInput(c, &step.TCPHost).Placeholder("127.0.0.1 或 {{host}}").Grow(2).Label("主机 Host")
		ui.TextInput(c, &step.TCPPort).Placeholder("8080").Grow(1).Label("端口 Port")
		ui.TextInput(c, &step.TCPKeepAlive).Placeholder("0=关闭, 如 30").Grow(1).Label("Keepalive(秒)")
		ui.Checkbox(c, &step.TCPTLS, "TLS")
	})

	ui.Divider(c)
	ui.Text(c, "连接模式（长连接 / 短连接）").FontSize(12).Bold()
	fieldHint(c, t, "short=短连接，每步独立建立、发完即关；long=长连接，按「连接名」跨步骤复用同一 socket，场景结束自动关闭。勾选「关闭连接」可显式释放指定长连接。")
	ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
		connModes := []string{"short", "long"}
		cmode := step.TCPConnMode
		if cmode == "" {
			cmode = "short"
		}
		if ui.Select(c, &cmode, connModes).Label("连接模式").Width(140).Changed() {
			step.TCPConnMode = cmode
		}
		ui.TextInput(c, &step.TCPConnKey).Placeholder("留空为默认连接").Grow(1).Label("连接名 (long 复用键)")
		ui.Checkbox(c, &step.TCPClose, "关闭连接")
	})
}

func (a *App) tcpFrameView(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Text(c, "帧头类型").FontSize(12).Bold()
	fieldHint(c, t, "字符串头：位数可自定义（如 3/5/8），十进制长度字面值不足左补零，如 8 位头+8 字节报文→00000008。1 字节二进制头：最大 255 字节。2 字节二进制头：最大 65535 字节（64K-1），可切换字节序。仅在发送/读取选择 frame 模式时生效。")
	ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
		ftype := step.TCPFrameType
		if ftype == "" && strings.TrimSpace(step.TCPFrameHead) != "" {
			ftype = "ascii" // 兼容旧场景：仅配置了位数时按 ascii 展示
		}
		ftypeLabel := tcpFrameTypeLabel(ftype)
		if ui.Select(c, &ftypeLabel, tcpFrameTypeLabels).Label("帧头类型").Width(230).Changed() {
			step.TCPFrameType = tcpFrameTypeValue(ftypeLabel)
			if step.TCPFrameType == "" {
				step.TCPFrameHead = "" // 防止旧位数触发兼容推断，令“不启用”失效
			}
		}
		switch ftype {
		case "ascii":
			ui.TextInput(c, &step.TCPFrameHead).Placeholder("8").Grow(1).Label("位数 (如 3/5/8)")
		case "bin2":
			endianLabel := "大端 (网络序)"
			if strings.EqualFold(strings.TrimSpace(step.TCPFrameEndian), "little") {
				endianLabel = "小端"
			}
			if ui.Select(c, &endianLabel, []string{"大端 (网络序)", "小端"}).Label("字节序").Width(140).Changed() {
				if endianLabel == "小端" {
					step.TCPFrameEndian = "little"
				} else {
					step.TCPFrameEndian = "big"
				}
			}
		}
	})
}

func (a *App) tcpSendView(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Text(c, "发送数据").FontSize(12).Bold()
	fieldHint(c, t, "支持 {{变量}} 与转义 \\n \\r\\n \\t \\xNN。line=末尾补换行；raw=原样；hex=十六进制；frame=按帧头类型自动加长度头。")
	ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
		sendModes := []string{"line", "raw", "hex", "frame"}
		mode := step.TCPSendMode
		if mode == "" {
			mode = "line"
		}
		if ui.Select(c, &mode, sendModes).Label("发送模式").Width(150).Changed() {
			step.TCPSendMode = mode
		}
	})
	resizableTextArea(c, t, &step.TCPSend, &a.tcpSendHeight,
		"例如：{\"cmd\":\"ping\"} 或 HELLO", 56, 400)
}

func (a *App) tcpReadView(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Text(c, "读取响应").FontSize(12).Bold()
	fieldHint(c, t, "line=读一行；all=读到连接关闭/超时；fixed=读固定字节；until=读到结束标记；frame=按帧头类型先读长度头再读定长报文。响应以文本供提取/断言使用。")
	ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
		readModes := []string{"line", "all", "fixed", "until", "frame"}
		rmode := step.TCPReadMode
		if rmode == "" {
			rmode = "line"
		}
		if ui.Select(c, &rmode, readModes).Label("读取模式").Width(150).Changed() {
			step.TCPReadMode = rmode
		}
		ui.TextInput(c, &step.TCPReadNum).Placeholder("如 1024").Grow(1).Label("字节数 (fixed)")
		ui.TextInput(c, &step.TCPUntil).Placeholder("如 \\n 或 >").Grow(1).Label("结束标记 (until)")
		ui.TextInput(c, &step.TCPTimeout).Placeholder("5000").Grow(1).Label("超时 ms")
	})
}
