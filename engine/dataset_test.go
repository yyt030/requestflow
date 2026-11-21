package engine

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"requestflow/model"
)

func TestRunDatasetInjectsVariables(t *testing.T) {
	var gotNames []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotNames = append(gotNames, r.URL.Query().Get("who"))
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	sc := &model.Scenario{
		Name:      "ds",
		Variables: map[string]string{"who": "global"},
		Steps: []model.Step{{
			Type: model.StepRequest, Enabled: true, Method: "GET",
			URL: srv.URL, Query: map[string]string{"who": "{{who}}"},
		}},
	}
	rows := []DataRow{{"who": "张三"}, {"who": "李四"}}
	results := RunDataset(sc, rows, nil)
	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}
	if !results[0].Result.OK || !results[1].Result.OK {
		t.Fatalf("迭代失败: %+v", results)
	}
	if !reflect.DeepEqual(gotNames, []string{"张三", "李四"}) {
		t.Fatalf("got who=%v, want [张三 李四]", gotNames)
	}
}

func TestLoadDatasetCSV(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "users.csv")
	content := "name,age,city\n张三,18,北京\n李四,20,上海\n"
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	rows, err := LoadDataset(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	want := DataRow{"name": "张三", "age": "18", "city": "北京"}
	if !reflect.DeepEqual(rows[0], want) {
		t.Fatalf("row0 = %v, want %v", rows[0], want)
	}
}

func TestLoadDatasetJSON(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "users.json")
	content := `[{"name":"张三","age":18,"ok":true},{"name":"李四","age":20,"ok":false}]`
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	rows, err := LoadDataset(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0]["age"] != "18" || rows[0]["ok"] != "true" {
		t.Fatalf("unexpected rows: %v", rows)
	}
	// 包裹对象形态
	p2 := filepath.Join(dir, "wrapped.json")
	if err := os.WriteFile(p2, []byte(`{"data":[{"x":"1"}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	rows2, err := LoadDataset(p2)
	if err != nil || len(rows2) != 1 || rows2[0]["x"] != "1" {
		t.Fatalf("wrapped rows=%v err=%v", rows2, err)
	}
}
