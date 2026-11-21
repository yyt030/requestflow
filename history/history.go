// Package history 记录每次请求调试/运行的历史，以 JSONL 追加写入可执行文件
// 同目录的 requestflow-history.jsonl，供 GUI 回看与重新载入。
package history

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Entry 一条请求历史记录。
type Entry struct {
	Time       string `json:"time"`
	Method     string `json:"method"`
	URL        string `json:"url"`
	Status     int    `json:"status"`
	DurationMS int64  `json:"durationMs"`
	Error      string `json:"error,omitempty"`
	Source     string `json:"source,omitempty"` // debug | run
	// 完整请求快照，便于重新载入调试面板
	Headers  map[string]string `json:"headers,omitempty"`
	BodyType string            `json:"bodyType,omitempty"`
	Body     string            `json:"body,omitempty"`
}

var (
	mu         sync.Mutex
	customPath string // 测试可覆盖
)

// path 返回历史文件路径（exe 同目录），可用 SetPath 覆盖（测试用）。
func path() string {
	if customPath != "" {
		return customPath
	}
	if exe, err := os.Executable(); err == nil && exe != "" {
		if d := filepath.Dir(exe); d != "" {
			return filepath.Join(d, "requestflow-history.jsonl")
		}
	}
	return "requestflow-history.jsonl"
}

// SetPath 覆盖历史文件路径（主要供测试使用；传空恢复默认）。
func SetPath(p string) { customPath = p }

// Append 追加一条历史记录。写入失败返回错误，不静默丢弃（调用方决定是否提示）。
func Append(e Entry) error {
	if e.Time == "" {
		e.Time = time.Now().Format("2006-01-02 15:04:05")
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	p := path()
	if d := filepath.Dir(p); d != "" {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintln(f, string(b))
	return err
}

// Load 读取全部历史，最新的在前。文件不存在返回空切片而非错误。
func Load() ([]Entry, error) {
	mu.Lock()
	defer mu.Unlock()
	p := path()
	f, err := os.Open(p)
	if os.IsNotExist(err) {
		return []Entry{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := make([]Entry, 0)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Entry
		if json.Unmarshal(line, &e) == nil {
			out = append(out, e)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	// 反转：最新在前
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// Clear 清空历史（删除文件）。
func Clear() error {
	mu.Lock()
	defer mu.Unlock()
	p := path()
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
