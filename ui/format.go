package ui

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"regexp"
	"strings"
	"unicode/utf8"
)

// FormatContent 尝试识别文本类型并美化，返回 (格式化结果, 类型名, 是否发生变化)。
// 支持：
//   - JSON：用 json.Indent 两空格缩进
//   - XML/HTML 片段：用 xml.Decoder token 流重新缩进
//   - 键值对文本（每行 key=value / key: value）：对齐分隔符便于阅读
//
// 无法识别或已格式化时，返回原文，changed=false。
//
// 用于编辑区的「格式化」按钮，与调试响应区已有的美化能力保持一致的体验。
func FormatContent(s string) (string, string, bool) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return s, "", false
	}

	// 1) JSON
	var buf bytes.Buffer
	if json.Valid([]byte(trimmed)) && json.Indent(&buf, []byte(trimmed), "", "  ") == nil {
		out := buf.String()
		if out != s {
			return out, "JSON", true
		}
		return s, "JSON", false
	}

	// 2) XML / HTML（含根元素或片段）
	if out, ok := indentXML(trimmed); ok && out != s {
		return out, "XML", true
	}

	// 3) 键值对文本（至少两行且每行都含分隔符）
	if out, ok := alignKeyValue(s); ok && out != s {
		return out, "键值", true
	}

	return s, "", false
}

// htmlAutoCloseTags 是常见 HTML 自闭合标签，配合 Strict=false 让 XML 解析器
// 能宽松处理 HTML 片段（<br>、<img> 等无闭合标签）。
var htmlAutoCloseTags = []string{"br", "hr", "img", "input", "meta", "link", "area", "base", "col", "embed", "source", "track", "wbr"}

// indentXML 用 token 流把 XML/HTML 片段重新缩进。任何解析错误都视为非 XML。
func indentXML(s string) (string, bool) {
	dec := xml.NewDecoder(strings.NewReader(s))
	dec.Strict = false
	dec.AutoClose = htmlAutoCloseTags

	// 必须以标签开头且含真实元素，避免把纯文本/键值文本当作 CharData 误判为 XML。
	if !strings.HasPrefix(s, "<") {
		return "", false
	}
	var out bytes.Buffer
	depth := 0
	sawElement := false
	const indent = "  "
	lineBreak := func() {
		out.WriteByte('\n')
		out.WriteString(strings.Repeat(indent, depth))
	}

	for {
		tok, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return "", false
		}
		switch t := tok.(type) {
		case xml.StartElement:
			sawElement = true
			if out.Len() > 0 {
				lineBreak()
			}
			out.WriteString("<" + t.Name.Local)
			for _, attr := range t.Attr {
				out.WriteString(" " + attr.Name.Local + `="` + escapeAttr(attr.Value) + `"`)
			}
			out.WriteString(">")
			depth++
		case xml.EndElement:
			depth--
			if depth < 0 {
				return "", false
			}
			lineBreak()
			out.WriteString("</" + t.Name.Local + ">")
		case xml.CharData:
			text := strings.TrimSpace(string(t))
			if text != "" {
				lineBreak()
				out.WriteString(text)
			}
		case xml.Comment:
			lineBreak()
			out.WriteString("<!--" + string(t) + "-->")
		case xml.ProcInst:
			lineBreak()
			out.WriteString("<?" + t.Target + " " + string(t.Inst) + "?>")
		}
	}
	if depth != 0 || !sawElement || out.Len() == 0 {
		return "", false
	}
	return strings.TrimSpace(out.String()) + "\n", true
}

func escapeAttr(v string) string {
	v = strings.ReplaceAll(v, `&`, "&amp;")
	v = strings.ReplaceAll(v, `"`, "&#34;")
	return v
}

var kvLineRE = regexp.MustCompile(`^\s*([^\s:=][^:=]*?)\s*[:=]\s*(.*?)\s*$`)

// alignKeyValue 把多行 key=value / key: value 文本按最长 key 对齐分隔符。
// 只在至少两行、且每行都能解析出 key 时生效，避免误伤普通文本或代码。
func alignKeyValue(s string) (string, bool) {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	type kv struct{ key, sep, val string }
	parsed := make([]kv, 0, len(lines))
	nonEmpty := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			parsed = append(parsed, kv{})
			continue
		}
		m := kvLineRE.FindStringSubmatch(line)
		if m == nil || strings.ContainsAny(m[2], "{}") {
			return "", false // 值里含花括号，可能是脚本或 JSON 片段
		}
		sep := ":"
		if strings.Contains(line, "=") && !strings.Contains(line, ":") {
			sep = "="
		}
		parsed = append(parsed, kv{m[1], sep, m[2]})
		nonEmpty++
	}
	if nonEmpty < 2 {
		return "", false
	}
	maxKey := 0
	for _, p := range parsed {
		if w := utf8.RuneCountInString(p.key); w > maxKey {
			maxKey = w
		}
	}
	var out bytes.Buffer
	for i, p := range parsed {
		if i > 0 {
			out.WriteByte('\n')
		}
		if p.key == "" {
			continue
		}
		out.WriteString(p.key)
		out.WriteString(strings.Repeat(" ", maxKey-utf8.RuneCountInString(p.key)+1))
		out.WriteString(p.sep)
		out.WriteString(" ")
		out.WriteString(p.val)
	}
	return out.String(), true
}
