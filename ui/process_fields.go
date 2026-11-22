package ui

// 本文件定义 process 各 op 的「结构化参数字段」元数据，供界面按 op 动态渲染表单。
// 字段与 engine/processor*.go 中实际读取的 args key 一一对应，避免界面与引擎脱节。

// procFieldKind 字段控件类型：text 文本框 / number 数字框 / select 下拉。
type procFieldKind int

const (
	procText   procFieldKind = iota // 普通文本框
	procNumber                      // 数字文本框
	procSelect                      // 下拉选择
)

// procField 描述 process 的一个参数字段。
// Key 对应 args 的 key；Label/Placeholder 用于界面；Default 在字段为空时作为占位提示；
// Options 仅 select 类型使用。
type procField struct {
	Key         string
	Label       string
	Placeholder string
	Kind        procFieldKind
	Options     []string
}

// procOpDef 一个 op 的展示名、说明与字段列表。
type procOpDef struct {
	Label  string
	Desc   string
	Fields []procField
}

// 常用字段模板，减少重复定义。
var (
	fFrom     = procField{Key: "from", Label: "来源变量", Placeholder: "rows", Kind: procText}
	fTo       = procField{Key: "to", Label: "输出到变量", Placeholder: "result", Kind: procText}
	fField    = procField{Key: "field", Label: "字段名", Placeholder: "url（可留空按整行）", Kind: procText}
	fFromMany = procField{Key: "from", Label: "来源变量(多个)", Placeholder: "titles,links（逗号分隔）", Kind: procText}
	fCount    = procField{Key: "count", Label: "条数 N", Placeholder: "30", Kind: procNumber}
)

// processOpDefs 返回所有 op 的结构化定义（有序）。
// 顺序与 processOps() 保持一致；新增 op 时此处与 processOps、engine 三处同步。
func processOpDefs() map[string]procOpDef {
	return map[string]procOpDef{
		"set": {
			Label: "设置变量", Desc: "把一个值写入变量，值支持 {{变量}}。",
			Fields: []procField{
				{Key: "name", Label: "变量名", Placeholder: "token", Kind: procText},
				{Key: "value", Label: "值", Placeholder: "abc 或 {{other}}", Kind: procText},
			},
		},
		"append": {
			Label: "追加累积", Desc: "把来源变量的值追加到数组末尾，常用于循环内逐项累积。",
			Fields: []procField{
				fFrom,
				{Key: "to", Label: "累积到数组", Placeholder: "rows（默认）", Kind: procText},
			},
		},
		"json_to_table": {
			Label: "JSON 转表格", Desc: "把 JSON 数组字符串解析成可表格展示的对象数组。",
			Fields: []procField{
				fFrom,
				{Key: "to", Label: "输出到变量", Placeholder: "table（默认）", Kind: procText},
			},
		},
		"filter": {
			Label: "过滤", Desc: "按字段值筛选数组元素。",
			Fields: []procField{
				fFrom, fField,
				{Key: "op", Label: "条件", Placeholder: "eq", Kind: procSelect,
					Options: []string{"eq", "ne", "contains", "gt", "gte", "lt", "lte"}},
				{Key: "value", Label: "比较值", Placeholder: "8.0", Kind: procText},
				{Key: "to", Label: "输出到变量", Placeholder: "filtered（默认）", Kind: procText},
			},
		},
		"sort": {
			Label: "排序", Desc: "按字段对数组排序，支持数值与文本。",
			Fields: []procField{
				fFrom, fField,
				{Key: "order", Label: "顺序", Placeholder: "asc", Kind: procSelect,
					Options: []string{"asc", "desc"}},
				{Key: "to", Label: "输出到变量", Placeholder: "sorted（默认）", Kind: procText},
			},
		},
		"pick": {
			Label: "取字段", Desc: "只保留对象的指定字段，组成新数组。",
			Fields: []procField{
				fFrom,
				{Key: "fields", Label: "保留字段", Placeholder: "title,rating（逗号分隔）", Kind: procText},
				{Key: "to", Label: "输出到变量", Placeholder: "picked（默认）", Kind: procText},
			},
		},
		"flatten": {
			Label: "合并数组", Desc: "把多个数组变量合并成一个数组。",
			Fields: []procField{
				fFromMany,
				{Key: "to", Label: "输出到变量", Placeholder: "allRows（默认）", Kind: procText},
			},
		},
		"dedup": {
			Label: "去重", Desc: "按字段（或整行）去重，保持原顺序。",
			Fields: []procField{
				fFrom, fField,
				{Key: "to", Label: "输出到变量", Placeholder: "deduped（默认）", Kind: procText},
				{Key: "stat", Label: "重复数变量", Placeholder: "dupCount（可留空）", Kind: procText},
			},
		},
		"zip": {
			Label: "合成对象行", Desc: "多个数组按索引合成对象数组，字段与来源一一对应，免写 JS。",
			Fields: []procField{
				fFromMany,
				{Key: "fields", Label: "输出字段名", Placeholder: "title,url（逗号分隔）", Kind: procText},
				{Key: "to", Label: "输出到变量", Placeholder: "rows（默认）", Kind: procText},
			},
		},
		"limit": {
			Label: "取前 N 条", Desc: "取数组区间，例如取前 30 条，免写 JS slice。",
			Fields: []procField{
				fFrom, fCount,
				{Key: "start", Label: "起始下标", Placeholder: "0", Kind: procNumber},
				{Key: "end", Label: "结束下标", Placeholder: "留空则用条数", Kind: procNumber},
				{Key: "to", Label: "输出到变量", Placeholder: "top（默认）", Kind: procText},
			},
		},
		"resolve_url": {
			Label: "URL 绝对化", Desc: "相对链接按基准地址补全，并解码百度/Bing 跳转。",
			Fields: []procField{
				fFrom,
				{Key: "base", Label: "基准地址", Placeholder: "https://example.com（可留空）", Kind: procText},
				{Key: "to", Label: "输出到变量", Placeholder: "urls", Kind: procText},
			},
		},
		"format_time": {
			Label: "时间格式化", Desc: "把时间戳或 RFC3339 时间格式化为可读字符串。",
			Fields: []procField{
				fFrom,
				{Key: "in", Label: "输入类型", Placeholder: "unix", Kind: procSelect,
					Options: []string{"unix", "unixms"}},
				{Key: "layout", Label: "格式模板", Placeholder: "2006-01-02 15:04:05", Kind: procText},
				{Key: "to", Label: "输出到变量", Placeholder: "留空覆盖来源", Kind: procText},
			},
		},
		"join": {
			Label: "拼接字符串", Desc: "把多个值用分隔符拼接成一个字符串。",
			Fields: []procField{
				{Key: "values", Label: "待拼接值", Placeholder: "a,b,c（逗号分隔）", Kind: procText},
				{Key: "sep", Label: "分隔符", Placeholder: "空", Kind: procText},
				{Key: "to", Label: "输出到变量", Placeholder: "joined", Kind: procText},
			},
		},
		"base64encode": {
			Label: "Base64 编码", Desc: "编码字符串或数组中的每一项。",
			Fields: []procField{
				fFrom,
				{Key: "to", Label: "输出到变量", Placeholder: "encoded（默认）", Kind: procText},
			},
		},
		"base64decode": {
			Label: "Base64 解码", Desc: "解码字符串或数组中的每一项（宽容模式）。",
			Fields: []procField{
				fFrom,
				{Key: "to", Label: "输出到变量", Placeholder: "decoded（默认）", Kind: procText},
			},
		},
		"urlencode": {
			Label: "URL 编码", Desc: "对字符串做 URL 编码。",
			Fields: []procField{
				fFrom,
				{Key: "to", Label: "输出到变量", Placeholder: "encoded", Kind: procText},
			},
		},
		"urldecode": {
			Label: "URL 解码", Desc: "对字符串做 URL 解码。",
			Fields: []procField{
				fFrom,
				{Key: "to", Label: "输出到变量", Placeholder: "decoded", Kind: procText},
			},
		},
	}
}

// procOpLabel 返回 op 的中文展示名，未定义时回退原值。
func procOpLabel(op string) string {
	if d, ok := processOpDefs()[op]; ok && d.Label != "" {
		return d.Label
	}
	return op
}
