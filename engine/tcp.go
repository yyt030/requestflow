package engine

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"requestflow/model"
)

const (
	maxBin1Payload = 255   // 1 字节长度头可表示的最大值（2^8 - 1）
	maxBin2Payload = 65535 // 2 字节长度头可表示的最大值（2^16 - 1），即 64K-1 字节
)

// frameSpec 描述 TCP 帧头协议，供发送组包与读取拆包共用。
//   - kind: ""（不启用） | "ascii"（N 位十进制左补零，位数自定义） | "bin1"（1 字节） | "bin2"（2 字节）
//   - digits: ascii 的长度头位数（默认 8，可设 3/5 等）
//   - bigEnd: bin2 的字节序，true=大端/网络序（默认），false=小端

// frameSpec 描述 TCP 帧头协议，供发送组包与读取拆包共用。
//   - kind: ""（不启用） | "ascii"（N 位十进制左补零，位数自定义） | "bin1"（1 字节） | "bin2"（2 字节）
//   - digits: ascii 的长度头位数（默认 8，可设 3/5 等）
//   - bigEnd: bin2 的字节序，true=大端/网络序（默认），false=小端
type frameSpec struct {
	kind   string
	digits int
	bigEnd bool
}

func (f frameSpec) enabled() bool { return f.kind == "ascii" || f.kind == "bin1" || f.kind == "bin2" }

// headSize 返回帧头占用的字节数；未启用返回 0。

// headSize 返回帧头占用的字节数；未启用返回 0。
func (f frameSpec) headSize() int {
	switch f.kind {
	case "ascii":
		if f.digits <= 0 {
			return 8
		}
		return f.digits
	case "bin1":
		return 1
	case "bin2":
		return 2
	}
	return 0
}

// parseFrameSpec 从步骤解析帧头规格。
// 兼容旧场景：未配置 tcpFrameType 但配置了 tcpFrameHead 位数时，按 ascii 处理。

// parseFrameSpec 从步骤解析帧头规格。
// 兼容旧场景：未配置 tcpFrameType 但配置了 tcpFrameHead 位数时，按 ascii 处理。
func (e *Engine) parseFrameSpec(step model.Step) frameSpec {
	typ := strings.ToLower(strings.TrimSpace(e.ctx.Render(step.TCPFrameType)))
	digits, _ := strconv.Atoi(strings.TrimSpace(e.ctx.Render(step.TCPFrameHead)))

	switch typ {
	case "ascii", "ascii8", "string", "string8":
		if digits <= 0 {
			digits = 8
		}
		return frameSpec{kind: "ascii", digits: digits}
	case "bin1", "binary1", "byte":
		return frameSpec{kind: "bin1"}
	case "bin2", "binary2", "binary":
		endian := strings.ToLower(strings.TrimSpace(e.ctx.Render(step.TCPFrameEndian)))
		return frameSpec{kind: "bin2", bigEnd: endian != "little"}
	default:
		// 旧场景：仅配置位数即视为 ascii 头
		if digits > 0 {
			return frameSpec{kind: "ascii", digits: digits}
		}
		return frameSpec{}
	}
}

// runTCPStep 执行一个 TCP Socket 步骤。
// 连接模式：
//   - short（默认）：每步新建连接，结束即关闭，兼容旧场景。
//   - long：按 TCPConnKey 跨步骤共享同一 socket，场景结束统一关闭。
//
// TCPClose=true 时关闭指定连接而不发送数据。
//
// 帧协议（send/read 的 frame 模式，帧头类型界面可选）：
//   - ascii（默认）：N 位十进制字符串长度头，位数自定义（3/5/8 等），左补零，如 8 位头 + 8 字节报文 → "0000000888888888"。
//   - bin1：1 字节二进制长度头，最大 255 字节报文。
//   - bin2：2 字节二进制长度头，最大 65535（64K-1）字节；字节序 big（默认，网络序）/ little。
//
// TCPKeepAlive > 0 时开启底层 keepalive 并设为该秒数探测周期；0/空时显式关闭（覆盖 Go 默认开启行为）。
// 响应的提取/断言逻辑与 HTTP 步骤完全复用，status=200 表示成功。

// runTCPStep 执行一个 TCP Socket 步骤。
// 连接模式：
//   - short（默认）：每步新建连接，结束即关闭，兼容旧场景。
//   - long：按 TCPConnKey 跨步骤共享同一 socket，场景结束统一关闭。
//
// TCPClose=true 时关闭指定连接而不发送数据。
//
// 帧协议（send/read 的 frame 模式，帧头类型界面可选）：
//   - ascii（默认）：N 位十进制字符串长度头，位数自定义（3/5/8 等），左补零，如 8 位头 + 8 字节报文 → "0000000888888888"。
//   - bin1：1 字节二进制长度头，最大 255 字节报文。
//   - bin2：2 字节二进制长度头，最大 65535（64K-1）字节；字节序 big（默认，网络序）/ little。
//
// TCPKeepAlive > 0 时开启底层 keepalive 并设为该秒数探测周期；0/空时显式关闭（覆盖 Go 默认开启行为）。
// 响应的提取/断言逻辑与 HTTP 步骤完全复用，status=200 表示成功。
func (e *Engine) runTCPStep(step model.Step) error {
	if err := e.runStepScript(step.PreScript, step, e.lastResp, "Pre-request"); err != nil {
		return err
	}

	if e.tcpPool == nil {
		e.tcpPool = newTCPConnPool()
	}
	connKey := strings.TrimSpace(e.ctx.Render(step.TCPConnKey))
	if connKey == "" {
		connKey = "_default"
	}
	connMode := strings.ToLower(strings.TrimSpace(e.ctx.Render(step.TCPConnMode)))
	fs := e.parseFrameSpec(step)

	// 关闭指定长连接
	if step.TCPClose {
		if e.tcpPool.remove(connKey) {
			e.emit("log", step.ID, step.Name, fmt.Sprintf("TCP 已关闭长连接 [%s]", connKey))
			return nil
		}
		e.emit("log", step.ID, step.Name, fmt.Sprintf("TCP 长连接 [%s] 不存在，无需关闭", connKey))
		return nil
	}

	host := e.ctx.Render(step.TCPHost)
	port := e.ctx.Render(step.TCPPort)
	addr := net.JoinHostPort(host, port)
	sendMode := strings.ToLower(strings.TrimSpace(e.ctx.Render(step.TCPSendMode)))
	readMode := strings.ToLower(strings.TrimSpace(e.ctx.Render(step.TCPReadMode)))
	timeoutMS, _ := strconv.Atoi(strings.TrimSpace(e.ctx.Render(step.TCPTimeout)))
	if timeoutMS <= 0 {
		timeoutMS = 5000
	}
	timeout := time.Duration(timeoutMS) * time.Millisecond
	keepAliveSec, _ := strconv.Atoi(strings.TrimSpace(e.ctx.Render(step.TCPKeepAlive)))
	if keepAliveSec < 0 {
		keepAliveSec = 0
	}

	// 取得连接：long 复用池中连接；short 每次新建并在本步结束关闭。
	var conn net.Conn
	var ownsConn bool // 本步骤是否负责关闭该连接
	if connMode == "long" {
		if entry := e.tcpPool.get(connKey); entry != nil {
			conn = entry.conn
			e.emit("log", step.ID, step.Name, fmt.Sprintf("TCP 复用长连接 [%s] → %s", connKey, addr))
		}
	}
	if conn == nil {
		var err error
		conn, err = dialTCP(host, addr, timeout, step.TCPTLS)
		if err != nil {
			return fmt.Errorf("TCP 连接 %s 失败: %w", addr, err)
		}
		// 应用底层 keepalive：>0 开启并按秒设置探测周期；<=0 显式关闭。
		applyTCPKeepAlive(conn, keepAliveSec)
		ownsConn = true
		if connMode == "long" {
			e.tcpPool.put(connKey, &tcpConnEntry{conn: conn, timeout: timeout})
			e.emit("log", step.ID, step.Name, fmt.Sprintf("TCP 建立长连接 [%s] → %s", connKey, addr))
		} else {
			e.emit("log", step.ID, step.Name, fmt.Sprintf("TCP 连接 %s (超时 %dms)", addr, timeoutMS))
		}
	}
	if ownsConn && connMode != "long" {
		defer conn.Close()
	}
	// 每次收发都刷新超时，长连接空闲一段时间后仍可用。
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return fmt.Errorf("TCP 设置超时失败: %w", err)
	}

	// 构造并发送数据：frame 模式自动加长度头（按帧头类型 ascii/bin2）
	payload, err := buildTCPPayload(e.ctx.Render(step.TCPSend), sendMode, fs)
	if err != nil {
		return err
	}
	if len(payload) > 0 {
		if _, err := conn.Write(payload); err != nil {
			if connMode == "long" {
				// 复用连接写失败，移除坏连接避免后续步骤继续使用
				e.tcpPool.remove(connKey)
			}
			return fmt.Errorf("TCP 发送数据失败: %w", err)
		}
		e.emit("log", step.ID, step.Name, fmt.Sprintf("TCP 已发送 %d 字节", len(payload)))
	}

	// 读取响应：frame 模式先读头再读定长 body
	readNum, _ := strconv.Atoi(strings.TrimSpace(e.ctx.Render(step.TCPReadNum)))
	data, err := readTCP(conn, readMode, readNum, unescapeTCP(e.ctx.Render(step.TCPUntil)), fs)
	if err != nil {
		if connMode == "long" {
			e.tcpPool.remove(connKey)
		}
		return fmt.Errorf("TCP 读取响应失败: %w", err)
	}
	e.emit("log", step.ID, step.Name, fmt.Sprintf("TCP 收到 %d 字节", len(data)))

	resp := &model.Response{
		StatusCode: 200,
		Body:       data,
		Text:       string(data),
		Headers:    map[string][]string{},
	}
	if jsonv := parseJSONSafe(string(data)); jsonv != nil {
		resp.JSON = jsonv
	}
	e.lastResp = resp

	if err := applyExtracts(e.ctx, resp, step.Extract); err != nil {
		return err
	}
	details, detErr := applyAssertsDetailed(e.ctx, resp, step.Assert)
	e.trace.addAssertions(step.Name, step.ID, details)
	if detErr != nil {
		return detErr
	}
	e.ctx.SetStepResponse(step.Name, responseView(resp))
	if err := e.runStepScript(step.PostScript, step, resp, "Tests"); err != nil {
		return err
	}
	return nil
}

// dialTCP 建立一条 TCP（或 TLS）连接。

// dialTCP 建立一条 TCP（或 TLS）连接。
func dialTCP(host, addr string, timeout time.Duration, useTLS bool) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: timeout}
	if useTLS {
		return tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: host, InsecureSkipVerify: true})
	}
	return dialer.Dial("tcp", addr)
}

// applyTCPKeepAlive 对底层 TCP socket 配置 keepalive。
// seconds<=0 时显式关闭（Go 默认开启 keepalive，需覆盖）；>0 时开启并按秒设置探测周期。
// TLS 连接通过 NetConn() 取回底层 TCPConn。设置失败仅记录、不阻断连接。

// applyTCPKeepAlive 对底层 TCP socket 配置 keepalive。
// seconds<=0 时显式关闭（Go 默认开启 keepalive，需覆盖）；>0 时开启并按秒设置探测周期。
// TLS 连接通过 NetConn() 取回底层 TCPConn。设置失败仅记录、不阻断连接。
func applyTCPKeepAlive(conn net.Conn, seconds int) {
	var tc *net.TCPConn
	switch c := conn.(type) {
	case *net.TCPConn:
		tc = c
	case *tls.Conn:
		if raw, ok := c.NetConn().(*net.TCPConn); ok {
			tc = raw
		}
	}
	if tc == nil {
		return
	}
	if seconds <= 0 {
		_ = tc.SetKeepAlive(false)
		return
	}
	if err := tc.SetKeepAlive(true); err == nil {
		_ = tc.SetKeepAlivePeriod(time.Duration(seconds) * time.Second)
	}
}

// parseJSONSafe 尝试把文本解析为 JSON，失败返回 nil（不影响响应文本本身）。

// parseJSONSafe 尝试把文本解析为 JSON，失败返回 nil（不影响响应文本本身）。
func parseJSONSafe(s string) interface{} {
	if !json.Valid([]byte(s)) {
		return nil
	}
	var v interface{}
	if json.Unmarshal([]byte(s), &v) != nil {
		return nil
	}
	return v
}

// buildTCPPayload 根据发送模式把文本转换为字节。
//   - hex：解析转义后按十六进制解码
//   - raw：原样 UTF-8（含转义解析）
//   - frame：原始内容 + 帧头（ascii 十进制左补零 / bin1 单字节 / bin2 两字节）
//   - line（默认）：末尾追加 \n
