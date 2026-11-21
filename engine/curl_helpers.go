package engine

import (
	"fmt"
	"strings"
)

// shortURL 去掉协议，取路径用于步骤默认名。
func shortURL(u string) string {
	if i := strings.Index(u, "://"); i >= 0 {
		u = u[i+3:]
	}
	if len(u) > 40 {
		return u[:40] + "…"
	}
	return u
}

// newStepID 生成一个简单的唯一 ID（时间无关，仅界面定位用）。

// newStepID 生成一个简单的唯一 ID（时间无关，仅界面定位用）。
func newStepID() string {
	curlSeq++
	return fmt.Sprintf("curl-%d", curlSeq)
}

var curlSeq int

// splitHeader 把 "Key: value" 拆成键值对。

// splitHeader 把 "Key: value" 拆成键值对。
func splitHeader(s string) (string, string, bool) {
	i := strings.Index(s, ":")
	if i <= 0 {
		return "", "", false
	}
	return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:]), true
}

// joinCookie 合并多个 Cookie 来源。

// joinCookie 合并多个 Cookie 来源。
func joinCookie(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "; " + b
}

// splitBasic 解析 user:pass。

// splitBasic 解析 user:pass。
func splitBasic(s string) (string, string) {
	if i := strings.Index(s, ":"); i > 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

// isIgnoredHeader 过滤掉对导入无意义或会干扰 Go HTTP 客户端的头。

// isIgnoredHeader 过滤掉对导入无意义或会干扰 Go HTTP 客户端的头。
func isIgnoredHeader(k string) bool {
	switch k {
	case "accept-encoding", "content-length", "connection", "host", "authority",
		":authority", ":method", ":path", ":scheme", "upgrade-insecure-requests",
		"sec-fetch-dest", "sec-fetch-mode", "sec-fetch-site", "sec-ch-ua",
		"sec-ch-ua-mobile", "sec-ch-ua-platform", "priority":
		return true
	}
	return false
}

// curlValueOption 把选项映射为语义名。orig 用于区分大小写敏感选项
// （-X 指定方法、-x 指定代理），lower 用于其余长选项匹配。

// curlValueOption 把选项映射为语义名。orig 用于区分大小写敏感选项
// （-X 指定方法、-x 指定代理），lower 用于其余长选项匹配。
func curlValueOption(orig, lower string) (string, bool) {
	if orig == "-X" {
		return "method", true
	}
	if orig == "-x" {
		return "ignore", true // 小写 -x 是 --proxy
	}
	switch lower {
	case "--request":
		return "method", true
	case "-h", "--header":
		return "header", true
	case "--url":
		return "url", true
	case "-d", "--data", "--data-raw", "--data-binary", "--data-ascii", "--data-urlencode":
		return "data", true
	case "-b", "--cookie":
		return "cookie", true
	case "-u", "--user":
		return "user", true
	case "-a", "--user-agent":
		return "user-agent", true
	case "-e", "--referer":
		return "referer", true
	case "-m", "--max-time", "--connect-timeout", "-o", "--output", "--proxy",
		"--resolve", "-w", "--write-out", "--cacert", "--cert", "--key", "-t", "--form":
		return "ignore", true
	}
	return "", false
}

// curlBoolFlag 判断是否为无值标志。

// curlBoolFlag 判断是否为无值标志。
func curlBoolFlag(lower string) bool {
	switch lower {
	case "--compressed", "-k", "--insecure", "-l", "--location", "-v", "--verbose",
		"-s", "--silent", "--show-error", "-i", "--include", "-I", "--head",
		"--http1.1", "--http2", "-#", "--progress-bar", "-g", "--globoff", "--location-trusted":
		return true
	}
	return false
}

// curlHasMethod 检查命令里是否显式指定了请求方法。

// curlHasMethod 检查命令里是否显式指定了请求方法。
func curlHasMethod(tokens []string) bool {
	for _, t := range tokens {
		if t == "-X" || strings.ToLower(t) == "--request" {
			return true
		}
	}
	return false
}

// tokenizeCurl 把 cURL 命令按 shell 规则切分，支持单引号、双引号、转义、
// 以及 bash 的 \ 续行、cmd 的 ^ 续行、PowerShell 的 ` 续行。

// tokenizeCurl 把 cURL 命令按 shell 规则切分，支持单引号、双引号、转义、
// 以及 bash 的 \ 续行、cmd 的 ^ 续行、PowerShell 的 ` 续行。
func tokenizeCurl(s string) ([]string, error) {
	var tokens []string
	var cur strings.Builder
	inSingle, inDouble, escaped := false, false, false
	has := false

	flush := func() {
		if has || cur.Len() > 0 {
			tokens = append(tokens, cur.String())
			cur.Reset()
			has = false
		}
	}

	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		if escaped {
			switch ch {
			case '\n', '\r', '^', '`':
				// 续行符：跳过
			case 'n':
				cur.WriteRune('\n')
			case 't':
				cur.WriteRune('\t')
			default:
				cur.WriteRune(ch)
			}
			escaped = false
			has = true
			continue
		}
		if ch == '\\' && !inSingle {
			escaped = true
			continue
		}
		// cmd 的 ^ 续行、PowerShell 的 ` 续行：仅当紧跟换行时生效（可在引号内）
		if (ch == '^' || ch == '`') && i+1 < len(runes) && (runes[i+1] == '\n' || runes[i+1] == '\r') {
			i++ // 跳过后续换行
			continue
		}
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			has = true
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			has = true
			continue
		}
		if (ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r') && !inSingle && !inDouble {
			flush()
			continue
		}
		cur.WriteRune(ch)
		has = true
	}
	if escaped {
		cur.WriteRune('\\')
	}
	flush()
	if inSingle || inDouble {
		return nil, fmt.Errorf("cURL 命令中有未闭合的引号")
	}
	return tokens, nil
}
