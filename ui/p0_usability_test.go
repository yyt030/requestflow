package ui

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"requestflow/model"
)

func TestInsertVarRef(t *testing.T) {
	cases := []struct {
		in   string
		name string
		want string
	}{
		{"", "token", "{{token}}"},
		{"   ", "token", "{{token}}"},
		{"a", "token", "a {{token}}"},
		{"{{baseUrl}}/", "page", "{{baseUrl}}/ {{page}}"},
		{"abc ", "token", "abc {{token}}"},
		{"{{a}}{{b}}", "c", "{{a}}{{b}}{{c}}"},
	}
	for _, c := range cases {
		got := c.in
		insertVarRef(&got, c.name)
		if got != c.want {
			t.Errorf("insertVarRef(%q, %q) = %q, want %q", c.in, c.name, got, c.want)
		}
	}
}

func TestCollectVarNames(t *testing.T) {
	sc := &model.Scenario{
		Variables: map[string]string{"baseURL2": "x", "uid": "1"},
		Steps: []model.Step{
			{
				Type:    model.StepRequest,
				Extract: []model.Extract{{Name: "token"}, {Name: "  title  "}},
			},
			{
				Type: model.StepLoop,
				Loop: &model.Loop{PageVar: "pn", ItemVar: "item", DedupVar: "allRows"},
				Children: []model.Step{
					{Type: model.StepRequest, Extract: []model.Extract{{Name: "nestedVar"}}},
				},
			},
			{
				Type: model.StepBranch,
				Branch: &model.Branch{
					Then: []model.Step{{Extract: []model.Extract{{Name: "thenVar"}}}},
					Else: []model.Step{{Extract: []model.Extract{{Name: "elseVar"}}}},
				},
			},
		},
	}
	got := collectVarNames(sc)
	want := []string{"baseUrl", "baseURL2", "elseVar", "item", "nestedVar", "pn", "thenVar", "title", "token", "allRows", "uid"}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("collectVarNames = %v, want %v", got, want)
	}
}

func TestCollectVarNamesEmptyScenario(t *testing.T) {
	got := collectVarNames(&model.Scenario{})
	if len(got) != 1 || got[0] != "baseUrl" {
		t.Errorf("empty scenario should only have baseUrl, got %v", got)
	}
}

func TestProcessOpDefsCoverAllOps(t *testing.T) {
	defs := processOpDefs()
	for _, op := range processOps() {
		d, ok := defs[op]
		if !ok {
			t.Errorf("processOpDefs 缺少 op %q 的字段定义", op)
			continue
		}
		if len(d.Fields) == 0 {
			t.Errorf("op %q 没有字段定义", op)
		}
		for _, f := range d.Fields {
			if f.Key == "" {
				t.Errorf("op %q 存在空 Key 字段", op)
			}
			if f.Kind == procSelect && len(f.Options) == 0 {
				t.Errorf("op %q 字段 %q 是 select 但没有选项", op, f.Key)
			}
		}
	}
}

func TestProcessOpDefsNoExtra(t *testing.T) {
	defs := processOpDefs()
	inOps := map[string]bool{}
	for _, op := range processOps() {
		inOps[op] = true
	}
	for op := range defs {
		if !inOps[op] {
			t.Errorf("processOpDefs 含未在 processOps 暴露的 op %q", op)
		}
	}
}

func TestVarSummary(t *testing.T) {
	if !strings.Contains(varSummary(nil), "当前无可用变量") {
		t.Error("nil 变量摘要应提示无可用变量")
	}
	got := varSummary([]string{"a", "b", "c"})
	for _, n := range []string{"a", "b", "c"} {
		if !strings.Contains(got, n) {
			t.Errorf("摘要应包含变量 %q: %s", n, got)
		}
	}
}
