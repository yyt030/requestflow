package engine

import (
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
	"golang.org/x/text/encoding/simplifiedchinese"

	"requestflow/model"
)

func TestDecodeContentEncoding_Gzip(t *testing.T) {
	raw := []byte(`{"name":"肖申克的救赎","rating":9.7}`)
	var buf strings.Builder
	zw := gzip.NewWriter(&buf)
	zw.Write(raw)
	zw.Close()
	got := decodeContentEncoding([]byte(buf.String()), "gzip")
	if string(got) != string(raw) {
		t.Fatalf("gzip decode = %q, want %q", got, raw)
	}
}

func TestDecodeContentEncoding_Br(t *testing.T) {
	raw := []byte("hello brotli 你好")
	var buf strings.Builder
	bw := brotli.NewWriter(&buf)
	bw.Write(raw)
	bw.Close()
	got := decodeContentEncoding([]byte(buf.String()), "br")
	if string(got) != string(raw) {
		t.Fatalf("br decode = %q, want %q", got, raw)
	}
}

func TestDecodeCharset_GBK(t *testing.T) {
	gbk, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("中文标题"))
	if err != nil {
		t.Fatal(err)
	}
	got := decodeCharset(gbk, "text/html; charset=gbk")
	if string(got) != "中文标题" {
		t.Fatalf("gbk decode = %q, want 中文标题", got)
	}
}

func TestDecodeCharset_UTF8Unchanged(t *testing.T) {
	raw := []byte("已经是 utf-8 中文")
	if got := decodeCharset(raw, "text/plain; charset=utf-8"); string(got) != string(raw) {
		t.Fatalf("utf8 should be unchanged, got %q", got)
	}
}

// 端到端：客户端显式声明 Accept-Encoding: gzip，服务端返回 gzip 压缩的中文，
// 验证调试结果 Body 为可读 UTF-8（Go 传输层此时不会自动解压）。
func TestDebug_GzipWithExplicitAcceptEncoding(t *testing.T) {
	raw := "中文响应体 ok"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Encoding", "gzip")
		zw := gzip.NewWriter(w)
		zw.Write([]byte(raw))
		zw.Close()
	}))
	defer srv.Close()

	res := Debug(DebugRequest{
		Method:  "GET",
		URL:     srv.URL,
		Headers: map[string]string{"Accept-Encoding": "gzip"},
	}, nil)
	if !res.OK {
		t.Fatalf("debug not OK: %s", res.Err)
	}
	if !strings.Contains(res.Body, "中文响应体") {
		t.Fatalf("gzip body not decoded to UTF-8: %q", res.Body)
	}
}

// 端到端：默认请求（不显式设 Accept-Encoding），服务端返回 gzip。验证 Transport
// 关闭自动解压后，展示头保留 Content-Encoding: gzip，且 body 由 decodeResponseBody
// 解压为可读中文（回归：此前 Go 自动解压会抹掉该头）。
func TestDebug_ContentEncodingVisibleWithGzipResponse(t *testing.T) {
	raw := "自动解压的中文响应"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Encoding", "gzip")
		zw := gzip.NewWriter(w)
		zw.Write([]byte(raw))
		zw.Close()
	}))
	defer srv.Close()

	client := NewHTTPClient(2*time.Second, false)
	resp, err := client.DoRequest(&model.Step{}, "GET", srv.URL, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// body 必须已被解压为可读文本
	if resp.Text != raw {
		t.Fatalf("auto-decompressed body = %q, want %q", resp.Text, raw)
	}
	// 展示头必须含 Content-Encoding: gzip
	var found bool
	for _, kv := range resp.Headers["Content-Encoding"] {
		if strings.EqualFold(strings.TrimSpace(kv), "gzip") {
			found = true
		}
	}
	if !found {
		t.Fatalf("Content-Encoding gzip not visible after auto-decompress: %#v", resp.Headers)
	}
}

// 端到端：服务端以 chunked 分块传输，验证 Transfer-Encoding 头展示出来。
func TestDebug_ChunkedHeaderVisible(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("chunk-a"))
		w.(http.Flusher).Flush()
		w.Write([]byte("chunk-b"))
	}))
	defer srv.Close()

	client := NewHTTPClient(2*time.Second, false)
	resp, err := client.DoRequest(&model.Step{}, "GET", srv.URL, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, kv := range resp.Headers["Transfer-Encoding"] {
		if strings.Contains(strings.ToLower(kv), "chunked") {
			found = true
		}
	}
	if !found {
		t.Fatalf("Transfer-Encoding chunked not present: %#v", resp.Headers)
	}
}
