package engine

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"requestflow/model"
)

// DataRow 一行数据：列名 → 值。运行时注入到变量上下文（同名变量覆盖场景全局变量）。
type DataRow map[string]string

// LoadDataset 从 CSV 或 JSON 文件加载数据集。
//   - CSV：第一行为表头，其余每行一条记录。
//   - JSON：支持两种形态：
//     1) 对象数组  [ {"name":"张三","age":"18"}, ... ]
//     2) 包裹对象  { "data": [ ... ] } 或 { "rows": [ ... ] }
// 非字符串值（数字/布尔）会被转为字符串。
func LoadDataset(path string) ([]DataRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取数据文件失败: %w", err)
	}
	ext := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(path), ""))
	if idx := strings.LastIndex(ext, "."); idx >= 0 {
		ext = ext[idx:]
	}
	if ext == ".json" {
		return parseJSONDataset(data)
	}
	// 默认按 CSV 解析（.csv / .txt / 无后缀）
	return parseCSVDataset(data)
}

func parseCSVDataset(data []byte) ([]DataRow, error) {
	reader := csv.NewReader(strings.NewReader(strings.ReplaceAll(string(data), "\r\n", "\n")))
	reader.FieldsPerRecord = -1 // 允许行长度不一
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("解析 CSV 失败: %w", err)
	}
	if len(records) == 0 {
		return []DataRow{}, nil
	}
	header := records[0]
	rows := make([]DataRow, 0, len(records)-1)
	for i := 1; i < len(records); i++ {
		row := DataRow{}
		for j, h := range header {
			key := strings.TrimSpace(h)
			if key == "" || j >= len(records[i]) {
				continue
			}
			row[key] = strings.TrimSpace(records[i][j])
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func parseJSONDataset(data []byte) ([]DataRow, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return []DataRow{}, nil
	}
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("解析 JSON 数据失败: %w", err)
	}
	var list []interface{}
	switch v := raw.(type) {
	case []interface{}:
		list = v
	case map[string]interface{}:
		for _, key := range []string{"data", "rows", "items", "records"} {
			if arr, ok := v[key].([]interface{}); ok {
				list = arr
				break
			}
		}
		if list == nil {
			return nil, fmt.Errorf("JSON 对象需包含 data/rows/items/records 数组")
		}
	default:
		return nil, fmt.Errorf("JSON 数据顶层须为数组或对象")
	}
	rows := make([]DataRow, 0, len(list))
	for _, item := range list {
		obj, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("数据项须为对象，实际为 %T", item)
		}
		row := DataRow{}
		for k, val := range obj {
			row[k] = dataValueString(val)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func dataValueString(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return fmt.Sprint(x)
		}
		return string(b)
	}
}

// DatasetResult 一次数据迭代的结果。
type DatasetResult struct {
	Index    int                 // 从 1 开始
	Data     DataRow             // 本行数据
	Result   *Result             // 场景执行结果
	Vars     map[string]interface{} // 本次迭代结束时的变量快照
}

// IterationEvent 迭代过程中产生的事件，包装了迭代序号与原始 Event。
type IterationEvent struct {
	Iteration int
	Event     Event
}

// RunDataset 用数据集逐行运行场景。每次迭代创建独立引擎（独立 CookieJar 与上下文），
// 把该行数据注入为变量（覆盖场景同名全局变量），完整跑一遍场景。
// 某一行失败默认继续下一行；全部跑完后汇总。
func RunDataset(sc *model.Scenario, rows []DataRow, onEvent func(IterationEvent)) []DatasetResult {
	results := make([]DatasetResult, 0, len(rows))
	for i, row := range rows {
		iter := i + 1
		// 合并场景全局变量与本行数据（行数据优先）
		vars := make(map[string]string, len(sc.Variables)+len(row))
		for k, v := range sc.Variables {
			vars[k] = v
		}
		for k, v := range row {
			vars[k] = v
		}
		iterScenario := *sc
		iterScenario.Variables = vars

		var wrapped func(Event)
		if onEvent != nil {
			wrapped = func(ev Event) {
				onEvent(IterationEvent{Iteration: iter, Event: ev})
			}
		}
		eng := NewEngine(&iterScenario, wrapped)
		res := eng.Run()
		results = append(results, DatasetResult{
			Index:  iter,
			Data:   row,
			Result: res,
			Vars:   res.Vars,
		})
	}
	return results
}
