# RequestFlow · HTTP 流程自动化工具

一个用 Go + [mygo](https://github.com/egoist/mygo) 原生 UI 构建的桌面工具：把「登录 → 按顺序发请求 → 提取数据 → 处理 → 导出」配置成可视化步骤，**一键执行**，类似接口自动化测试。单个二进制、无 cgo、原生界面秒开。

## 特性

- **可视化配置**：左侧步骤列表可勾选启用、上下排序、删除；**全局变量可视化编辑（增/改/删）**；中间按步骤类型显示表单。
- **执行可视化**：底部三视图切换 —— **日志 / 结果表 / 变量**。结果表用原生表格展示提取的数组数据（可选数据源、列排序、刷新）；变量视图列出全部运行时变量。
- **可配置顺序步骤**：`request / branch / switch / loop / parallel / merge / process / script / assert / export` 十种步骤自由编排。
- **条件分支（可视化配置）**：`branch` 步骤按条件走不同分支——**变量 + 操作符（大于/小于/等于/包含/为空…）+ 值** 下拉拼装，支持 and/or 组合与嵌套；条件成立执行 then、否则执行 else，各分支内可放请求/脚本/处理子步骤。也可切换高级模式直接写 `{{amount}} > 0` 表达式。
- **多分支 switch**：`switch` 步骤配置多个 case，**自上而下匹配第一个满足的 case** 后执行其步骤并跳出，都不满足则走 default。case 条件与 branch 一致（and/or + 中文操作符 / 高级表达式），支持用 ↑/↓ 调整优先级。适合按状态码、等级、类型分发到不同处理流程。
- **扇出（并发）**：`parallel` 步骤把多个子任务**并发执行**（各自独立变量沙箱、共享会话，类似 Go 的 goroutine 扇出），全部完成后按 `array / object / table / concat` 方式把各任务产物汇合到一个变量。同时自动登记每个任务的状态表（见下）。任一任务失败默认忽略并继续，可勾选「失败即整体失败」。
- **扇入（门控）**：`merge` 步骤作为屏障，**等待上游并行任务全部完成后**用高级表达式检查各任务状态/产物，条件成立才放行后续步骤，否则**中止场景**（类似断言失败）。表达式支持 `&&` / `||` / 比较 / `len` 等函数，可引用 `task.<任务名>.status/output/error/duration` 及 `task.allSuccess/success/failed/total`。由此实现「必须所有并行任务都成功且满足条件，下一步才能开始」的依赖关系。
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

### 步骤间如何衔接 + 条件分支

步骤共享一个**变量上下文**：上一步用 `extract` 把响应字段（JSONPath / 正则 / Header / Cookie）提取成变量，下一步在 URL / Header / Body 里用 `{{变量名}}` 引用——登录 token、查询结果都靠它串联。`branch` 步骤在这个基础上做判断，实现「根据上一步返回内容走不同请求」：

> 例：上一步提取金额为 `balance`，正的走详情页、负的走告警页

1. 步骤 1（request）：请求余额接口，`extract` 把金额存为 `balance`（JSONPath `args.amount`）。
2. 步骤 2（branch）：条件配 `balance` **大于** `0`；then 放「请求详情页」、else 放「请求告警页」——两个分支都是普通的 request 子步骤，可继续提取变量。

条件用界面下拉拼装（变量 + 操作符 + 值），支持 **and/or** 多个条件组合；操作符有大于/大于等于/小于/小于等于/等于/不等于/包含/不包含/存在/为空/为真，数值和字符串都能比。多条件用 and/or 组合即可（不必写 `&&`）；若偏好表达式，「高级」框支持单个比较如 `{{amount}} > 0` 或 `{{len(rows)}} == 0`（走 `Eval`）。分支可**嵌套**（分支的 then/else 里再放分支），也可放在 loop 循环体内。完整可运行示例见 `scenarios/branch-demo.json`。

### 常用任务组合组件：switch 与扇出扇入

当分支多于两个（如按状态码 200/404/500 分流），用 `switch` 更清晰：界面里添加多个 case，每个 case 配条件与子步骤，用 **↑/↓** 调整优先级。执行时**自上而下匹配第一个满足的 case**，命中后执行它的步骤并跳出整个 switch；都不命中则执行 default。case 条件语法与 branch 完全相同（中文操作符 / and/or / 高级表达式）。

`parallel` 把多个子任务**同时执行**，适合并行拉取多个接口或多页数据。每个子任务在自己的**变量沙箱**中跑（写变量互不污染、不回传父级，但共享登录会话），任务内用 `extract` 取数，并在任务上指定「产物变量名」（`var`）或「常量文本」（`value`）。全部完成后按 `mode` 汇合到目标变量：

| mode | 汇合结果 |
|---|---|
| `array` | 各任务产物按顺序组成数组 |
| `object` | 以任务名为 key 组成对象 |
| `table` | 各任务产物（数组）展开合并成一张表，**可直接导出** |
| `concat` | 各任务产物转为文本拼接（`separator` 指定分隔符） |

> 例：并行拉取来源一、来源二，各提取一个值 → `table` 模式汇成 `allRows` → 后续 export 步骤统一导出。
> 产物顺序按任务配置顺序固定，与实际完成先后无关；任一任务失败默认忽略并继续汇合，勾选 `failFast` 则整体失败。

完整可运行示例见 `scenarios/components-demo.json`（switch 评级 + 并行拉取汇合导出）。

### 扇出 / 扇入（类似 Go 的 fan-out / fan-in）

把 `parallel`（扇出）和 `merge`（扇入）搭配使用，实现「**多个并行任务全部成功且满足条件，下一步才能开始**」的依赖门控：

1. **扇出**：一个 `parallel` 步骤里配多个命名子任务，它们**并发执行**（类似 Go 的 goroutine）。执行完自动登记状态表到变量 `task`（可用 `statusVar` 改名）：

   | 引用 | 含义 |
   |---|---|
   | `task.<任务名>.status` | `success` / `fail` |
   | `task.<任务名>.output` | 该任务产物（按 `var`/`value`） |
   | `task.<任务名>.error` / `.duration` | 失败原因 / 耗时(ms) |
   | `task.allSuccess` / `success` / `failed` / `total` | 是否全成功 / 成功数 / 失败数 / 总数 |

2. **扇入**：紧跟一个 `merge` 步骤，在 `when` 里写高级表达式做门控，支持 `&&` / `||` / 比较运算 / `len()` 等函数。表达式为**真**才放行后续步骤，为**假**则报错中止场景。表达式留空默认 `task.allSuccess`。

> 门控表达式示例：
> - `task.allSuccess` —— 必须全部成功
> - `task.A.status == "success" && task.B.status == "success"` —— 指定任务都成功
> - `task.failed == 0 && task.total == 3` —— 三个任务零失败
> - `task.订单查询.output > 0 && len(task) >= 3` —— 产物条件 + 数量

> 注意：需要扇入裁决时，扇出步骤**不要**勾选 `failFast`（否则某任务失败会直接中止，走不到 merge）；让失败进入状态表，由 merge 表达式决定放行还是中止。

完整可运行示例见 `scenarios/fanin-demo.json`（3 个并发拉取 → 全成功门控 → 导出）。

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

`scenarios/demo.json`（可直接跑，连 httpbin）、`scenarios/pagination.json`（真实业务模板）、`scenarios/branch-demo.json`（条件分支）、`scenarios/components-demo.json`（switch + 并行汇合）、`scenarios/fanin-demo.json`（扇出扇入门控）是完整示例。

## 步骤类型

| type | 作用 | 关键字段 |
|---|---|---|
| `request` | 发 HTTP 请求 | `method` `url` `headers` `query` `body` `authType` `authToken` `extract` `assert` |
| `branch` | 条件分支 | `branch`（`conditions`+`logic` 或 `when`；`then`/`else` 子步骤） |
| `switch` | 多分支 | `switch.cases[]`（每个 case 有 `name` `conditions`+`logic` 或 `when` + `steps`）、`switch.default[]` |
| `loop` | 循环子步骤（翻页） | `loop`（见下表）+ `children` |
| `parallel` | 扇出：并行子任务 | `parallel.tasks[]`（`name` `steps` `var`/`value`）、`var` `mode` `separator` `failFast` `statusVar` |
| `merge` | 扇入：门控屏障 | `merge.when`（高级表达式，不成立即中止；留空默认 `task.allSuccess`） |
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
- 逻辑组合：`&&` / `||`（短路求值，引号/括号内的符号不误判），如 `{{task.A.status}} == "success" && {{task.B.status}} == "success"`
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
