package ui

import (
	"testing"

	"requestflow/engine"
	"requestflow/model"
)

func TestPreviewVarEntriesOriginsAndOrder(t *testing.T) {
	sc := &model.Scenario{
		BaseURL:   "https://example.com",
		Variables: map[string]string{"page": "1"},
	}
	// 一个提取步骤产出 books 变量，供 process 步骤的上游引用。
	extractStep := &model.Step{
		Extract: []model.Extract{{Name: "books", Source: "body", Kind: "jsonpath", Path: "data.books"}},
	}
	processStep := &model.Step{Op: "filter"}
	sc.Steps = []model.Step{*extractStep, *processStep}

	a := &App{
		scenario: sc,
		debugResult: &engine.DebugResult{
			OK: true, StatusCode: 200, Body: sampleJSONResponse, Formatted: sampleJSONResponse, IsJSON: true,
		},
	}

	entries := a.previewVarEntries(&sc.Steps[1])
	byName := map[string]previewVarEntry{}
	for _, e := range entries {
		byName[e.Name] = e
	}

	if e, ok := byName["books"]; !ok || e.Origin != "响应提取" {
		t.Fatalf("books 应标注为 响应提取，实际: %+v", e)
	}
	if e, ok := byName["page"]; !ok || e.Origin != "场景" {
		t.Fatalf("page 应标注为 场景，实际: %+v", e)
	}
	if e, ok := byName["baseUrl"]; !ok || e.Origin != "内置" {
		t.Fatalf("baseUrl 应标注为 内置，实际: %+v", e)
	}

	// 排序：响应提取的 books 应排在场景变量 page 之前。
	pos := map[string]int{}
	for i, e := range entries {
		pos[e.Name] = i
	}
	if pos["books"] >= pos["page"] {
		t.Fatalf("books(响应提取) 应排在 page(场景) 之前: books=%d page=%d", pos["books"], pos["page"])
	}
}

func TestOriginRank(t *testing.T) {
	if !(originRank("响应提取") < originRank("场景") && originRank("场景") < originRank("内置") && originRank("内置") < originRank("循环模拟")) {
		t.Fatal("来源排序应为 响应提取 < 场景 < 内置 < 循环模拟")
	}
}
