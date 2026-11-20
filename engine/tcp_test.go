package engine

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
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
func TestFrameLengthHead(t *testing.T) {
	cases := []struct {
		length int
		digits int
		want   string
	}{
		{8, 8, "00000008"},
		{123, 8, "00000123"},
		{0, 8, "00000000"},
		{8, 4, "0008"},
		{12345, 4, "2345"}, // 超长保留低位，保持帧边界
	}
	for _, c := range cases {
		got := string(frameLengthHead(c.length, c.digits))
		if got != c.want {
			t.Errorf("frameLengthHead(%d,%d) = %q, 期望 %q", c.length, c.digits, got, c.want)
		}
	}
}

// TestBuildTCPPayloadFrame 验证 frame 发送组包：ascii 长度头 + 报文体。
func TestBuildTCPPayloadFrame(t *testing.T) {
	got, err := buildTCPPayload("88888888", "frame", frameSpec{kind: "ascii", digits: 8})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "0000000888888888" {
		t.Fatalf("ascii frame 组包 = %q, 期望 %q", string(got), "0000000888888888")
	}
	// 未选择帧头类型应报错
	if _, err := buildTCPPayload("abc", "frame", frameSpec{}); err == nil {
		t.Fatal("frame 模式未选择帧头类型应报错")
	}
}

// TestTCPLongConnectionReuse 验证长连接跨步骤复用：同一连接只 accept 一次，两步骤都成功。
func TestTCPLongConnectionReuse(t *testing.T) {
	acceptCount := 0
	host, port, closeFn := startTCPServer(t, func(conn net.Conn) {
		acceptCount++
		defer conn.Close()
		r := bufio.NewReader(conn)
		for {
			line, err := r.ReadString('\n')
			if len(line) > 0 {
				_, _ = conn.Write([]byte("ACK:" + line))
			}
			if err != nil {
				return
			}
		}
	})
	defer closeFn()

	step := func(name, send, varName string) model.Step {
		return model.Step{
			ID: name, Name: name, Type: model.StepTCP, Enabled: true,
			TCPHost: host, TCPPort: port,
			TCPConnMode: "long", TCPConnKey: "sess1",
			TCPSend: send, TCPSendMode: "line", TCPReadMode: "line", TCPTimeout: "3000",
			Extract: []model.Extract{{Name: varName, Source: "body", Kind: "whole"}},
		}
	}
	sc := &model.Scenario{Name: "long-conn", Steps: []model.Step{
		step("第一步", "first", "r1"),
		step("第二步", "second", "r2"),
	}}
	res := NewEngine(sc, nil).Run()
	if !res.OK {
		t.Fatalf("长连接复用应成功，失败: %v", res.Err)
	}
	if acceptCount != 1 {
		t.Fatalf("长连接应只建立 1 次连接，实际 accept %d 次", acceptCount)
	}
	if !strings.Contains(fmt.Sprint(res.Vars["r1"]), "ACK:first") || !strings.Contains(fmt.Sprint(res.Vars["r2"]), "ACK:second") {
		t.Fatalf("两步响应异常: r1=%v r2=%v", res.Vars["r1"], res.Vars["r2"])
	}
}

// TestTCPFrameEcho 端到端验证帧协议：客户端发 0000000888888888，服务端按帧解析并回帧。
func TestTCPFrameEcho(t *testing.T) {
	host, port, closeFn := startTCPServer(t, func(conn net.Conn) {
		defer conn.Close()
		head := make([]byte, 8)
		if _, err := io.ReadFull(conn, head); err != nil {
			return
		}
		n, err := strconv.Atoi(string(head))
		if err != nil {
			return
		}
		body := make([]byte, n)
		if _, err := io.ReadFull(conn, body); err != nil {
			return
		}
		// 回显：把收到的 body 用同样的帧头发回
		respBody := append([]byte("ECHO:"), body...)
		_, _ = conn.Write(frameLengthHead(len(respBody), 8))
		_, _ = conn.Write(respBody)
	})
	defer closeFn()

	step := model.Step{
		ID: "frame", Name: "帧协议", Type: model.StepTCP, Enabled: true,
		TCPHost: host, TCPPort: port,
		TCPSend: "88888888", TCPSendMode: "frame", TCPReadMode: "frame", TCPFrameType: "ascii", TCPFrameHead: "8", TCPTimeout: "3000",
		Extract: []model.Extract{{Name: "frameResp", Source: "body", Kind: "whole"}},
		Assert:  []model.Assert{{Source: "body", Op: "contains", Value: "ECHO:88888888"}},
	}
	res := NewEngine(&model.Scenario{Name: "s", Steps: []model.Step{step}}, nil).Run()
	if !res.OK {
		t.Fatalf("帧协议应成功，失败: %v", res.Err)
	}
	if fmt.Sprint(res.Vars["frameResp"]) != "ECHO:88888888" {
		t.Fatalf("帧响应异常: %v", res.Vars["frameResp"])
	}
}

// TestEncodeFrameHeader 覆盖 ASCII 自定义位数与 1/2 字节二进制帧头编码及字节序。
func TestEncodeFrameHeader(t *testing.T) {
	cases := []struct {
		name   string
		length int
		fs     frameSpec
		want   []byte
	}{
		{"ascii8", 8, frameSpec{kind: "ascii", digits: 8}, []byte("00000008")},
		{"ascii3", 88, frameSpec{kind: "ascii", digits: 3}, []byte("088")},
		{"ascii5", 8, frameSpec{kind: "ascii", digits: 5}, []byte("00008")},
		{"bin1-8", 8, frameSpec{kind: "bin1"}, []byte{0x08}},
		{"bin1-max", 255, frameSpec{kind: "bin1"}, []byte{0xff}},
		{"bin2-big-8", 8, frameSpec{kind: "bin2", bigEnd: true}, []byte{0x00, 0x08}},
		{"bin2-big-max", 65535, frameSpec{kind: "bin2", bigEnd: true}, []byte{0xff, 0xff}},
		{"bin2-little-8", 8, frameSpec{kind: "bin2", bigEnd: false}, []byte{0x08, 0x00}},
		{"bin2-little-258", 258, frameSpec{kind: "bin2", bigEnd: false}, []byte{0x02, 0x01}},
	}
	for _, c := range cases {
		got, err := encodeFrameHeader(c.length, c.fs)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !bytes.Equal(got, c.want) {
			t.Errorf("%s: got % x, want % x", c.name, got, c.want)
		}
	}
	// 越界应报错，而不是静默截断破坏帧边界
	if _, err := encodeFrameHeader(256, frameSpec{kind: "bin1"}); err == nil {
		t.Fatal("bin1 报文超过 255 字节应报错")
	}
	if _, err := encodeFrameHeader(65536, frameSpec{kind: "bin2", bigEnd: true}); err == nil {
		t.Fatal("bin2 报文超过 65535 字节应报错")
	}
}

// TestDecodeFrameHeader 覆盖两种帧头解码及非法输入。
func TestDecodeFrameHeader(t *testing.T) {
	cases := []struct {
		name string
		head []byte
		fs   frameSpec
		want int
	}{
		{"ascii8", []byte("00000123"), frameSpec{kind: "ascii", digits: 8}, 123},
		{"ascii3", []byte("088"), frameSpec{kind: "ascii", digits: 3}, 88},
		{"bin1-8", []byte{0x08}, frameSpec{kind: "bin1"}, 8},
		{"bin1-max", []byte{0xff}, frameSpec{kind: "bin1"}, 255},
		{"bin2-big", []byte{0x01, 0x02}, frameSpec{kind: "bin2", bigEnd: true}, 258},
		{"bin2-little", []byte{0x02, 0x01}, frameSpec{kind: "bin2", bigEnd: false}, 258},
	}
	for _, c := range cases {
		got, err := decodeFrameHeader(c.head, c.fs)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
	if _, err := decodeFrameHeader([]byte("abcdefgh"), frameSpec{kind: "ascii", digits: 8}); err == nil {
		t.Fatal("非法 ASCII 帧头应报错")
	}
	if _, err := decodeFrameHeader([]byte{0x01}, frameSpec{kind: "bin2", bigEnd: true}); err == nil {
		t.Fatal("bin2 帧头不足 2 字节应报错")
	}
	if _, err := decodeFrameHeader([]byte{}, frameSpec{kind: "bin1"}); err == nil {
		t.Fatal("bin1 帧头不足 1 字节应报错")
	}
}

// TestParseFrameSpec 验证帧头类型解析，以及旧场景仅配置位数时的兼容推断。
func TestParseFrameSpec(t *testing.T) {
	e := NewEngine(&model.Scenario{Name: "s"}, nil)
	cases := []struct {
		name string
		step model.Step
		want frameSpec
	}{
		{"默认不启用", model.Step{}, frameSpec{}},
		{"ascii显式", model.Step{TCPFrameType: "ascii"}, frameSpec{kind: "ascii", digits: 8}},
		{"ascii自定义位数", model.Step{TCPFrameType: "ascii", TCPFrameHead: "4"}, frameSpec{kind: "ascii", digits: 4}},
		{"ascii3位", model.Step{TCPFrameType: "ascii", TCPFrameHead: "3"}, frameSpec{kind: "ascii", digits: 3}},
		{"旧场景按位数推断", model.Step{TCPFrameHead: "8"}, frameSpec{kind: "ascii", digits: 8}},
		{"bin1", model.Step{TCPFrameType: "bin1"}, frameSpec{kind: "bin1"}},
		{"bin2默认大端", model.Step{TCPFrameType: "bin2"}, frameSpec{kind: "bin2", bigEnd: true}},
		{"bin2小端", model.Step{TCPFrameType: "bin2", TCPFrameEndian: "little"}, frameSpec{kind: "bin2", bigEnd: false}},
	}
	for _, c := range cases {
		got := e.parseFrameSpec(c.step)
		if got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

// TestTCPBin2FrameEcho 端到端验证 2 字节二进制帧头（大端）收发。
func TestTCPBin2FrameEcho(t *testing.T) {
	host, port, closeFn := startTCPServer(t, func(conn net.Conn) {
		defer conn.Close()
		head := make([]byte, 2)
		if _, err := io.ReadFull(conn, head); err != nil {
			return
		}
		n := int(binary.BigEndian.Uint16(head))
		body := make([]byte, n)
		if _, err := io.ReadFull(conn, body); err != nil {
			return
		}
		respBody := append([]byte("BIN:"), body...)
		respHead := make([]byte, 2)
		binary.BigEndian.PutUint16(respHead, uint16(len(respBody)))
		_, _ = conn.Write(respHead)
		_, _ = conn.Write(respBody)
	})
	defer closeFn()

	step := model.Step{
		ID: "bin2", Name: "二进制帧头", Type: model.StepTCP, Enabled: true,
		TCPHost: host, TCPPort: port,
		TCPSend: "88888888", TCPSendMode: "frame", TCPReadMode: "frame",
		TCPFrameType: "bin2", TCPFrameEndian: "big", TCPTimeout: "3000",
		Extract: []model.Extract{{Name: "binResp", Source: "body", Kind: "whole"}},
		Assert:  []model.Assert{{Source: "body", Op: "contains", Value: "BIN:88888888"}},
	}
	res := NewEngine(&model.Scenario{Name: "s", Steps: []model.Step{step}}, nil).Run()
	if !res.OK {
		t.Fatalf("bin2 帧协议应成功，失败: %v", res.Err)
	}
	if fmt.Sprint(res.Vars["binResp"]) != "BIN:88888888" {
		t.Fatalf("bin2 帧响应异常: %v", res.Vars["binResp"])
	}
}

// TestTCPBin2FrameLongConnection 端到端验证「长连接 + bin2 帧头」跨步骤复用：只建一次连接，两步均收发成功。
func TestTCPBin2FrameLongConnection(t *testing.T) {
	var acceptCount int32
	host, port, closeFn := startTCPServer(t, func(conn net.Conn) {
		atomic.AddInt32(&acceptCount, 1)
		defer conn.Close()
		for {
			head := make([]byte, 2)
			if _, err := io.ReadFull(conn, head); err != nil {
				return
			}
			n := int(binary.BigEndian.Uint16(head))
			body := make([]byte, n)
			if _, err := io.ReadFull(conn, body); err != nil {
				return
			}
			respBody := append([]byte("OK:"), body...)
			respHead := make([]byte, 2)
			binary.BigEndian.PutUint16(respHead, uint16(len(respBody)))
			if _, err := conn.Write(respHead); err != nil {
				return
			}
			if _, err := conn.Write(respBody); err != nil {
				return
			}
		}
	})
	defer closeFn()

	makeStep := func(name, send, varName string) model.Step {
		return model.Step{
			ID: name, Name: name, Type: model.StepTCP, Enabled: true,
			TCPHost: host, TCPPort: port,
			TCPConnMode: "long", TCPConnKey: "bin-sess",
			TCPSend: send, TCPSendMode: "frame", TCPReadMode: "frame",
			TCPFrameType: "bin2", TCPFrameEndian: "big", TCPTimeout: "3000",
			Extract: []model.Extract{{Name: varName, Source: "body", Kind: "whole"}},
		}
	}
	sc := &model.Scenario{Name: "bin2-long", Steps: []model.Step{
		makeStep("登录帧", "LOGIN zhangsan", "r1"),
		makeStep("查询帧", "QUERY.profile", "r2"),
	}}
	res := NewEngine(sc, nil).Run()
	if !res.OK {
		t.Fatalf("bin2 长连接帧协议应成功，失败: %v", res.Err)
	}
	if got := atomic.LoadInt32(&acceptCount); got != 1 {
		t.Fatalf("长连接应只建立 1 次连接，实际 accept %d 次", got)
	}
	if fmt.Sprint(res.Vars["r1"]) != "OK:LOGIN zhangsan" || fmt.Sprint(res.Vars["r2"]) != "OK:QUERY.profile" {
		t.Fatalf("两步响应异常: r1=%v r2=%v", res.Vars["r1"], res.Vars["r2"])
	}
}

// TestTCPFrameNoType 验证 frame 模式未选帧头类型时报清晰错误，不产生半截报文。
func TestTCPFrameNoType(t *testing.T) {
	if _, err := buildTCPPayload("abc", "frame", frameSpec{}); err == nil {
		t.Fatal("frame 发送未选帧头类型应报错")
	}
	// 旧场景仅配置位数仍可工作（兼容）
	got, err := buildTCPPayload("abc", "frame", frameSpec{kind: "ascii", digits: 8})
	if err != nil || string(got) != "00000003abc" {
		t.Fatalf("ascii frame 兼容路径异常: got=%q err=%v", string(got), err)
	}
}

// TestTCPBin1FrameEcho 端到端验证 1 字节二进制帧头收发。
func TestTCPBin1FrameEcho(t *testing.T) {
	host, port, closeFn := startTCPServer(t, func(conn net.Conn) {
		defer conn.Close()
		head := make([]byte, 1)
		if _, err := io.ReadFull(conn, head); err != nil {
			return
		}
		n := int(head[0])
		body := make([]byte, n)
		if _, err := io.ReadFull(conn, body); err != nil {
			return
		}
		respBody := append([]byte("B1:"), body...)
		if _, err := conn.Write([]byte{byte(len(respBody))}); err != nil {
			return
		}
		_, _ = conn.Write(respBody)
	})
	defer closeFn()

	step := model.Step{
		ID: "bin1", Name: "1字节帧", Type: model.StepTCP, Enabled: true,
		TCPHost: host, TCPPort: port,
		TCPSend: "HELLO", TCPSendMode: "frame", TCPReadMode: "frame",
		TCPFrameType: "bin1", TCPTimeout: "3000",
		Extract: []model.Extract{{Name: "bin1Resp", Source: "body", Kind: "whole"}},
		Assert:  []model.Assert{{Source: "body", Op: "contains", Value: "B1:HELLO"}},
	}
	res := NewEngine(&model.Scenario{Name: "s", Steps: []model.Step{step}}, nil).Run()
	if !res.OK {
		t.Fatalf("bin1 帧协议应成功，失败: %v", res.Err)
	}
	if fmt.Sprint(res.Vars["bin1Resp"]) != "B1:HELLO" {
		t.Fatalf("bin1 帧响应异常: %v", res.Vars["bin1Resp"])
	}
}

// TestTCPAsciiCustomDigitsEcho 端到端验证自定义位数（3 位）字符串帧头收发。
func TestTCPAsciiCustomDigitsEcho(t *testing.T) {
	host, port, closeFn := startTCPServer(t, func(conn net.Conn) {
		defer conn.Close()
		head := make([]byte, 3)
		if _, err := io.ReadFull(conn, head); err != nil {
			return
		}
		n, err := strconv.Atoi(string(head))
		if err != nil {
			return
		}
		body := make([]byte, n)
		if _, err := io.ReadFull(conn, body); err != nil {
			return
		}
		respBody := append([]byte("A3:"), body...)
		_, _ = conn.Write(frameLengthHead(len(respBody), 3))
		_, _ = conn.Write(respBody)
	})
	defer closeFn()

	step := model.Step{
		ID: "ascii3", Name: "3位字符串帧", Type: model.StepTCP, Enabled: true,
		TCPHost: host, TCPPort: port,
		TCPSend: "PING", TCPSendMode: "frame", TCPReadMode: "frame",
		TCPFrameType: "ascii", TCPFrameHead: "3", TCPTimeout: "3000",
		Extract: []model.Extract{{Name: "a3Resp", Source: "body", Kind: "whole"}},
		Assert:  []model.Assert{{Source: "body", Op: "contains", Value: "A3:PING"}},
	}
	res := NewEngine(&model.Scenario{Name: "s", Steps: []model.Step{step}}, nil).Run()
	if !res.OK {
		t.Fatalf("3 位 ascii 帧协议应成功，失败: %v", res.Err)
	}
	if fmt.Sprint(res.Vars["a3Resp"]) != "A3:PING" {
		t.Fatalf("3 位 ascii 帧响应异常: %v", res.Vars["a3Resp"])
	}
}

// TestApplyTCPKeepAlive 验证 keepalive 配置对普通 TCP 连接不报错：
// seconds=0 关闭、>0 开启并按秒设置周期，均不影响连接可用性。
func TestApplyTCPKeepAlive(t *testing.T) {
	host, port, closeFn := startTCPServer(t, echoLineServer)
	defer closeFn()

	for _, sec := range []int{0, 1, 30} {
		conn, err := dialTCP(host, net.JoinHostPort(host, port), 2*time.Second, false)
		if err != nil {
			t.Fatalf("dial failed: %v", err)
		}
		applyTCPKeepAlive(conn, sec)
		_ = conn.Close()
	}
}
