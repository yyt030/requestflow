package engine

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// resultTable 把任意数组变量规整为表头 + 行，供导出使用。
type resultTable struct {
	headers []string
	rows    [][]string
}

// BuildTable 把任意数组变量规整为表头与行，供 UI 结果表格预览使用。
// 支持 []map、[]interface{}、JSON 数组字符串及标量数组。
func BuildTable(v interface{}) (headers []string, rows [][]string, err error) {
	t, err := toResultTable(v)
	if err != nil {
		return nil, nil, err
	}
	return t.headers, t.rows, nil
}

// IsTabular 判断变量值是否可以作为表格展示（数组或 JSON 数组字符串）。
func IsTabular(v interface{}) bool {
	switch v.(type) {
	case []interface{}, []map[string]interface{}, []string:
		return true
	case string:
		trimmed := strings.TrimSpace(v.(string))
		return strings.HasPrefix(trimmed, "[")
	}
	return false
}

// toResultTable 将变量值（数组 map / 数组标量 / 字符串 JSON）规整为表格。
func toResultTable(v interface{}) (*resultTable, error) {
	list, ok := toSlice(v)
	if !ok {
		return nil, fmt.Errorf("导出数据不是数组")
	}
	if len(list) == 0 {
		return &resultTable{}, nil
	}

	// 全部为 map：收集列名后排序，避免 map 遍历随机导致列序不稳定
	headers := []string{}
	allMaps := true
	for _, item := range list {
		if _, ok := item.(map[string]interface{}); !ok {
			allMaps = false
			break
		}
	}
	if allMaps {
		seen := map[string]bool{}
		for _, item := range list {
			for k := range item.(map[string]interface{}) {
				if !seen[k] {
					seen[k] = true
					headers = append(headers, k)
				}
			}
		}
		sort.Strings(headers)
		rows := make([][]string, 0, len(list))
		for _, item := range list {
			m := item.(map[string]interface{})
			row := make([]string, 0, len(headers))
			for _, h := range headers {
				row = append(row, cellStr(m[h]))
			}
			rows = append(rows, row)
		}
		return &resultTable{headers: headers, rows: rows}, nil
	}

	// 标量数组：单列
	headers = []string{"value"}
	rows := make([][]string, 0, len(list))
	for _, item := range list {
		rows = append(rows, []string{cellStr(item)})
	}
	return &resultTable{headers: headers, rows: rows}, nil
}

func cellStr(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return toStr(x)
		}
		return string(b)
	}
}

// runExport 把变量导出为文件，返回写入的绝对路径。
func runExport(ctx *Context, from, format, path string) (string, error) {
	data := ctx.Get(from)
	format = strings.ToLower(format)
	if format == "" {
		format = "xlsx"
	}
	if path == "" {
		path = "result." + format
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}

	switch format {
	case "json":
		f, err := os.Create(path)
		if err != nil {
			return "", err
		}
		defer f.Close()
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		if err := enc.Encode(data); err != nil {
			return "", err
		}
	case "txt":
		table, err := toResultTable(data)
		if err != nil {
			return "", err
		}
		var b strings.Builder
		for _, row := range table.rows {
			b.WriteString(strings.Join(row, "\t"))
			b.WriteString("\n")
		}
		if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
			return "", err
		}
	case "csv":
		table, err := toResultTable(data)
		if err != nil {
			return "", err
		}
		f, err := os.Create(path)
		if err != nil {
			return "", err
		}
		defer f.Close()
		w := csv.NewWriter(f)
		if len(table.headers) > 0 {
			if err := w.Write(table.headers); err != nil {
				return "", err
			}
		}
		for _, row := range table.rows {
			if err := w.Write(row); err != nil {
				return "", err
			}
		}
		w.Flush()
		return path, w.Error()
	case "xlsx":
		table, err := toResultTable(data)
		if err != nil {
			return "", err
		}
		f := excelize.NewFile()
		defer f.Close()
		sheet := "Sheet1"
		rowIdx := 1
		if len(table.headers) > 0 {
			for colIdx, h := range table.headers {
				cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx)
				_ = f.SetCellValue(sheet, cell, h)
			}
			rowIdx++
		}
		for _, row := range table.rows {
			for colIdx, val := range row {
				cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx)
				_ = f.SetCellValue(sheet, cell, val)
			}
			rowIdx++
		}
		if err := f.SaveAs(path); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("未知导出格式: %s", format)
	}
	return path, nil
}
