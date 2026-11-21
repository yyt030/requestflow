package engine

import (
	"bytes"
	"net"
	"testing"
	"time"

	"requestflow/model"
)

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
