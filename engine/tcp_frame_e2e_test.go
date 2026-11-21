package engine

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"requestflow/model"
)

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
