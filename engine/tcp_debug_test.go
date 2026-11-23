package engine

import (
	"bufio"
	"net"
	"strings"
	"testing"

	"requestflow/model"
)

func TestDebugTCP_LineEcho(t *testing.T) {
	host, port, closeFn := startTCPServer(t, echoLineServer)
	defer closeFn()

	step := &model.Step{
		Type: model.StepTCP, TCPHost: host, TCPPort: port,
		TCPSend: "hello", TCPSendMode: "line", TCPReadMode: "line", TCPTimeout: "2000",
	}
	res := DebugTCPStep(step, nil)
	if !res.OK {
		t.Fatalf("期望调试成功，实际失败: %s", res.Err)
	}
	if res.Protocol != "tcp" {
		t.Fatalf("Protocol 期望 tcp，实际 %q", res.Protocol)
	}
	if res.URL != net.JoinHostPort(host, port) {
		t.Fatalf("URL 期望 host:port，实际 %q", res.URL)
	}
	if !strings.Contains(res.Formatted, "ECHO:hello") {
		t.Fatalf("响应体期望含 ECHO:hello，实际 %q", res.Formatted)
	}
	if got := string(res.Sent); got != "hello\n" {
		t.Fatalf("发送字节期望 hello\\n，实际 %q", got)
	}
	if res.Size <= 0 {
		t.Fatalf("响应大小应大于 0，实际 %d", res.Size)
	}
}

func TestDebugTCP_RendersVariables(t *testing.T) {
	host, port, closeFn := startTCPServer(t, echoLineServer)
	defer closeFn()

	step := &model.Step{
		Type: model.StepTCP, TCPHost: "{{host}}", TCPPort: "{{port}}",
		TCPSend: "{{msg}}", TCPSendMode: "line", TCPReadMode: "line", TCPTimeout: "2000",
	}
	vars := map[string]string{"host": host, "port": port, "msg": "VAR_OK"}
	res := DebugTCPStep(step, vars)
	if !res.OK {
		t.Fatalf("变量渲染后应调试成功，实际: %s", res.Err)
	}
	if !strings.Contains(res.Formatted, "VAR_OK") {
		t.Fatalf("响应体期望含 VAR_OK，实际 %q", res.Formatted)
	}
}

func TestDebugTCP_BinaryHexView(t *testing.T) {
	// 服务端回显含不可打印字节的数据。
	host, port, closeFn := startTCPServer(t, func(conn net.Conn) {
		defer conn.Close()
		buf := make([]byte, 64)
		n, _ := conn.Read(buf)
		_, _ = conn.Write([]byte{0x00, 0x01, 0xff, 0xfe, byte(n)})
	})
	defer closeFn()

	step := &model.Step{
		Type: model.StepTCP, TCPHost: host, TCPPort: port,
		TCPSend: "00FF", TCPSendMode: "hex", TCPReadMode: "fixed", TCPReadNum: "5", TCPTimeout: "2000",
	}
	res := DebugTCPStep(step, nil)
	if !res.OK {
		t.Fatalf("期望调试成功，实际: %s", res.Err)
	}
	if !res.IsBinary {
		t.Fatalf("含不可打印字节应标记为二进制")
	}
	if !strings.Contains(res.Formatted, "0001 ff") && !strings.Contains(res.Formatted, "00 01 ff") {
		t.Fatalf("期望十六进制 dump，实际 %q", res.Formatted)
	}
}

func TestDebugTCP_ConnectionFailure(t *testing.T) {
	step := &model.Step{
		Type: model.StepTCP, TCPHost: "127.0.0.1", TCPPort: "1",
		TCPSend: "x", TCPTimeout: "500",
	}
	res := DebugTCPStep(step, nil)
	if res.OK {
		t.Fatalf("连接不存在端口应失败")
	}
	if !strings.Contains(res.Err, "连接") {
		t.Fatalf("错误信息应含「连接」，实际 %q", res.Err)
	}
}

func TestDebugTCP_CloseStepHint(t *testing.T) {
	step := &model.Step{Type: model.StepTCP, TCPHost: "127.0.0.1", TCPPort: "1", TCPClose: true}
	res := DebugTCPStep(step, nil)
	if res.OK {
		t.Fatalf("关闭连接步骤不应有成功调试结果")
	}
	if !strings.Contains(res.Err, "关闭") {
		t.Fatalf("应提示无可发送数据，实际 %q", res.Err)
	}
}

func TestPrintableText(t *testing.T) {
	cases := []struct {
		data []byte
		want bool
	}{
		{[]byte("hello\n中文"), true},
		{[]byte("{\"a\":1}"), true},
		{[]byte{0x00, 0xff}, false},
		{[]byte("a\x00b"), false},
	}
	for _, tc := range cases {
		if got := printableText(tc.data); got != tc.want {
			t.Fatalf("printableText(%v) = %v, want %v", tc.data, got, tc.want)
		}
	}
}

// 确保 bufio 服务端与固定读取组合可用（回归保护）。
func TestDebugTCP_FixedRead(t *testing.T) {
	host, port, closeFn := startTCPServer(t, func(conn net.Conn) {
		defer conn.Close()
		r := bufio.NewReader(conn)
		line, _ := r.ReadString('\n')
		_, _ = conn.Write([]byte("LEN:" + strings.TrimSpace(line)))
	})
	defer closeFn()
	step := &model.Step{
		Type: model.StepTCP, TCPHost: host, TCPPort: port,
		TCPSend: "abc", TCPSendMode: "line", TCPReadMode: "fixed", TCPReadNum: "7", TCPTimeout: "2000",
	}
	res := DebugTCPStep(step, nil)
	if !res.OK || res.Formatted != "LEN:abc" {
		t.Fatalf("固定读取期望 LEN:abc，实际 OK=%v body=%q err=%q", res.OK, res.Formatted, res.Err)
	}
}
