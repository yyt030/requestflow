package ui

import (
	"fmt"
	"strings"
)

// hexDumpLimit Hex 视图最多转储的字节数，避免大响应体放大成数十倍文本拖慢 UI。
const hexDumpLimit = 64 * 1024

// hexDump 把字节序列渲染为经典 hexdump：8 位偏移 + 16 字节十六进制（8 字节分组）+ ASCII 可打印区。
// 不可打印字节在 ASCII 区显示为 '.'。超出上限时截断并注明。
func hexDump(data []byte) string {
	if len(data) == 0 {
		return "（空响应体，无字节可展示）"
	}
	total := len(data)
	truncated := total > hexDumpLimit
	if truncated {
		data = data[:hexDumpLimit]
	}
	const perLine = 16
	var b strings.Builder
	for off := 0; off < len(data); off += perLine {
		end := off + perLine
		if end > len(data) {
			end = len(data)
		}
		fmt.Fprintf(&b, "%08x  ", off)
		for i := 0; i < perLine; i++ {
			if i == 8 {
				b.WriteByte(' ')
			}
			if off+i < end {
				fmt.Fprintf(&b, "%02x ", data[off+i])
			} else {
				b.WriteString("   ")
			}
		}
		b.WriteByte(' ')
		b.WriteByte('|')
		for i := off; i < end; i++ {
			c := data[i]
			if c >= 0x20 && c <= 0x7e {
				b.WriteByte(c)
			} else {
				b.WriteByte('.')
			}
		}
		b.WriteByte('|')
		if end < len(data) {
			b.WriteByte('\n')
		}
	}
	if truncated {
		fmt.Fprintf(&b, "\n\n… 仅展示前 %d 字节（共 %d 字节），Hex 视图上限 64KB。", hexDumpLimit, total)
	}
	return b.String()
}
