# RequestFlow · HTTP 流程自动化工具

一个用 Go + [mygo](https://github.com/egoist/mygo) 原生 UI 构建的桌面工具：把「登录 → 按顺序发请求 → 提取数据 → 处理 → 导出」配置成可视化步骤，**一键执行**，类似接口自动化测试。单个二进制、无 cgo、原生界面秒开。

## 特性

- **可视化配置**：左侧步骤列表可勾选启用、上下排序、删除；**全局变量可视化编辑（增/改/删）**；中间按步骤类型显示表单。
- **执行可视化**：底部三视图切换 —— **日志 / 结果表 / 变量**。结果表用原生表格展示提取的数组数据（可选数据源、列排序、刷新）；变量视图列出全部运行时变量。
- **可配置顺序步骤**：`request / loop / process / script / assert / export` 六种步骤自由编排。
- **两种登录态**：Bearer Token 从响应提取到变量后用 `{{token}}` 注入；Cookie 会话由全局 CookieJar 自动跨步骤携带。
- **循环翻页**：`count`（固定次数）、`foreach`（遍历数组）、`until`（翻到空数据自动停）三种模式，页码变量自动递增。
- **内置处理 + 自定义脚本**：常用处理逻辑内置；复杂逻辑用 JavaScript（goja）直接处理响应。
- **多格式导出**：xlsx / csv / txt / json。
- **从 Chrome 一键导入 cURL**：DevTools 中右键请求 → Copy as cURL，粘贴后一键解析成请求步骤，自动识别 Bearer Token、Cookie、查询参数与请求体，支持 bash / cmd / PowerShell 三种格式。
- **接口调试**：工具栏「⚡ 接口调试」打开独立调试面板，单独发送请求即时查看响应，无需运行整个场景。支持 method/url/headers/query/body/认证/超时/TLS；响应区分「响应体 / 响应头 / Cookie / cURL」四视图，JSON 自动美化，显示状态码、耗时、字节数。request 步骤编辑器里还有「⚡ 试发送此请求」按钮，用当前步骤配置（含 `{{变量}}` 渲染）直接调试；调试响应可一键反向生成 cURL 并导回场景。
- **类似自动化测试**：每步可配断言（状态码 / JSONPath / 文本包含 / 变量比较），失败即中止，也可设「失败后继续」。
- **无界面模式**：`requestflow -run scenario.json` 便于 CI / 命令行跑批。

## 环境要求

- **Go 1.27+**（mygo 要求，工具链会自动下载）
- 桌面环境用于 GUI；无界面模式无需桌面

```bash
./build.sh                              # 推荐：自动注入版本号构建（见下）
./requestflow                           # 启动 GUI
./requestflow -run scenarios/demo.json  # 无界面运行
./requestflow -version                  # 查看版本号
# 或普通编译（版本号为 dev）：
go build -o requestflow .
```

## 打包（Windows 分发）

默认只打 **Windows / amd64**，一条命令生成可直接分发的 zip（含 exe、README、示例场景、使用说明与 SHA256 校验文件）：

```bash
./pack.sh                       # 默认 windows/amd64 → dist/requestflow-<版本>-windows-amd64.zip
./pack.sh windows/arm64         # 需要其他平台时显式指定 <goos>/<goarch>
VERSION=v2.0.0 ./pack.sh        # CI 中手动指定版本号
```

产物在 `dist/`，解压后双击 `requestflow.exe` 即可用 GUI；命令行支持 `-run` / `-version`。版本号由 git tag + commit 自动注入，无需 cgo。

## 版本号

版本号由 git 的 **tag + commit hash** 自动生成，构建时通过 `-ldflags -X` 注入二进制：

| 字段 | 来源 | 示例 |
|---|---|---|
| 版本 | `git describe --tags --always --dirty` | `v1.0.0`、`v1.0.0-3-gabc1234`、`v1.0.0-dirty` |
| 提交哈希 | `git rev-parse HEAD` | `55a27e3d3219...` |
| 构建时间 | `date -u`（RFC3339） | `2026-11-16T14:39:44Z` |

打 tag 后构建即为干净版本号；tag 之后有新提交会自动带 `-N-gHASH`；工作区有未提交改动带 `-dirty`。

```bash
./build.sh                # 输出 ./requestflow，自动注入上述信息
./build.sh -o ./bin/rf    # 指定输出路径
VERSION=v2.0.0 ./build.sh # CI 中手动指定版本
```

**查看方式**：
- CLI：`./requestflow -version` 打印单行完整版本
- GUI：窗口标题栏显示版本号；工具栏右侧的 `ⓘ 版本号` 按钮点击后弹出完整信息（版本/哈希/构建时间/Go 版本/平台）

## 设计架构

```
┌─ UI 层 (mygo ui) ── 场景/步骤列表 · 步骤编辑器 · 执行日志 ─┐
├─ 控制层 ── 配置加载/保存(JSON) · 执行调度 · 状态显示 ──────┤
├─ 执行引擎 ─ 变量上下文 · HTTP执行 · 提取/断言 · 循环 · 处理 · JS脚本 ┤
└─ 基础设施 ─ net/http+CookieJar · excelize · gjson · goja ───┘
```

**一个场景 = 一个 JSON 文件**，步骤按顺序执行，步骤间通过变量上下文传递数据。源码分层：`model/`（数据模型）、`engine/`（执行引擎，含单元测试）、`ui/`（界面）。

## 快速开始（GUI）

1. 启动后点「新建」或「打开」场景。
2. 顶部填写 **Base URL**（如 `https://api.example.com`）。
3. 左侧展开「全局变量」，用 `变量名 / 值 / +` 添加变量（步骤里用 `{{变量名}}` 引用）。
4. 左侧「+ 添加」选择步骤类型，添加后用 **↑/↓** 调整顺序。
5. 中间配置当前步骤：请求方法/URL/Body、提取规则、断言等。
6. 点「▶ 一键运行」；底部切到 **日志** 看实时过程、**结果表** 看提取的数据表格、**变量** 看全部运行时变量；可随时「■ 停止」。
7. 「保存」把场景导出为 JSON，可复用或纳入版本管理。

**不想手工配请求？** 点工具栏「⇥ 导入 cURL」：在 Chrome 开发者工具 Network 面板右键目标请求 → **Copy → Copy as cURL**，粘贴到面板里点「解析并添加为步骤」。URL、方法、请求头、Cookie、查询参数、请求体自动填好；若请求 URL 以顶部 Base URL 开头，会自动替换成 `{{baseUrl}}` 便于环境切换。

**只想调试单个接口？** 点工具栏「⚡ 接口调试」打开调试面板：上半区配置请求（方法 / URL / Headers / Query / Body / 认证 / 超时 / TLS），点「▶ 发送」；下半区即时查看响应，四个标签页分别是**响应体**（JSON 自动美化）、**响应头**、**Cookie**、**cURL**（当前请求反向生成的命令，可一键「⇥ cURL 导入」导回场景）。URL、Header、Body 都支持 `{{变量}}`，自动套用场景全局变量与 Base URL。如果在编排模式下想快速验证某个 request 步骤，可直接在步骤编辑器点「⚡ 试发送此请求」，会自动载入该步骤配置并发起调试。

## 场景 JSON 格式

```jsonc
{
  "name": "场景名",
  "baseUrl": "https://api.example.com",
  "variables": { "pageSize": "20" },
  "steps": [
    {
      "id": "login", "name": "登录", "type": "request", "enabled": true,
      "method": "POST", "url": "/api/login",
      "headers": { "Content-Type": "application/json" },
      "body": "{\"user\":\"{{username}}\",\"pwd\":\"{{password}}\"}",
      "extract": [
        { "name": "token", "source": "body", "kind": "jsonpath", "path": "data.token" }
      ],
      "assert": [
        { "name": "登录成功", "source": "jsonpath", "path": "code", "op": "eq", "value": "0" }
      ]
    },
    {
      "id": "pages", "name": "翻页", "type": "loop", "enabled": true,
      "loop": { "mode": "until", "breakWhen": "{{len(rows)}} == 0",
                "pageVar": "page", "pageStart": 1, "pageStep": 1, "max": 100 },
      "children": [
        { "name": "拉取", "type": "request", "enabled": true,
          "method": "GET", "url": "/api/orders",
          "query": { "page": "{{page}}", "size": "{{pageSize}}" },
          "authType": "bearer", "authToken": "{{token}}",
          "extract": [ { "name": "rows", "kind": "jsonpath", "path": "data.list" } ] },
        { "name": "累积", "type": "process", "enabled": true,
          "op": "flatten", "args": { "from": "allRows,rows", "to": "allRows" } }
      ]
    },
    { "name": "导出", "type": "export", "enabled": true,
      "from": "allRows", "format": "xlsx", "path": "orders.xlsx" }
  ]
}
```

`scenarios/demo.json`（可直接跑，连 httpbin）和 `scenarios/pagination.json`（真实业务模板）是完整示例。

## 步骤类型

| type | 作用 | 关键字段 |
|---|---|---|
| `request` | 发 HTTP 请求 | `method` `url` `headers` `query` `body` `authType` `authToken` `extract` `assert` |
| `loop` | 循环子步骤（翻页） | `loop`（见下表）+ `children` |
| `process` | 内置数据处理 | `op` + `args` |
| `script` | 自定义 JS 处理 | `code` |
| `assert` | 独立断言（针对最近响应） | `assert` |
| `export` | 导出结果 | `from` `format` `path` |

所有步骤都有 `id` `name` `enabled` `continueOnFail`。

### 循环模式（loop）

| mode | 说明 | 关键字段 |
|---|---|---|
| `count` | 固定次数 | `count` |
| `foreach` | 遍历数组变量 | `listVar` `itemVar` `indexVar` |
| `until` | 直到条件成立退出 | `breakWhen`，如 `{{len(rows)}} == 0` |

`pageVar` / `pageStart` / `pageStep` 让页码变量每轮自动递增，`max` 防止死循环（默认 100）。

### 提取器（extract）

`source`：`body`（默认）/ `header` / `cookie` / `status`；`kind`：`jsonpath` / `regex` / `whole`。
例如 `{"name":"token","kind":"jsonpath","path":"data.token"}`、`{"name":"id","kind":"regex","path":"id=(\\d+)","group":1}`。

### 断言（assert）

`source`：`status` / `jsonpath` / `body` / `header` / `variable`；`op`：`eq` `ne` `contains` `exists` `gt` `gte` `lt` `lte`。

## 变量与表达式

用 `{{表达式}}` 在任意字符串字段中插值。支持：

- 变量与路径：`{{token}}`、`{{rows.0.id}}`
- 比较：`{{len(rows)}} == 0`、`{{page}} >= 3`
- 内置函数：`len` `empty` `add` `sub` `upper` `lower` `contains` `default` `toint` `tostring` `json`
- 字面量：`"字符串"`、`123`、`true` / `false`

例：`{{add(page,1)}}`、`{{upper(name)}}`、`{{default(nick, "匿名")}}`。

## 内置处理（process op）

| op | 说明 | args |
|---|---|---|
| `set` | 设置变量 | `name` `value` |
| `append` | 循环内累积到数组 | `from` `to` |
| `json_to_table` | JSON 数组转表格变量 | `from` `to` |
| `filter` | 过滤数组 | `from` `field` `op` `value` `to` |
| `sort` | 排序 | `from` `field` `order=asc/desc` `to` |
| `pick` | 取指定字段 | `from` `fields=a,b,c` `to` |
| `flatten` | 合并多个数组变量 | `from=a,b` `to` |
| `format_time` | 时间戳格式化 | `from` `in=unix/unixms` `layout` `to` |
| `join` `base64encode` `base64decode` `urlencode` `urldecode` | 字符串/编解码工具 | 见 UI 提示 |

## 自定义脚本（script）

用 JavaScript（goja，纯 Go 引擎）处理响应。注入对象：

- `vars`：全部运行时变量，可读写（改动自动写回）
- `last`：最近一次响应 `{ status, body, json, headers }`
- `log(...)`：输出到控制台
- `JSON.stringify` / `JSON.parse`

```js
var data = last.json.data.list;
vars.rows = data.filter(function(x){ return x.amount > 100; });
vars.total = data.length;
log("共 " + vars.total + " 条");
```

## 测试

```bash
go test ./...       # 全部包（engine / ui / version）
```

已覆盖：登录提取 Token + Bearer 注入、Cookie 会话、循环翻页终止、脚本写变量 + 导出、表达式求值、表格构建、变量增删改、版本信息格式化。

## 目录

```
main.go            入口（GUI / -run 无界面 / -version）
model/model.go     数据模型
engine/            执行引擎 + cURL 解析器 + 接口调试 + 单元测试
ui/app.go          mygo 原生界面
version/version.go 版本信息（构建期注入）+ 单元测试
build.sh           自动注入版本号的构建脚本
scenarios/         示例场景
```
