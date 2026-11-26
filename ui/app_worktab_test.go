package ui

import (
	"fmt"
	"testing"
)

// newWorkTab 在测试中构造一个最小工作标签。
func testWorkTab(id string) workTab {
	return workTab{id: id, label: id, kind: "step", step: 0}
}

// TestCloseWorkTabNoPanic 验证关闭任意位置标签都不越界，且至少保留一个标签。
func TestCloseWorkTabNoPanic(t *testing.T) {
	for n := 1; n <= 5; n++ {
		for closeAt := 0; closeAt < n || (n == 1 && closeAt == 0); closeAt++ {
			if closeAt >= n {
				break
			}
			app := &App{activeWork: n - 1}
			for i := 0; i < n; i++ {
				app.workTabs = append(app.workTabs, testWorkTab(fmt.Sprintf("t%d", i)))
			}
			before := len(app.workTabs)
			app.closeWorkTab(closeAt)
			want := before
			if before > 1 {
				want = before - 1
			}
			if len(app.workTabs) != want {
				t.Fatalf("n=%d closeAt=%d: 关闭后数量=%d 期望=%d", n, closeAt, len(app.workTabs), want)
			}
			if app.activeWork < 0 || app.activeWork >= len(app.workTabs) {
				t.Fatalf("n=%d closeAt=%d: activeWork=%d 越界(len=%d)", n, closeAt, app.activeWork, len(app.workTabs))
			}
		}
	}
}

// TestCloseWorkTabActiveShift 验证关闭激活标签之前的标签时，激活索引随之前移，指向同一逻辑标签。
func TestCloseWorkTabActiveShift(t *testing.T) {
	app := &App{activeWork: 2}
	for i := 0; i < 4; i++ {
		app.workTabs = append(app.workTabs, testWorkTab(fmt.Sprintf("t%d", i)))
	}
	// 激活 t2，关闭它前面的 t0：期望激活索引变为 1，仍指向 t2。
	app.closeWorkTab(0)
	if app.activeWork != 1 || app.workTabs[app.activeWork].id != "t2" {
		t.Fatalf("关闭前置标签后激活错位: activeWork=%d tab=%s", app.activeWork, app.workTabs[app.activeWork].id)
	}
}
