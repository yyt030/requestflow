package model

// HTTPTiming 一次 HTTP 请求的分段耗时（单位：毫秒），对齐 Chrome DevTools Network Timing。
// 各段之和约等于 Total；Other 吸收排队/停滞及时间点间隙，保证不出现负值与缺口。
type HTTPTiming struct {
	Total     float64 `json:"totalMs"`    // 总耗时
	DNS       float64 `json:"dnsMs"`      // DNS 解析
	Connect   float64 `json:"connectMs"`  // TCP 建连
	TLS       float64 `json:"tlsMs"`      // TLS 握手
	Send      float64 `json:"sendMs"`     // 请求发送（连接就绪→写完请求）
	Wait      float64 `json:"waitMs"`     // 等待首字节 TTFB（写完请求→首字节）
	Download  float64 `json:"downloadMs"` // 内容下载（首字节→读完响应体）
	Other     float64 `json:"otherMs"`    // 排队/停滞等未归类时间
	Reused    bool    `json:"reused"`     // 是否复用了已有连接
	Available bool    `json:"available"`  // 是否成功采集到分段（TCP 响应为 false）
}

// TimingPhase 供 UI 堆叠条与明细展示的一个分段。
type TimingPhase struct {
	Name string
	MS   float64
}

// Phases 按 Chrome Network Timing 的顺序返回各分段（仅返回耗时 > 0 的段）。
func (t *HTTPTiming) Phases() []TimingPhase {
	if t == nil || !t.Available {
		return nil
	}
	all := []TimingPhase{
		{"排队/停滞", t.Other},
		{"DNS 解析", t.DNS},
		{"TCP 建连", t.Connect},
		{"TLS 握手", t.TLS},
		{"请求发送", t.Send},
		{"等待响应 TTFB", t.Wait},
		{"内容下载", t.Download},
	}
	out := make([]TimingPhase, 0, len(all))
	for _, p := range all {
		if p.MS > 0.001 {
			out = append(out, p)
		}
	}
	return out
}
