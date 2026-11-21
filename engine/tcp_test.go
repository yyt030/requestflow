package engine

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"requestflow/model"
)

// startTCPServer 启动一个简单的 TCP 服务，返回 host/port 与关闭函数。
// handler 接收每个连接，可自定义读写行为。
func startTCPServer(t *testing.T, handler func(net.Conn)) (string, string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("启动 TCP 服务失败: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handler(conn)
		}
	}()
	host, port, _ := net.SplitHostPort(ln.Addr().String())
	return host, port, func() { _ = ln.Close() }
}

// echoLineServer 逐行回显，并在每行前加 "ECHO:" 前缀。

// echoLineServer 逐行回显，并在每行前加 "ECHO:" 前缀。
func echoLineServer(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	for {
		line, err := r.ReadString('\n')
		if len(line) > 0 {
			_, _ = conn.Write([]byte("ECHO:" + line))
		}
		if err != nil {
			return
		}
	}
}

func TestTCPLineEcho(t *testing.T) {
	host, port, closeFn := startTCPServer(t, echoLineServer)
	defer closeFn()

	step := model.Step{
		ID: "tcp1", Name: "TCP 回显", Type: model.StepTCP, Enabled: true,
		TCPHost: host, TCPPort: port,
		TCPSend: "hello", TCPSendMode: "line", TCPReadMode: "line", TCPTimeout: "3000",
		Assert: []model.Assert{{Source: "body", Op: "contains", Value: "ECHO:hello"}},
	}
	res := NewEngine(&model.Scenario{Name: "s", Steps: []model.Step{step}}, nil).Run()
	if !res.OK {
		t.Fatalf("TCP 行回显应成功，失败: %v", res.Err)
	}
}

func TestTCPVariablesAndExtract(t *testing.T) {
	// 服务端收到 JSON 请求后回一个带 id 的 JSON 响应
	host, port, closeFn := startTCPServer(t, func(conn net.Conn) {
		defer conn.Close()
		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		if strings.Contains(string(buf[:n]), "ping") {
			_, _ = conn.Write([]byte(`{"code":0,"session":"sess-abc"}` + "\n"))
		}
	})
	defer closeFn()

	step := model.Step{
		ID: "tcp2", Name: "TCP 提取", Type: model.StepTCP, Enabled: true,
		TCPHost: "{{srvhost}}", TCPPort: "{{srvport}}",
		TCPSend: "cmd={{cmd}}", TCPSendMode: "line", TCPReadMode: "line", TCPTimeout: "3000",
		Extract: []model.Extract{
			{Name: "code", Source: "body", Kind: "jsonpath", Path: "code"},
			{Name: "sess", Source: "body", Kind: "jsonpath", Path: "session"},
		},
		Assert: []model.Assert{{Source: "body", Op: "contains", Value: "sess-abc"}},
	}
	sc := &model.Scenario{
		Name:      "s",
		Variables: map[string]string{"srvhost": host, "srvport": port, "cmd": "ping"},
		Steps:     []model.Step{step},
	}
	res := NewEngine(sc, nil).Run()
	if !res.OK {
		t.Fatalf("TCP 变量+提取应成功，失败: %v", res.Err)
	}
	if fmt.Sprint(res.Vars["sess"]) != "sess-abc" {
		t.Fatalf("提取 session 失败: %v", res.Vars["sess"])
	}
}

func TestTCPHexSendAndFixedRead(t *testing.T) {
	// 服务端读固定字节并以 hex 原样回显
	host, port, closeFn := startTCPServer(t, func(conn net.Conn) {
		defer conn.Close()
		buf := make([]byte, 4)
		_, _ = io.ReadFull(conn, buf)
		_, _ = conn.Write(buf)
	})
	defer closeFn()

	step := model.Step{
		ID: "tcp3", Name: "TCP hex", Type: model.StepTCP, Enabled: true,
		TCPHost: host, TCPPort: port,
		TCPSend: "DEADBEEF", TCPSendMode: "hex", TCPReadMode: "fixed", TCPReadNum: "4", TCPTimeout: "3000",
		Assert: []model.Assert{{Source: "body", Op: "eq", Value: string([]byte{0xDE, 0xAD, 0xBE, 0xEF})}},
	}
	res := NewEngine(&model.Scenario{Name: "s", Steps: []model.Step{step}}, nil).Run()
	if !res.OK {
		t.Fatalf("TCP hex 发送+固定读取应成功，失败: %v", res.Err)
	}
}

func TestTCPTimeout(t *testing.T) {
	// 服务端接受连接但永不发送数据，验证读取超时不 panic（all 模式超时返回已读数据）
	host, port, closeFn := startTCPServer(t, func(conn net.Conn) {
		defer conn.Close()
		time.Sleep(2 * time.Second)
	})
	defer closeFn()

	step := model.Step{
		ID: "tcp4", Name: "TCP 超时", Type: model.StepTCP, Enabled: true,
		TCPHost: host, TCPPort: port,
		TCPSend: "hi", TCPSendMode: "line", TCPReadMode: "all", TCPTimeout: "300",
	}
	res := NewEngine(&model.Scenario{Name: "s", Steps: []model.Step{step}}, nil).Run()
	if !res.OK {
		t.Fatalf("TCP 读取超时应作为正常结束，失败: %v", res.Err)
	}
}

func TestUnescapeTCP(t *testing.T) {
	cases := map[string]string{
		"a\\nb":      "a\nb",
		"a\\r\\nb":   "a\r\nb",
		"\\x41\\x42": "AB",
		"a\\\\b":     "a\\b",
		"plain":      "plain",
	}
	for in, want := range cases {
		if got := unescapeTCP(in); got != want {
			t.Errorf("unescapeTCP(%q) = %q, 期望 %q", in, got, want)
		}
	}
}

// TestFrameLengthHead 验证 N 位十进制长度头左补零。
