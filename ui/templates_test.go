package ui

import (
	"testing"

	"requestflow/model"
)

func TestScenarioTemplates(t *testing.T) {
	templates := scenarioTemplates()
	if len(templates) < 5 {
		t.Fatalf("模板数量不足: %d", len(templates))
	}
	for _, tpl := range templates {
		sc := tpl.Build()
		if sc == nil || sc.Name == "" {
			t.Fatalf("模板 %q 构建的场景无效", tpl.Name)
		}
		if len(sc.Steps) == 0 && tpl.Name != "空白场景" {
			t.Fatalf("模板 %q 无步骤", tpl.Name)
		}
		// 每个步骤都应有有效 ID 与类型
		for i, s := range sc.Steps {
			if s.ID == "" || s.Type == "" {
				t.Fatalf("模板 %q 步骤 %d 缺少 ID/类型", tpl.Name, i)
			}
		}
	}
}

func TestTemplateNewCapabilities(t *testing.T) {
	templates := scenarioTemplates()
	byName := map[string]model.Scenario{}
	for _, tpl := range templates {
		byName[tpl.Name] = *tpl.Build()
	}

	// TCP 模板应含长连接 + 长度头帧 + keepalive
	tcp := byName["TCP 长连接 + 长度头帧"]
	if len(tcp.Steps) == 0 || tcp.Steps[0].TCPConnMode != "long" ||
		tcp.Steps[0].TCPFrameType != "ascii" || tcp.Steps[0].TCPKeepAlive != "30" {
		t.Fatalf("TCP 模板字段不完整: %+v", tcp.Steps)
	}

	// 搜索采集模板应含 base64decode 数组 + dedup + CSS 提取
	search := byName["搜索结果采集（含 base64 解码去重）"]
	foundDecode, foundDedup, foundCSS := false, false, false
	for _, s := range search.Steps {
		if s.Op == "base64decode" {
			foundDecode = true
		}
		if s.Op == "dedup" {
			foundDedup = true
		}
		for _, ex := range s.Extract {
			if ex.Kind == "css" {
				foundCSS = true
			}
		}
	}
	if !foundDecode || !foundDedup || !foundCSS {
		t.Fatalf("搜索模板缺少新能力: decode=%v dedup=%v css=%v", foundDecode, foundDedup, foundCSS)
	}

	// 翻页模板应配置累积去重字段
	crawl := byName["翻页采集（CSS + 去重 + 导出）"]
	if len(crawl.Steps) == 0 || crawl.Steps[0].Loop == nil || crawl.Steps[0].Loop.DedupVar != "allRows" {
		t.Fatalf("翻页模板未配置累积去重: %+v", crawl.Steps)
	}
}
