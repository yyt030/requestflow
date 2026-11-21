package engine

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
)

// buildTCPPayload 根据发送模式把文本转换为字节。
//   - hex：解析转义后按十六进制解码
//   - raw：原样 UTF-8（含转义解析）
//   - frame：原始内容 + 帧头（ascii 十进制左补零 / bin1 单字节 / bin2 两字节）
//   - line（默认）：末尾追加 \n
func buildTCPPayload(text, mode string, fs frameSpec) ([]byte, error) {
	switch mode {
	case "hex":
		clean := strings.NewReplacer(" ", "", "\n", "", "\r", "", "\t", "").Replace(unescapeTCP(text))
		b, err := hex.DecodeString(clean)
		if err != nil {
			return nil, fmt.Errorf("TCP 十六进制数据无效: %w", err)
		}
		return b, nil
	case "raw":
		return []byte(unescapeTCP(text)), nil
	case "frame":
		body := []byte(unescapeTCP(text))
		head, err := encodeFrameHeader(len(body), fs)
		if err != nil {
			return nil, err
		}
		return append(head, body...), nil
	default: // line
		return append([]byte(unescapeTCP(text)), '\n'), nil
	}
}

// encodeFrameHeader 按帧头规格编码报文长度。

// encodeFrameHeader 按帧头规格编码报文长度。
func encodeFrameHeader(length int, fs frameSpec) ([]byte, error) {
	if length < 0 {
		length = 0
	}
	switch fs.kind {
	case "ascii":
		return frameLengthHead(length, fs.digits), nil
	case "bin1":
		if length > maxBin1Payload {
			return nil, fmt.Errorf("TCP bin1 帧头仅 1 字节，最大支持 %d 字节报文，当前 %d 字节超出范围", maxBin1Payload, length)
		}
		return []byte{byte(length)}, nil
	case "bin2":
		if length > maxBin2Payload {
			return nil, fmt.Errorf("TCP bin2 帧头最大支持 %d 字节（64K-1），当前报文 %d 字节超出范围", maxBin2Payload, length)
		}
		head := make([]byte, 2)
		if fs.bigEnd {
			binary.BigEndian.PutUint16(head, uint16(length))
		} else {
			binary.LittleEndian.PutUint16(head, uint16(length))
		}
		return head, nil
	default:
		return nil, fmt.Errorf("TCP frame 发送模式需先选择帧头类型（ASCII 字符串头 / 1 字节二进制 / 2 字节二进制）")
	}
}

// decodeFrameHeader 解析已读出的帧头字节，返回报文长度。

// decodeFrameHeader 解析已读出的帧头字节，返回报文长度。
func decodeFrameHeader(head []byte, fs frameSpec) (int, error) {
	switch fs.kind {
	case "ascii":
		length, err := strconv.Atoi(strings.TrimSpace(string(head)))
		if err != nil || length < 0 {
			return 0, fmt.Errorf("TCP ASCII 帧头无效: %q", string(head))
		}
		return length, nil
	case "bin1":
		if len(head) < 1 {
			return 0, fmt.Errorf("TCP bin1 帧头需要 1 字节，实际读到 %d 字节", len(head))
		}
		return int(head[0]), nil
	case "bin2":
		if len(head) < 2 {
			return 0, fmt.Errorf("TCP bin2 帧头需要 2 字节，实际读到 %d 字节", len(head))
		}
		if fs.bigEnd {
			return int(binary.BigEndian.Uint16(head[:2])), nil
		}
		return int(binary.LittleEndian.Uint16(head[:2])), nil
	default:
		return 0, fmt.Errorf("TCP frame 读取模式需先选择帧头类型（ASCII 字符串头 / 1 字节二进制 / 2 字节二进制）")
	}
}

// frameLengthHead 生成 N 位十进制长度头，不足左补零（ascii 帧头专用）。
// 例：frameLengthHead(8, 8) → "00000008"。

// frameLengthHead 生成 N 位十进制长度头，不足左补零（ascii 帧头专用）。
// 例：frameLengthHead(8, 8) → "00000008"。
func frameLengthHead(length, digits int) []byte {
	s := strconv.Itoa(length)
	if len(s) >= digits {
		return []byte(s[len(s)-digits:]) // 超长时保留低位，避免破坏帧边界
	}
	out := make([]byte, digits)
	for i := 0; i < digits-len(s); i++ {
		out[i] = '0'
	}
	copy(out[digits-len(s):], s)
	return out
}

// unescapeTCP 解析常见的转义序列：\n \r \t \0 \\ \xNN。

// unescapeTCP 解析常见的转义序列：\n \r \t \0 \\ \xNN。
func unescapeTCP(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case '0':
			b.WriteByte(0)
		case '\\':
			b.WriteByte('\\')
		case 'x':
			if i+2 < len(s) {
				if v, err := strconv.ParseUint(s[i+1:i+3], 16, 8); err == nil {
					b.WriteByte(byte(v))
					i += 2
					continue
				}
			}
			b.WriteByte('x')
		default:
			b.WriteByte('\\')
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// readTCP 按模式从连接读取数据。
//   - all：读到对端关闭或超时；fixed：读固定字节数；until：读到指定标记；line：读一行
//   - frame：先读帧头解析出长度 L，再精确读 L 字节（ascii/bin1/bin2 三种帧头）

// readTCP 按模式从连接读取数据。
//   - all：读到对端关闭或超时；fixed：读固定字节数；until：读到指定标记；line：读一行
//   - frame：先读帧头解析出长度 L，再精确读 L 字节（ascii/bin1/bin2 三种帧头）
func readTCP(conn net.Conn, mode string, num int, until string, fs frameSpec) ([]byte, error) {
	if mode == "frame" {
		size := fs.headSize()
		if size <= 0 {
			return nil, fmt.Errorf("TCP frame 读取模式需先选择帧头类型（ASCII 字符串头 / 1 字节二进制 / 2 字节二进制）")
		}
		head := make([]byte, size)
		if _, err := io.ReadFull(conn, head); err != nil {
			return nil, err
		}
		length, err := decodeFrameHeader(head, fs)
		if err != nil {
			return nil, err
		}
		body := make([]byte, length)
		if length > 0 {
			if _, err := io.ReadFull(conn, body); err != nil {
				return nil, err
			}
		}
		return body, nil
	}
	switch mode {
	case "all":
		data, err := io.ReadAll(conn)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				return data, nil
			}
			return nil, err
		}
		return data, nil
	case "fixed":
		if num <= 0 {
			num = 1024
		}
		buf := make([]byte, num)
		n, err := io.ReadFull(conn, buf)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return nil, err
		}
		return buf[:n], nil
	case "until":
		if until == "" {
			until = "\n"
		}
		reader := bufio.NewReader(conn)
		data, err := reader.ReadBytes(until[0])
		if err != nil && err != io.EOF {
			if ne, ok := err.(net.Error); ok && ne.Timeout() && len(data) > 0 {
				return data, nil
			}
			return nil, err
		}
		return data, nil
	default: // line
		reader := bufio.NewReader(conn)
		line, err := reader.ReadBytes('\n')
		if err != nil && err != io.EOF {
			if ne, ok := err.(net.Error); ok && ne.Timeout() && len(line) > 0 {
				return line, nil
			}
			return nil, err
		}
		return line, nil
	}
}
