package engine

import (
	"crypto/tls"
	"net/http/httptrace"
	"sync"
	"time"

	"requestflow/model"
)

// timingCollector 用 httptrace.ClientTrace 采集一次 requests 各阶段时间戳。
// 回调可能来自不同 goroutine，用 mu 保护写入；各回调幂等（只记录首次/末次）。
type timingCollector struct {
	mu sync.Mutex

	start time.Time
	// DNS
	dnsStart, dnsEnd time.Time
	// TCP 建连（可能多次：记录第一次开始、最后一次结束）
	connStart, connEnd time.Time
	// TLS 握手
	tlsStart, tlsEnd time.Time
	// 连接就绪（GotConn）与请求写完（WroteRequest）
	gotConn, wroteReq time.Time
	reused            bool
	// 首字节（GotFirstResponseByte）
	firstByte time.Time
	// 响应体读完（DoRequest 中显式记录）
	bodyEnd time.Time
}

// newTimingCollector 返回采集器与注入请求用的 ClientTrace。
func newTimingCollector() (*timingCollector, *httptrace.ClientTrace) {
	c := &timingCollector{start: time.Now()}
	trace := &httptrace.ClientTrace{
		DNSStart: func(httptrace.DNSStartInfo) {
			c.mu.Lock()
			if c.dnsStart.IsZero() {
				c.dnsStart = time.Now()
			}
			c.mu.Unlock()
		},
		DNSDone: func(httptrace.DNSDoneInfo) {
			c.mu.Lock()
			c.dnsEnd = time.Now()
			c.mu.Unlock()
		},
		ConnectStart: func(network, addr string) {
			c.mu.Lock()
			if c.connStart.IsZero() {
				c.connStart = time.Now()
			}
			c.mu.Unlock()
		},
		ConnectDone: func(network, addr string, err error) {
			c.mu.Lock()
			c.connEnd = time.Now()
			c.mu.Unlock()
		},
		TLSHandshakeStart: func() {
			c.mu.Lock()
			c.tlsStart = time.Now()
			c.mu.Unlock()
		},
		TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
			c.mu.Lock()
			c.tlsEnd = time.Now()
			c.mu.Unlock()
		},
		GotConn: func(info httptrace.GotConnInfo) {
			c.mu.Lock()
			if c.gotConn.IsZero() {
				c.gotConn = time.Now()
				c.reused = info.Reused
			}
			c.mu.Unlock()
		},
		WroteRequest: func(httptrace.WroteRequestInfo) {
			c.mu.Lock()
			c.wroteReq = time.Now()
			c.mu.Unlock()
		},
		GotFirstResponseByte: func() {
			c.mu.Lock()
			if c.firstByte.IsZero() {
				c.firstByte = time.Now()
			}
			c.mu.Unlock()
		},
	}
	return c, trace
}

// markBodyEnd 在响应体读完时记录结束时间。
func (c *timingCollector) markBodyEnd() {
	c.mu.Lock()
	c.bodyEnd = time.Now()
	c.mu.Unlock()
}

// result 汇总为 model.HTTPTiming。用时间点差值计算每段，Other 兜底吸收间隙。
func (c *timingCollector) result() *model.HTTPTiming {
	c.mu.Lock()
	defer c.mu.Unlock()

	end := c.bodyEnd
	if end.IsZero() {
		end = time.Now()
	}
	t := &model.HTTPTiming{Available: true, Reused: c.reused}
	t.Total = span(c.start, end)

	t.DNS = span(c.dnsStart, c.dnsEnd)
	t.Connect = span(c.connStart, c.connEnd)
	t.TLS = span(c.tlsStart, c.tlsEnd)

	// 请求发送：连接就绪 → 请求写完。若 GotConn 未触发（异常路径）则从起点算到写完。
	sendFrom := c.gotConn
	if sendFrom.IsZero() {
		sendFrom = c.start
	}
	t.Send = span(sendFrom, c.wroteReq)
	// 等待响应 TTFB：请求写完 → 首字节。
	t.Wait = span(c.wroteReq, c.firstByte)
	// 内容下载：首字节 → 响应体读完。
	t.Download = span(c.firstByte, c.bodyEnd)

	sum := t.DNS + t.Connect + t.TLS + t.Send + t.Wait + t.Download
	t.Other = t.Total - sum
	if t.Other < 0 {
		t.Other = 0
	}
	return t
}

// span 返回两个时间点之间的毫秒数；任一端缺失或倒序返回 0。
func span(a, b time.Time) float64 {
	if a.IsZero() || b.IsZero() || b.Before(a) {
		return 0
	}
	return float64(b.Sub(a)) / float64(time.Millisecond)
}
