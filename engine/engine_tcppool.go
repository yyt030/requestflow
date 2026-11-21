package engine

import (
	"net"
	"sync"
	"time"
)

// tcpConnPool 线程安全的长连接池。
type tcpConnPool struct {
	mu    sync.Mutex
	conns map[string]*tcpConnEntry
}

type tcpConnEntry struct {
	conn    net.Conn
	timeout time.Duration
}

func newTCPConnPool() *tcpConnPool {
	return &tcpConnPool{conns: map[string]*tcpConnEntry{}}
}

// get 取连接；不存在返回 nil。

// get 取连接；不存在返回 nil。
func (p *tcpConnPool) get(key string) *tcpConnEntry {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.conns[key]
}

// put 存入连接；若同名连接已存在则先关闭旧连接。

// put 存入连接；若同名连接已存在则先关闭旧连接。
func (p *tcpConnPool) put(key string, entry *tcpConnEntry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if old := p.conns[key]; old != nil && old.conn != entry.conn {
		_ = old.conn.Close()
	}
	p.conns[key] = entry
}

// remove 关闭并移除连接。

// remove 关闭并移除连接。
func (p *tcpConnPool) remove(key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if entry := p.conns[key]; entry != nil {
		_ = entry.conn.Close()
		delete(p.conns, key)
		return true
	}
	return false
}

// closeAll 关闭所有长连接。

// closeAll 关闭所有长连接。
func (p *tcpConnPool) closeAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for key, entry := range p.conns {
		_ = entry.conn.Close()
		delete(p.conns, key)
	}
}
