package engine

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"net"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"requestflow/model"
)

// DebugTCPStep 对单个 TCP 步骤发起一次独立调试连接，不依赖 Engine 运行态
// （连接池 / 变量上下文 / 日志 / 提取断言脚本），供界面「试发送此步骤」使用。
// 流程与 runTCPStep 的收发部分一致：渲染变量 → 拨号 → 组包发送 → 读取响应；
// 但与场景运行隔离，long/short 在调试中一律按一次性短连接处理，调试结束即关闭。
func DebugTCPStep(step *model.Step, vars map[string]string) *DebugResult {
	ctx := NewContext(vars)
	start := time.Now()
	res := &DebugResult{Protocol: "tcp", Method: "TCP"}

	host := ctx.Render(step.TCPHost)
	port := ctx.Render(step.TCPPort)
	addr := net.JoinHostPort(host, port)
	res.URL = addr

	sendMode := strings.ToLower(strings.TrimSpace(ctx.Render(step.TCPSendMode)))
	readMode := strings.ToLower(strings.TrimSpace(ctx.Render(step.TCPReadMode)))
	timeoutMS, _ := strconv.Atoi(strings.TrimSpace(ctx.Render(step.TCPTimeout)))
	if timeoutMS <= 0 {
		timeoutMS = 5000
	}
	timeout := time.Duration(timeoutMS) * time.Millisecond
	keepAliveSec, _ := strconv.Atoi(strings.TrimSpace(ctx.Render(step.TCPKeepAlive)))
	if keepAliveSec < 0 {
		keepAliveSec = 0
	}
	fs := parseFrameSpecWith(ctx, *step)

	// 关闭连接类步骤没有收发，给出明确提示而非空结果。
	if step.TCPClose {
		res.Err = "该步骤用于关闭长连接，无可发送的调试数据；请选择带发送内容的 TCP 步骤。"
		return res
	}
	conn, err := dialTCP(host, addr, timeout, step.TCPTLS)
	if err != nil {
		res.Err = "TCP 连接 " + addr + " 失败: " + err.Error()
		return res
	}
	defer conn.Close()
	applyTCPKeepAlive(conn, keepAliveSec)
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		res.Err = "TCP 设置超时失败: " + err.Error()
		return res
	}

	payload, err := buildTCPPayload(ctx.Render(step.TCPSend), sendMode, fs)
	if err != nil {
		res.Err = err.Error()
		return res
	}
	res.Sent = payload
	if len(payload) > 0 {
		if _, err := conn.Write(payload); err != nil {
			res.Err = "TCP 发送数据失败: " + err.Error()
			return res
		}
	}

	readNum, _ := strconv.Atoi(strings.TrimSpace(ctx.Render(step.TCPReadNum)))
	data, err := readTCP(conn, readMode, readNum, unescapeTCP(ctx.Render(step.TCPUntil)), fs)
	res.DurationMS = time.Since(start).Milliseconds()
	if err != nil {
		res.Err = "TCP 读取响应失败: " + err.Error()
		if len(data) > 0 {
			fillTCPBody(res, data)
		}
		return res
	}

	res.OK = true
	res.StatusCode = 200
	res.StatusText = "OK"
	fillTCPBody(res, data)
	return res
}

// fillTCPBody 填充响应体展示：JSON 美化；含不可打印字节时按十六进制展示；否则原文。
func fillTCPBody(res *DebugResult, data []byte) {
	res.Size = len(data)
	res.ReceivedRaw = append([]byte(nil), data...)

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && json.Valid(trimmed) {
		var buf bytes.Buffer
		if json.Indent(&buf, trimmed, "", "  ") == nil {
			res.Formatted = buf.String()
			res.Body = string(data)
			res.IsJSON = true
			return
		}
	}
	if !printableText(data) {
		res.IsBinary = true
		res.Formatted = hex.Dump(data)
		res.Body = hex.EncodeToString(data)
		return
	}
	res.Formatted = string(data)
	res.Body = string(data)
}

// printableText 判断字节序列是否为可显示的 UTF-8 文本（控制字符仅允许常见空白）。
func printableText(data []byte) bool {
	if !utf8.Valid(data) {
		return false
	}
	for _, r := range string(data) {
		if r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}
