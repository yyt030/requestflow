# RequestFlow · request流程自动化工具

一个用 Go + [mygo](https://github.com/egoist/mygo) 原生 UI 构建的桌面工具：把「登录 → 按顺序发请求（HTTP）或收发报文（TCP Socket）→ 提取数据 → 处理 → 导出」配置成可视化步骤，**一键执行**，类似接口自动化测试。单个二进制、无 cgo、原生界面秒开。

既支持 **HTTP/HTTPS** 请求（form-data/JSON/认证等），也支持 **TCP Socket**（连接、发送、按行/定长/标记读取响应，可选 TLS），两者可在同一场景里串联、互相传递变量。

## 特性

- **可视化配置**：左侧 **Postman 风格 Collections 树**（搜索过滤、类型色圆点兼启停开关、上下排序、删除）；**环境变量分层编辑（全局变量 + 多套环境切换，环境变量覆盖同名全局变量）**；右侧标签工作区按步骤类型显示表单。
- **环境变量分层**：场景可定义多套环境（如 dev/test/prod），每套环境一组变量，顶栏/侧栏下拉一键切换；运行时按「环境变量 > 全局变量」优先级解析。数据驱动时行数据优先级最高。旧场景无 `environments` 时行为不变。
- **请求级 Pre/Post 脚本（Postman 风格）**：request 步骤可配 `Pre-request Script`（请求发送前执行，改写变量以影响本次 URL/头/体）与 `Tests`（响应提取/断言后执行，`last` 访问响应），支持 JS / Python 双引擎，与独立 script 步骤共用语法。
- **Postman 风格布局**：顶栏品牌 + 场景 + 操作分组；左侧 Collection 导航树（可搜索）；右侧主工作区用 **步骤编排 / 接口调试 / 运行结果** 三个标签切换；底部状态栏。执行结果采用 **日志 / 结果表 / 变量** 三视图。结果表用原生表格展示提取的数组数据（可选数据源、列排序、刷新）；变量视图列出全部运行时变量。
- **可配置顺序步骤**：`request / tcp / branch / switch / loop / parallel / merge / process / script / assert / export` 十一种步骤自由编排。
- **CSS 选择器提取（HTML 抓取）**：extract 支持 `kind:css`，用 CSS 选择器从 HTML 响应提取，**多元素自动合并为数组**，可取文本 / 内部 HTML / 属性值（`text`/`html`/`attr`），自动解码实体与收敛空白。配合 `loop` 翻页 + `process` 累积 + `export`，无需写 JS 即可抓取网页列表（如豆瓣 Top100 片名/导演/评分）。纯 Go 实现，无 cgo。
- **数组去重与 Base64 编解码**：内置处理新增 `dedup`（数组按整行或按 `field` 字段去重，可选 `stat` 写入重复数）、`base64decode` / `base64encode` **支持数组逐项处理**，解码兼容标准 / URL-safe / 无 padding（可直接解 Bing 跳转链接）。JS（goja）与 Python（Starlark）脚本引擎均注入 Web 标准 `atob` / `btoa`，处理跳转链接无需手写解码函数。
- **变量预览表格**：运行结果「变量」视图按 **变量名 / 类型·数量 / 值** 三列展示，**数组与对象可点击展开逐元素查看**，标量显示完整值（不再截断到 80 字符）。
- **CSS 结构化行提取（`cssRows`）**：与 `kind:css` 并列新增 `kind:cssRows`——只需一个「行选择器」+ 若干「行内子字段」（每个子字段可独立取 `text`/`html`/`attr`），一次提取直接产出 `[{title, url}, ...]` 对象数组，**从机制上杜绝「多个选择器分别提数组再按索引 zip」的错位风险**；采集搜索结果/列表页免写 JS 组装。
- **数据整形内置处理**：新增 `zip`（多数组按索引合成对象数组）、`limit`/`take`/`slice`（取数组区间，如取前 30 条）、`resolve_url`（相对地址按 `baseUrl` 绝对化 + 百度/Bing 跳转链接解码），采集场景绝大多数整形需求无需写脚本。
- **采集健壮性与校验**：提取可配 `expectMin`（条数不足即报错，兜底反爬页/抓空页）；`loop` 支持 `delayMs` 限速、`minRowsVar`+`minRows` 空页检测与 `retryOnEmpty` 自动重试；结果表支持一键「复制列 / 导出 CSV」。
- **循环累积去重**：`loop` 可配 `dedupVar` + `dedupField`，循环结束后对累积数组按字段去重并在日志提示去掉的重复数（`dedupStat` 可写入该计数变量），多源/多页采集去重无需写脚本。
- **工具栏分组 + 场景模板向导**：顶栏按 **文件｜数据｜运行** 语义分组，次要功能（历史/报告/导入 API/关于）收进「⋯ 更多」溢出菜单；导入统一为一个「⇩ 导入」下拉（cURL 粘贴 / Postman·OpenAPI 文件）；点「新建」弹出**模板向导**，内置空白 / 登录取 Token / 翻页采集去重导出 / TCP 长连接帧 / 搜索采集解码去重等模板，一键载入，降低手写 JSON 门槛。
- **内容一键格式化**：Body（raw）与脚本编辑区提供「✨ 格式化」按钮，自动识别并美化 **JSON / XML / HTML / 键值对**（`key: value` / `key=value` 按列对齐）；JSON 缩进两空格、幂等不重复改动，非结构化内容保持原样并给出提示。与调试响应区已有的 JSON 美化体验一致。
- **易用性细节**：字段提示文字提亮、措辞统一；删除环境前二次确认；运行失败时点击状态栏可查看完整错误详情；数值输入统一为整数控件；共享的 key=value 编辑器保证全局变量/环境变量交互一致。
- **TCP Socket 步骤**：填写主机/端口即可建立 TCP 连接（可选 TLS），发送数据支持变量与 `\n`/`\xNN` 转义、四种发送模式（line 补换行 / raw 原样 / hex 十六进制 / **frame 长度头**）；读取支持五种模式（line 读一行 / all 读到关闭或超时 / fixed 定长 / until 读到结束标记 / **frame 长度头**）。响应以文本形式供 jsonpath/regex 提取与断言，可与 HTTP 步骤互相传递变量，实现「HTTP 登录取凭证 → TCP 协议校验」这类跨协议流程。
- **TCP 长连接 + 长度头帧协议**：`tcpConnMode=short`（默认，每步独立连接、发完即关）或 `long`（按 `tcpConnKey` 连接名**跨步骤复用同一 socket**，服务端会话状态得以保持，场景结束自动关闭，也可用 `tcpClose=true` 步骤显式关闭）。`tcpFrameHead` 配置帧头位数（如 8），配合发送/读取模式的 `frame`：发送时自动加 N 位十进制长度头（左补零），如 8 字节报文 `88888888` → `0000000888888888`；读取时先解析长度头再读定长报文。典型用法：步骤1 `LOGIN`、步骤2 `QUERY` 共用同一长连接名，用帧协议收发。
- **条件分支（可视化配置）**：`branch` 步骤按条件走不同分支——**变量 + 操作符（大于/小于/等于/包含/为空…）+ 值** 下拉拼装，支持 and/or 组合与嵌套；条件成立执行 then、否则执行 else，各分支内可放请求/脚本/处理子步骤。也可切换高级模式直接写 `{{amount}} > 0` 表达式。
- **多分支 switch**：`switch` 步骤配置多个 case，**自上而下匹配第一个满足的 case** 后执行其步骤并跳出，都不满足则走 default。case 条件与 branch 一致（and/or + 中文操作符 / 高级表达式），支持用 ↑/↓ 调整优先级。适合按状态码、等级、类型分发到不同处理流程。
- **扇出（并发）**：`parallel` 步骤把多个子任务**并发执行**（各自独立变量沙箱、共享会话，类似 Go 的 goroutine 扇出），全部完成后按 `array / object / table / concat` 方式把各任务产物汇合到一个变量。同时自动登记每个任务的状态表（见下）。任一任务失败默认忽略并继续，可勾选「失败即整体失败」。
- **扇入（门控）**：`merge` 步骤作为屏障，**等待上游并行任务全部完成后**用高级表达式检查各任务状态/产物，条件成立才放行后续步骤，否则**中止场景**（类似断言失败）。表达式支持 `&&` / `||` / 比较 / `len` 等函数，可引用 `task.<任务名>.status/output/error/duration` 及 `task.allSuccess/success/failed/total`。由此实现「必须所有并行任务都成功且满足条件，下一步才能开始」的依赖关系。
- **两种登录态**：Bearer Token 从响应提取到变量后用 `{{token}}` 注入；Cookie 会话由全局 CookieJar 自动跨步骤携带。
- **循环翻页**：`count`（固定次数）、`foreach`（遍历数组）、`until`（翻到空数据自动停）三种模式，页码变量自动递增。
- **内置处理 + 自定义脚本**：常用处理逻辑内置；复杂逻辑用脚本直接处理响应，支持 **JavaScript（goja）与 Python（Starlark 子集）双引擎**，编辑器内一键切换。
- **多格式请求体**：raw（JSON/文本，自动识别 Content-Type）/ form-data（支持文件上传）/ x-www-form-urlencoded / binary（文件路径）/ GraphQL / none 六种 Body 类型，编排步骤与接口调试面板均可配置。
- **完整认证方式**：none / Bearer Token / Basic / **API Key**（请求头或 URL 参数）/ **OAuth 2.0**（client_credentials 与 password 两种授权，自动换 token 并缓存复用）。
- **数据驱动运行**：用 CSV / JSON 数据文件逐行运行场景，每行数据注入为变量（覆盖同名全局变量），适合参数化批量测试；GUI「⊞ 数据驱动」按钮选文件即跑，CLI 用 `-data`。
- **测试报告导出**：汇总每步通过/失败、断言明细、耗时与迭代数据，导出 HTML / JSON 报告；GUI「⤓ 报告」按钮，CLI 用 `-report`。
- **多格式导出**：xlsx / csv / txt / json。
- **从 Chrome 一键导入 cURL**：DevTools 中右键请求 → Copy as cURL，粘贴后一键解析成请求步骤，自动识别 Bearer Token、Cookie、查询参数与请求体，支持 bash / cmd / PowerShell 三种格式。
- **导入 Postman / OpenAPI**：导入面板支持整个 API 定义文件——**Postman Collection v2.1**（文件夹递归展开为步骤，保留变量/认证/Body）与 **OpenAPI 3.x / Swagger 2.0**（paths 转请求步骤，server 变量与路径/查询参数转 `{{变量}}`）。自动识别格式，一次导入多步骤场景。
- **代码生成（Code Snippet）**：接口调试面板响应区「代码」标签，把当前请求（已解析 `{{变量}}`）反向生成 **Python requests / Go net/http / JavaScript fetch / cURL** 四种代码片段，一键复制，便于迁移到脚本或其他工具。
- **请求历史 History**：每次调试请求自动追加记录到 exe 同目录 `requestflow-history.jsonl`（时间/方法/URL/状态/耗时 + 请求快照）；顶栏「🕘 历史」打开列表，可刷新、清空、载入任一条回调试面板重发。
- **接口调试**：主工作区「接口调试」标签（或双击 HTTP 请求步骤）打开，采用 Postman 经典布局：请求行（方法色标 + 方法 + URL + 醒目「发送」按钮）+ 配置子标签 **Params / Headers / Body / Authorization**；单独发送请求即时查看响应，无需运行整个场景。响应区分 **Body / Headers / Cookies / cURL** 四视图并带状态码/耗时/字节数概览，JSON 自动美化。request 步骤编辑器里还有「⚡ 试发送此请求」按钮，用当前步骤配置（含 `{{变量}}` 渲染）直接调试；调试响应可一键反向生成 cURL 并导回场景。
- **类似自动化测试**：每步可配断言（状态码 / JSONPath / 文本包含 / 变量比较），失败即中止，也可设「失败后继续」。
- **无界面模式**：`requestflow -run scenario.json` 便于 CI / 命令行跑批。
- **默认运行日志**：GUI 与无界面模式的日常运行都会自动写日志文件（打开/保存/执行/调试等关键事件），无需手动开启。日志**显式**写入**可执行文件同目录**的 `requestflow.log`（绿色部署，日志与 exe 放一起），不写用户配置目录。可用 `-logdir` 显式指定其它目录。若日志目录不可写会在 stderr 明确报错且界面「ⓘ 版本」对话框提示，不会静默更换位置；该对话框也显示当前日志路径。
- **步骤列表交互**：单击步骤选中并在右侧显示配置详情；**双击** HTTP 请求步骤直接进入调试面板（自动载入该步骤的方法/URL/参数，不自动发送，编辑后点「发送」即可）；选中态采用淡蓝高亮 + 蓝色加粗标题，保证清晰可读。
- **Windows 无黑框**：双击 `requestflow.exe` 启动 GUI 时不再弹出 cmd 控制台窗口；命令行模式（`-version`/`-run`）的输出仍然正常显示。

## 环境要求

- **Go 1.27+**（mygo 要求，工具链会自动下载）
- 桌面环境用于 GUI；无界面模式无需桌面
- **Linux GUI 依赖 GTK3 + WebKitGTK**（mygo 内嵌浏览器引擎渲染界面）：
  - Debian/Ubuntu：`sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0`
  - Fedora：`sudo dnf install gtk3 webkit2gtk4.1`
  - 若缺库，启动会明确报错（不再静默卡住）

```bash
make build                              # 推荐：自动注入版本号构建（见下）
make                                    # 等价于 make build
./requestflow                           # 启动 GUI
./requestflow -software                 # Linux 图形异常时强制软件渲染启动 GUI
./requestflow -run scenarios/demo.json  # 无界面运行
./requestflow -run s.json -data users.csv -report report.html  # 数据驱动 + 报告
./requestflow -version                  # 查看版本号
# 指定输出路径 / CI 手动指定版本：
make build BIN=./bin/rf VERSION=v2.0.0
# 或普通编译（版本号为 dev）：
go build -o requestflow .
```

> **Linux 上运行 GUI 卡死/白屏？** RequestFlow 的界面基于 WebKitGTK，其 GPU（DMA-BUF）渲染在部分 NVIDIA 驱动、虚拟机、远程桌面上会把桌面合成器拖死，表现为"运行后完全没反应、连鼠标都卡住"。程序已默认强制走 CPU 软件渲染规避；若仍异常，可：① 用 `-software` 启动；② 终端执行前加 `GDK_BACKEND=x11 LIBGL_ALWAYS_SOFTWARE=1 WEBKIT_DISABLE_DMABUF_RENDERER=1 ./requestflow`；③ 无图形需求直接用 `-run` 无界面模式。若无 display（如 SSH/容器），程序会明确提示并退出，而非卡住。

## 打包（Windows / Linux 分发）

默认一次打出 **windows/amd64** 与 **linux/amd64** 两个包，各生成可直接分发的 **tar.gz**（含带应用图标的 Windows exe、README、示例场景、使用说明与 SHA256 校验文件）。每次打包会清理 `dist/` 下旧版本产物，只保留本次结果：

```bash
make pack                                  # 默认 windows/amd64 + linux/amd64 两个包
make pack PLATFORMS=linux/arm64            # 只打指定平台（可传多个，空格分隔）
make pack FORMAT=zip                       # 改用 zip 格式
make pack VERSION=v2.0.0 OUT=dist          # CI 中手动指定版本/输出目录
```

产物在 `dist/`（如 `requestflow-<版本>-windows-amd64.tar.gz`、`requestflow-<版本>-linux-amd64.tar.gz`）；Windows 解压后双击 `requestflow.exe`，Linux 在图形桌面执行 `./requestflow`（GUI 依赖见包内使用说明）。命令行支持 `-run` / `-version`。版本号由 git tag + commit 自动注入，无需 cgo。

**应用图标**：Windows exe 内嵌品牌图标（`assets/icon.png`，蓝青渐变 + 双节点汇聚成箭头的流程意象，对应扇出/扇入）。打包时若检测到 `go-winres` 会自动生成 `rsrc_windows_amd64.syso` 并链接进 exe；未安装则降级为无图标构建，不影响功能。安装方式：`go install github.com/tc-hib/go-winres@latest`。可用 `go-winres extract requestflow.exe` 查看内嵌图标资源。

## 版本号

版本号由 git 的 **tag + commit hash** 自动生成，构建时通过 `-ldflags -X` 注入二进制：

| 字段 | 来源 | 示例 |
|---|---|---|
| 版本 | `git describe --tags --always --dirty` | `v1.0.0`、`v1.0.0-3-gabc1234`、`v1.0.0-dirty` |
| 提交哈希 | `git rev-parse HEAD` | `55a27e3d3219...` |
| 构建时间 | `date -u`（RFC3339） | `2026-11-16T14:39:44Z` |

打 tag 后构建即为干净版本号；tag 之后有新提交会自动带 `-N-gHASH`；工作区有未提交改动带 `-dirty`。

```bash
make build                # 输出 ./requestflow，自动注入上述信息
make build BIN=./bin/rf   # 指定输出路径
VERSION=v2.0.0 make build # CI 中手动指定版本
```

**查看方式**：
- CLI：`./requestflow -version` 打印单行完整版本
- GUI：窗口标题栏显示版本号；工具栏右侧的 `ⓘ` 按钮点击后弹出完整信息（版本/哈希/构建时间/Go 版本/平台）

## 设计架构

```
┌─ UI 层 (mygo ui) ── 场景/步骤列表 · 步骤编辑器 · 执行日志 · 数据驱动 ─┐
├─ 控制层 ── 配置加载/保存(JSON) · 执行调度 · 状态显示 ──────┤
├─ 执行引擎 ─ 变量上下文 · HTTP执行(多Body/认证) · 提取/断言 · 循环 · 处理 · JS/Python脚本 ┤
├─ 报告层 ── 步骤/断言追踪 · HTML/JSON 报告导出 ──────────┤
└─ 基础设施 ─ net/http+CookieJar · excelize · gjson · goja · starlark ┘
```

**一个场景 = 一个 JSON 文件**，步骤按顺序执行，步骤间通过变量上下文传递数据。源码分层：`model/`（数据模型）、`engine/`（执行引擎，含单元测试）、`ui/`（界面）。

## 快速开始（GUI）

1. 启动后点工具栏「新建」或「打开」场景，左侧集合根节点可改场景名。
2. 左侧展开「环境变量」，用 `变量名 / 值 / +` 添加变量（步骤里用 `{{变量名}}` 引用）。
3. 左侧「+ 新建步骤」选择步骤类型，添加后用 **↑/↓** 调整顺序；点类型色圆点可启停步骤。
4. 右侧「步骤编排」标签下配置当前步骤：请求 URL 写完整 `http(s)://` 地址（也支持 `{{变量}}`）、方法/Body、提取规则、断言等。
5. 点工具栏「▶ 运行」；切到「运行结果」标签，用 **日志 / 结果表 / 变量** 查看实时过程与数据；可随时「■ 停止」。
6. 「保存」把场景导出为 JSON，可复用或纳入版本管理。

**不想手工配请求？** 点工具栏「⇥ 导入 cURL」：在 Chrome 开发者工具 Network 面板右键目标请求 → **Copy → Copy as cURL**，粘贴到面板里点「解析并添加为步骤」。URL、方法、请求头、Cookie、查询参数、请求体自动填好。

**只想调试单个接口？** 切到主工作区「接口调试」标签（或双击编排树里的 HTTP 请求步骤）：请求行是 Postman 风格（方法色标 + 方法选择 + URL + 「▶ 发送」），下方 **Params / Headers / Body / Authorization** 子标签配置请求，下半区即时查看响应，四个标签页分别是**响应体**（JSON 自动美化）、**响应头**、**Cookie**、**cURL**（当前请求反向生成的命令，可一键导回场景）。URL、Header、Body 都支持 `{{变量}}`，自动套用场景环境变量。如果在编排模式下想快速验证某个 request 步骤，可直接在步骤编辑器点「⚡ 试发送此请求」，会自动载入该步骤配置并发起调试。

### 请求体与认证（Postman 风格）

request 步骤与接口调试面板都支持完整的 Body 类型，在「Body 类型」下拉切换：

| 类型 | 说明 |
|---|---|
| `none` | 无请求体 |
| `raw` | 原始文本（JSON / XML / 任意文本），JSON 自动设 `Content-Type: application/json`，支持 `{{变量}}` |
| `form-data` |  multipart 表单，逐行添加字段；字段类型选 `file` 时填本地文件路径即可上传文件 |
| `urlencoded` | `application/x-www-form-urlencoded` 表单 |
| `binary` | 填写文件路径，按 `application/octet-stream` 发送二进制 |
| `graphql` | 填 Query 与 Variables（JSON），自动包装成 `{"query":...,"variables":...}` |

认证在「认证方式」下拉选择：`none` / `bearer` / `basic` / `apikey` / `oauth2`。

- **API Key**：填参数名与值，并选放在请求头（`header`，默认）还是 URL 查询参数（`query`）。
- **OAuth 2.0**：填 Token URL、Client ID / Secret、Scope 与授权类型（`client_credentials` 默认，或 `password` 时另填用户名密码）。执行时自动 POST 换取 `access_token` 并以 Bearer 注入；**同一凭据会缓存复用**、临近过期自动刷新，无需每步重复获取。

### 数据驱动运行

把测试数据放进 CSV 或 JSON 文件，一次运行 N 行、每行注入为变量：

```csv
keyword,expected
apple,200
banana,200
```

```json
[ {"keyword":"apple","expected":200}, {"keyword":"banana","expected":200} ]
```

- GUI：点工具栏「⊞ 数据驱动」选择文件，立即逐行运行，日志与「运行结果」标签按迭代展示（数据文件名/变量名与场景全局变量同名时，**数据行优先**）。
- CLI：`requestflow -run scene.json -data users.csv`。JSON 顶层支持对象数组或 `{"data":[...]}` 包裹；非字符串值自动转字符串。
- 每次迭代是**独立引擎**（独立 CookieJar 与变量上下文），互不影响；某一行失败不阻断后续行，最后汇总通过/失败数。

### 测试报告导出

执行完成后点工具栏「⤓ 报告」（或 CLI 加 `-report report.html`）。报告汇总：总迭代数 / 通过 / 失败 / 通过率 / 总耗时，每一轮的**步骤记录**（类型、状态、耗时、错误）、**断言明细**（来源、期望、实际、结果）与结束时变量。扩展名 `.json` 导出结构化 JSON，其它导出可直接浏览器打开的 HTML。CLI 数据驱动与报告可组合：

```bash
requestflow -run scene.json -data users.csv -report result.html
```

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
  "baseUrl": "{{baseUrl}}",
  "variables": { "pageSize": "20" },
  "environments": {
    "test": { "baseUrl": "https://test-api.example.com" },
    "prod": { "baseUrl": "https://api.example.com" }
  },
  "activeEnv": "prod",
  "steps": [
    {
      "id": "login", "name": "登录", "type": "request", "enabled": true,
      "method": "POST", "url": "/api/login",
      "scriptLang": "js",
      "preScript": "vars.ts = Date.now();",
      "postScript": "vars.loginOk = last.status == 200;",
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
| `tcp` | 建立 TCP Socket 连接、收发报文 | `tcpHost` `tcpPort` `tcpSend` `tcpSendMode` `tcpReadMode` `tcpReadNum` `tcpUntil` `tcpTimeout` `tcpKeepAlive` `tcpTls` `tcpConnMode` `tcpConnKey` `tcpClose` `tcpFrameType` `tcpFrameHead` `tcpFrameEndian` `extract` `assert` |
| `branch` | 条件分支 | `branch`（`conditions`+`logic` 或 `when`；`then`/`else` 子步骤） |
| `switch` | 多分支 | `switch.cases[]`（每个 case 有 `name` `conditions`+`logic` 或 `when` + `steps`）、`switch.default[]` |
| `loop` | 循环子步骤（翻页） | `loop`（见下表）+ `children` |
| `parallel` | 扇出：并行子任务 | `parallel.tasks[]`（`name` `steps` `var`/`value`）、`var` `mode` `separator` `failFast` `statusVar` |
| `merge` | 扇入：门控屏障 | `merge.when`（高级表达式，不成立即中止；留空默认 `task.allSuccess`） |
| `process` | 内置数据处理 | `op` + `args` |
| `script` | 自定义脚本（JS/Python） | `scriptLang`、`code` |
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

**累积去重**：三种模式都可在循环结束后对累积数组去重——`dedupVar`（数组变量名，如 `allRows`）、`dedupField`（可选，对象数组按此字段如 `link` 去重，留空按整行）、`dedupStat`（可选，写入去掉的重复数变量名）。去重结果写回同名变量，并在日志提示 `N 行 → M 行，去掉 K 条重复`。例：

```json
{"type":"loop","loop":{"mode":"until","breakWhen":"{{len(items)}} == 0","dedupVar":"allRows","dedupField":"link","dedupStat":"dupCount"},"children":[...]}
```

### TCP Socket 步骤（tcp）

| 字段 | 说明 |
|---|---|
| `tcpHost` / `tcpPort` | 目标主机与端口，支持 `{{变量}}` |
| `tcpSend` | 发送数据，支持 `{{变量}}` 与转义 `\n` `\r\n` `\t` `\xNN` |
| `tcpSendMode` | `line`（末尾补 `\n`，默认）/ `raw`（原样）/ `hex`（按十六进制解码）/ `frame`（按所选帧头类型自动加长度头） |
| `tcpReadMode` | `line`（读一行）/ `all`（读到关闭或超时）/ `fixed`（定长字节）/ `until`（读到结束标记）/ `frame`（按所选帧头类型读长度头 + 定长报文） |
| `tcpReadNum` | `fixed` 模式的字节数（默认 1024） |
| `tcpUntil` | `until` 模式的结束标记，如 `\n` 或 `>` |
| `tcpTimeout` | 连接与读取超时毫秒，默认 5000 |
| `tcpKeepAlive` | 底层 TCP 保活探测周期（秒）；`0`/空=关闭（默认），如 `30` 表示每 30 秒探测一次 |
| `tcpTls` | 是否启用 TLS（默认 false） |
| `tcpConnMode` | `short`（默认，短连接，每步独立建立、发完即关）/ `long`（长连接，按 `tcpConnKey` 跨步骤复用同一 socket） |
| `tcpConnKey` | 长连接共享键，同名步骤复用同一 socket；留空为 `_default` |
| `tcpClose` | `true` 时关闭 `tcpConnKey` 指定的长连接（不再发数据） |
| `tcpFrameType` | 帧头类型：`ascii`（位数自定义的十进制字符串头）/ `bin1`（1 字节二进制，≤255）/ `bin2`（2 字节二进制，≤64K-1）/ 空（不启用）。仅 `frame` 模式使用，界面可下拉选择 |
| `tcpFrameHead` | `ascii` 头的位数，如 `3` / `5` / `8`；留空按 8 处理，`bin1`/`bin2` 忽略此字段。旧场景仅填此项时自动按 `ascii` 兼容 |
| `tcpFrameEndian` | `bin2` 头的字节序：`big`（默认，网络序）/ `little` |

**三种长度头帧协议**（界面「帧头类型」下拉可选，`tcpSendMode=frame` / `tcpReadMode=frame` 时生效）：

- **位数自定义的 ASCII 字符串头**：`tcpFrameType=ascii`（或旧写法只填 `tcpFrameHead=8`）。长度写成十进制数字字面，位数由 `tcpFrameHead` 决定（如 3/5/8，默认 8），不足左侧补零。例如 8 位头 + 8 字节报文 → `0000000888888888`；3 位头 + 4 字节报文 → `004PING`。读取时先读 N 位头解析长度，再读定长报文。
- **1 字节二进制头**：`tcpFrameType=bin1`。长度以 1 个无符号字节写出，最大 255 字节报文，超出会明确报错。例如长度 10 → 字节 `0a`。
- **2 字节二进制头**：`tcpFrameType=bin2`。长度以 2 字节二进制写出，最大 65535 字节（64K-1），超出会明确报错。默认大端（网络序 `big`），可设 `tcpFrameEndian=little` 切小端。例如长度 8 → 大端 `00 08`、小端 `08 00`。

**保活（keepalive）**：`tcpKeepAlive` 填大于 0 的秒数时，底层 socket 开启 TCP keepalive 并按该周期探测；`0`/空时显式关闭（覆盖 Go 默认开启的行为）。对 TLS 连接作用于其底层 TCP socket。长连接空闲保活、短连接均可用。

JSON 示例（长连接 + 2 字节二进制帧头，两步复用同一 socket）：

```json
{
  "name": "二进制帧长连接示例",
  "steps": [
    {
      "id": "login", "name": "登录帧", "type": "tcp", "enabled": true,
      "tcpHost": "127.0.0.1", "tcpPort": "9000",
      "tcpConnMode": "long", "tcpConnKey": "session1",
      "tcpSend": "LOGIN zhangsan", "tcpSendMode": "frame", "tcpReadMode": "frame",
      "tcpFrameType": "bin2", "tcpFrameEndian": "big", "tcpTimeout": "3000"
    },
    {
      "id": "query", "name": "查询帧", "type": "tcp", "enabled": true,
      "tcpHost": "127.0.0.1", "tcpPort": "9000",
      "tcpConnMode": "long", "tcpConnKey": "session1",
      "tcpSend": "QUERY.profile", "tcpSendMode": "frame", "tcpReadMode": "frame",
      "tcpFrameType": "bin2", "tcpFrameEndian": "big", "tcpTimeout": "3000"
    }
  ]
}
```

**长连接跨步骤**：多个 TCP 步骤设 `tcpConnMode=long` 且 `tcpConnKey` 相同，即可在同一 socket 上依次收发，服务端会话状态（如登录态）得以保持；场景结束自动关闭所有长连接，也可加一个 `tcpClose=true` 的步骤显式关闭。长短连接在界面「连接模式」下拉选择。

响应以文本写入 `body`，`status=200` 表示连接并读取成功；因此 `extract`/`assert` 的 `body`、`jsonpath`、`regex`、`status` 用法与 HTTP 完全一致。典型跨协议场景：HTTP 登录提取 `session`/`tag` → TCP 步骤发送 `CHECK {{session}} {{tag}}` → 从 TCP 响应提取变量并断言。

### 提取器（extract）

`source`：`body`（默认）/ `header` / `cookie` / `status`；`kind`：`jsonpath` / `regex` / `whole` / `css`。
例如 `{"name":"token","kind":"jsonpath","path":"data.token"}`、`{"name":"id","kind":"regex","path":"id=(\\d+)","group":1}`。

**CSS 选择器提取（HTML 页面）**：`kind:css` 用 CSS 选择器从 HTML 响应提取，**多元素自动合并为数组**，基于 `x/net/html`（纯 Go、无 cgo），自动解码 HTML 实体（`&nbsp;` / `&amp;`）并收敛空白。

| 字段 | 说明 |
|---|---|
| `path` | CSS 选择器，如 `div.item .title` / `a.link`，支持 `:first-child` 等伪类 |
| `cssField` | `text`（默认，元素文本）/ `html`（内部 HTML）/ `attr`（属性值） |
| `cssAttr` | `cssField=attr` 时的属性名，如 `href` / `src` / `content` |

示例（抓取豆瓣 Top250 每页的片名、评分、海报链接）：

```json
"extract": [
  { "name": "names",   "kind": "css", "path": "div.item .hd .title:first-child", "cssField": "text" },
  { "name": "ratings", "kind": "css", "path": "div.item .rating_num",            "cssField": "text" },
  { "name": "posters", "kind": "css", "path": "div.item .pic img",               "cssField": "attr", "cssAttr": "src" }
]
```

无匹配元素返回 `null`（可用 `default` 兜底）；`kind:css` 与 `jsonpath`/`regex` 可混用。完整案例见 `verify/douban_top100_css_scenario.json`（loop 翻 4 页 → CSS 提取 → JS 组装 → 导出 100 条，与 JS 正则版逐条比对 0 差异）。

**结构化行提取（`kind:cssRows`）**：当需要从同一行的多个字段配对成对象时（如搜索结果的标题+链接），用 `cssRows` 一次提取，避免两个 `css` 选择器分别提数组再 zip 的错位风险。

| 字段 | 说明 |
|---|---|
| `path` | 行选择器，如 `div.c-container[tpl="www_index"]` |
| `cssFields` | 行内子字段数组，每项含 `name`（输出字段名）、`path`（行内选择器，空=行元素自身）、`cssField`（text/html/attr）、`cssAttr`、`resolve`（是否 URL 绝对化+跳转解码） |
| `expectMin` | 最少行数校验，不足即报错（兜底反爬页/抓空页） |

```json
{ "name": "rows", "kind": "cssrows", "path": "div.c-container[tpl=\"www_index\"]", "expectMin": 1,
  "cssFields": [
    { "name": "title", "path": "h3", "cssField": "text" },
    { "name": "url", "cssField": "attr", "cssAttr": "mu", "resolve": true }
  ] }
```

`resolve` / `resolve_url` 会：把相对地址按场景 `baseUrl`（或页面 `<base href>`）绝对化，并解码百度 `link?url=`、Bing `ck/a?u=a1...` 等跳转链接为真实目标地址。案例见 `verify/baidu_golang_v2_scenario.json`（百度 golang 前 30 条，与早期 JS zip 版 `verify/baidu_golang_top30.json` 逐条一致，0 差异）。

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

> **想把返回报文 JSON 里的 list 解析出来？** 数据处理步骤只认变量、不直接读响应报文，标准做法是两步：
> ① 在 HTTP 请求步骤的「提取」标签用 `jsonpath` 把数组取成变量，如路径填 `data.books`、变量名填 `books`；
> ② 在数据处理步骤直接用变量 `books`（filter/sort/pick/limit 等都可），或用 `json_to_table`（`from=books to=table`）把它规范成表格。
> 界面「提取」「处理」标签的**默认例子**已内置这条链路，一键套用后预览会即时显示结果。

### 案例：复杂 JSON 报文里定位具体记录（header + body + 20 条 list）

响应形如 `{"header":{...},"body":{"list":[{...}, ...20 条]}}` 时，在「提取」标签用 `jsonpath` 按层级写路径即可定位任意记录。**这 7 条已固化成界面「复杂JSON例」一键按钮**，点「载入20条报文」即可实时看结果：

| 想要的数据 | Path | 结果 |
|---|---|---|
| 头部字段 | `header.code` | `0` |
| 记录总数 | `body.list.#` | `20` |
| 第 3 条（下标从 0）的嵌套字段 | `body.list.2.detail.author` | `"王五"` |
| 按 id 定位整条记录 | `body.list.#(id==3)#` | 该条完整对象 |
| 按条件取多条的某字段 | `body.list.#(score>90)#.title` | `["乙","丁","辛","寅","巳","酉"]` |
| 嵌套字段做条件 | `body.list.#(detail.author=="王五").title` | `"丙"` |
| 整页 list | `body.list` | 20 条，供下游 filter/sort/pick |

> 关键坑：`#(条件).字段` 只返回**第一条**匹配；要拿全部匹配必须写成 `#(条件)#.字段`。
> gjson 的 `&&`/`||` 多条件表现不可靠，复杂筛选请用两步链路：先提取 `body.list` 成变量 `rows`，再用「处理」标签的 filter/sort/pick（已固化「复杂JSON例」按钮）。数组元素做条件用下标，如 `body.list.#(tags.0=="a").title`。

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
| `dedup` | 数组去重（保序），`field` 为空按整行去重 | `from` `field` `to=deduped` `stat`（可选，写入重复数） |
| `zip` | 多数组按索引合成对象数组（以最长为准，短数组补空） | `from=titles,links` `fields=title,url` `to=zipped` |
| `limit` `take` `slice` | 取数组区间（取前 N 条 / `[start,end)`） | `from` `start`（默认0）`count` 或 `end` `to=limited` |
| `resolve_url` | URL 绝对化 + 跳转链接解码（百度/Bing） | `from` `base`（可选）`to` |
| `join` | 拼接字符串 | `values=a,b,c` `sep` `to` |
| `base64encode` `base64decode` | Base64 编解码，**支持数组逐项处理**；解码兼容标准/URL-safe/无 padding | `from` `to` |
| `urlencode` `urldecode` | URL 编解码 | `from` `to` |

> `dedup` 示例：`{"op":"dedup","args":{"from":"links","field":"link","to":"uniq","stat":"dupCount"}}`
> `zip` 示例：`{"op":"zip","args":{"from":"titles,links","fields":"title,url","to":"rows"}}` — 免写 JS 的数组配对。
> `limit` 示例：`{"op":"limit","args":{"from":"rows","count":"30","to":"top30"}}` — 免写 `slice` 脚本。
> `base64decode` 对数组变量逐项解码返回数组；对标量返回字符串。Bing 跳转链接可用 `atob`、`resolve_url` 或此 op 解码。

## 自定义脚本（script）

脚本步骤支持 **JavaScript 与 Python 双引擎**，在编辑器的「语言」下拉切换；默认 `js`，已有场景无需改动。两个引擎注入同样的对象：

- `vars`：全部运行时变量，可读写（改动自动写回）
- `last`：最近一次响应 `{ status, body, json, headers }`
- `log(...)`：输出到控制台
- `JSON.stringify` / `JSON.parse`：JSON 编解码
- `atob(s)` / `btoa(s)`：Base64 解码/编码（Web 标准，兼容 URL-safe 与无 padding）；处理网站跳转链接无需手写解码函数

### JavaScript（goja，纯 Go 引擎）

```js
var data = last.json.data.list;
vars.rows = data.filter(function(x){ return x.amount > 100; });
vars.total = data.length;
log("共 " + vars.total + " 条");
```

### Python（Starlark 子集，纯 Go 无 cgo）

用 `scriptLang: "python"` 启用。语法是 Python 子集（Bazel 同款），支持变量、`if/else`、函数、列表/字典、列表推导、`for`（需写在函数内）、常用内置（`len/range/str/int/sum/any/all/abs` 等）与 `",".join(...)`。变量用 `vars["k"]` 读写。

```python
vars["token"] = last["json"]["data"]["token"]
names = ["a", "b", "c"]
vars["joined"] = ",".join([n.upper() for n in names])
log("joined=", vars["joined"])
```

> **差异提示**：Starlark 不是完整 CPython——顶层不能直接写 `for`（放进函数再调用），不支持 `while` / `try` / `class`，标准库有限。需要完整 Python 语法请改用 JS 引擎或内置处理步骤。选 Python 是为了保持单二进制、无 cgo（完整 CPython 需链接 libpython，破坏 Windows 绿色打包）。

## 测试

```bash
go test ./...                          # 全部包（engine / ui / version / logger）
go test ./engine -run TestRegression -v  # 只跑端到端回归案例
go test -race ./...                    # 竞态检测
```

单元测试已覆盖：登录提取 Token + Bearer 注入、Cookie 会话、循环翻页终止、脚本写变量 + 导出、表达式求值、表格构建、变量增删改、版本信息格式化，以及 **Python 引擎**（变量读写、函数/推导、`any/all/abs`、JSON、顶层 for 报错、JS 兼容回退）。

**端到端回归案例**（`engine/regression_test.go` + `engine/testdata/`）：用 `httptest` 起本地服务（无需外网、不依赖固定端口），从 JSON 加载完整场景并运行，覆盖「登录提取 → 携带 Token 鉴权请求 → 状态/JSONPath 断言 → 条件分支 → 循环翻页 → JS 脚本 → 导出」，校验关键变量与导出文件内容；另含分支 else 路径，以及 Python 脚本场景（`regression-python-scenario.json`：脚本写回变量被后续步骤渲染与断言使用）。场景中的 `{{BASE}}`/`{{OUT}}` 占位符在测试时替换为实际服务地址与临时输出路径，与 GUI/CLI 打开场景的加载路径一致。新增功能后跑一遍 `go test ./...` 即可回归。

## 目录

```
main.go            入口（GUI / -run 无界面 / -version），启动时初始化日志
console_windows.go Windows 平台：GUI 模式隐藏控制台黑框（build tag）
console_other.go   非 Windows 平台的空实现（build tag）
model/model.go     数据模型
engine/            执行引擎 + cURL 解析器 + 接口调试 + JS/Python 双脚本引擎 + 数据驱动(dataset) + 执行追踪(trace) + 单元测试 + 端到端回归测试
engine/testdata/   回归场景 JSON（占位符 BASE/OUT 运行时替换）
logger/            默认文件日志（GUI/无界面共用）+ 单元测试
report/            测试报告：步骤/断言汇总 → HTML/JSON 导出 + 单元测试
ui/app.go          mygo 原生界面（步骤选中/双击调试/日志落盘/数据驱动/报告导出）
version/version.go 版本信息（构建期注入）+ 单元测试
Makefile           构建/检查/打包统一入口：build/check/test 等；pack 内联实现双平台打包（tar.gz + 图标 + sha256，自动清理旧产物）
scenarios/         示例场景
```
