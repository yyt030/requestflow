package engine

import (
	"sync"
	"time"

	"requestflow/model"
)

// StepTrace 单步执行的结构化记录，供测试报告使用。
type StepTrace struct {
	Name       string         `json:"name"`
	Type       model.StepType `json:"type"`
	Status     string         `json:"status"` // pass | fail
	Err        string         `json:"error,omitempty"`
	Started    time.Time      `json:"started"`
	Ended      time.Time      `json:"ended"`
	DurationMS int64          `json:"durationMs"`
}

// TraceLog 收集一次场景执行的步骤与断言记录（线程安全，供并行子任务共享）。
type TraceLog struct {
	mu         sync.Mutex
	Steps      []StepTrace    `json:"steps"`
	Assertions []AssertDetail `json:"assertions"`
}

func (t *TraceLog) addStep(s StepTrace) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.Steps = append(t.Steps, s)
	t.mu.Unlock()
}

func (t *TraceLog) addAssertions(stepName, stepID string, details []AssertDetail) {
	if t == nil {
		return
	}
	t.mu.Lock()
	for _, d := range details {
		d.StepName = stepName
		d.StepID = stepID
		t.Assertions = append(t.Assertions, d)
	}
	t.mu.Unlock()
}

// Snapshot 返回步骤与断言的浅拷贝。
func (t *TraceLog) Snapshot() ([]StepTrace, []AssertDetail) {
	if t == nil {
		return nil, nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	steps := append([]StepTrace(nil), t.Steps...)
	asserts := append([]AssertDetail(nil), t.Assertions...)
	return steps, asserts
}
