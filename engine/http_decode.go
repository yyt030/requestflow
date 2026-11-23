package engine

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"io"
	"mime"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/andybalholm/brotli"
	"golang.org/x/net/html/charset"
)

// decodeResponseBody 把原始 HTTP 响应体还原为可读的 UTF-8 文本。
// 依次处理：1) Content-Encoding 解压（gzip / deflate / br）；
// 2) 按 Content-Type 的 charset 或 HTML meta 声明转码为 UTF-8。
// 任一步失败都安全降级为原始字节，不中断请求流程。
func decodeResponseBody(data []byte, header http.Header) []byte {
	out := decodeContentEncoding(data, header.Get("Content-Encoding"))
	return decodeCharset(out, header.Get("Content-Type"))
}

// decodeContentEncoding 按 Content-Encoding 解压。Go 的 http.Transport
// 只在请求未显式携带 Accept-Encoding 时自动解压 gzip；一旦用户从浏览器
// 粘贴了 Accept-Encoding（含 gzip/br），传输层不再自动解压，故这里统一处理。
func decodeContentEncoding(data []byte, enc string) []byte {
	for _, token := range strings.Split(strings.ToLower(strings.TrimSpace(enc)), ",") {
		switch strings.TrimSpace(token) {
		case "gzip", "x-gzip":
			r, err := gzip.NewReader(bytes.NewReader(data))
			if err != nil {
				continue
			}
			if b, err := io.ReadAll(r); err == nil {
				data = b
			}
			r.Close()
		case "deflate":
			// deflate 可能是 zlib 包装或裸 flate，先试 zlib 再试裸流。
			if b, err := readZlib(data); err == nil {
				data = b
			} else {
				r := flate.NewReader(bytes.NewReader(data))
				if b, err := io.ReadAll(r); err == nil {
					data = b
				}
				r.Close()
			}
		case "br":
			if b, err := io.ReadAll(brotli.NewReader(bytes.NewReader(data))); err == nil {
				data = b
			}
		}
	}
	return data
}

// readZlib 解压 zlib 包装的 deflate 数据。
func readZlib(data []byte) ([]byte, error) {
	zr, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

// decodeCharset 把非 UTF-8 编码的响应体转成 UTF-8。
// 先用 Content-Type 的 charset 参数；HTML 再尝试 <meta charset>。
// 已是合法 UTF-8 或无解码器时原样返回。
func decodeCharset(data []byte, contentType string) []byte {
	if utf8.Valid(data) {
		return data
	}
	name := charsetFromContentType(contentType)
	if name == "" {
		name = charsetFromHTMLMeta(data)
	}
	if name == "" || strings.EqualFold(name, "utf-8") || strings.EqualFold(name, "utf8") {
		return data
	}
	reader, err := charset.NewReaderLabel(name, bytes.NewReader(data))
	if err != nil {
		return data
	}
	out, err := io.ReadAll(reader)
	if err != nil {
		return data
	}
	return out
}

// charsetFromContentType 从 Content-Type 中提取 charset 参数。
func charsetFromContentType(contentType string) string {
	if contentType == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	return params["charset"]
}

// charsetFromHTMLMeta 从 HTML 的 <meta charset="..."> 声明中提取编码（仅看前 4KB）。
func charsetFromHTMLMeta(data []byte) string {
	head := strings.ToLower(string(data[:minInt(len(data), 4096)]))
	if idx := strings.Index(head, "charset"); idx >= 0 {
		tail := strings.TrimLeft(head[idx+len("charset"):], " \t=\"':;/")
		if end := strings.IndexAny(tail, "\"' ;/>"); end > 0 {
			return strings.TrimSpace(tail[:end])
		}
	}
	return ""
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
