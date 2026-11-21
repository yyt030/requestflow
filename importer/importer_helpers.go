package importer

import (
	"encoding/json"
	"fmt"
	"strings"
)

func str(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func toJSONString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(b)
}

// pathVarToTemplate 把 {var} 占位转成 {{var}}，避免与本工程模板冲突（已是 {{}} 的不动）。

// pathVarToTemplate 把 {var} 占位转成 {{var}}，避免与本工程模板冲突（已是 {{}} 的不动）。
func pathVarToTemplate(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '{' && i+1 < len(s) && s[i+1] != '{' {
			if j := strings.IndexByte(s[i+1:], '}'); j > 0 {
				b.WriteString("{{" + s[i+1:i+1+j] + "}}")
				i = i + 1 + j + 1
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

var impIDCounter int64

func newImpID() string {
	impIDCounter++
	return fmt.Sprintf("imp-%d", impIDCounter)
}
