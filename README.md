# 简介

本项目用于爬取 **Greatfire Analyzer** 检测到的在中国大陆被屏蔽的域名和 IP 地址。

数据来自 [GreatFire JSON API](https://en.greatfire.org/api/tags?status=blocked&limit=200&offset=0)：
读取首页的 `total`，以每页 200 条、`offset` 每次增加 200 的方式获取全部页面，
从 `items` 中提取 `registrable`，不再解析 HTML 页面或读取 `config.yaml`。

## 下载地址

如果不希望自行爬取列表，可直接下载使用下面列表：

- **domains.txt**：[https://github.com/Loyalsoldier/cn-blocked-domain/raw/release/domains.txt](https://github.com/Loyalsoldier/cn-blocked-domain/raw/release/domains.txt)
- **ip.txt**：[https://github.com/Loyalsoldier/cn-blocked-domain/raw/release/ip.txt](https://github.com/Loyalsoldier/cn-blocked-domain/raw/release/ip.txt)
- **deduplicated-domains.txt**：[https://github.com/Loyalsoldier/cn-blocked-domain/raw/release/deduplicated-domains.txt](https://github.com/Loyalsoldier/cn-blocked-domain/raw/release/deduplicated-domains.txt)

## 项目使用方式

如果希望自行爬取列表，按照下面步骤操作：

1. 安装 `git` 和 v1.24.0 或更新版本的 `Go`（只使用标准库，无第三方依赖）
2. 克隆项目代码：`git clone https://github.com/Loyalsoldier/cn-blocked-domain.git`
3. 进入项目根目录：`cd cn-blocked-domain`
4. 运行：`go run ./`

默认最多同时请求 10 个页面；所有请求（包括重试）的启动时间至少间隔 100ms。
先获取首页确定总数，再并发获取其余页面。可通过命令行参数修改并发数和输出目录：

```sh
go run ./ -max-parallel 5 -output-dir ./publish
go run ./ -help
```

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `-max-parallel` | `10` | 最大并行请求数，必须大于 0；设为 `1` 可串行请求 |
| `-output-dir` | `publish` | 输出目录，不存在时自动创建 |

每次请求超时为 30 秒。网络错误、HTTP 429 和 5xx 最多尝试 3 次，
重试前分别等待 1 秒和 2 秒。任一页面请求失败、JSON 格式错误或页面条目数不完整时，
程序返回非零退出码，不写入抓取不完整的结果。按 Ctrl+C 可以取消任务。

### 输出内容

- `domains.txt`：去重并排序后的有效域名，统一转为小写，去除末尾的根域名点。
  只接受 DNS 主机名形式的 ASCII 标签（包括 Punycode 和 `com` 这样的单标签域名）；
  不接受通配符、下划线、URL、端口或未经 Punycode 编码的 Unicode 域名，不做 DNS 存活检测。
- `deduplicated-domains.txt`：因父域名已存在而移除的子域名，去重并排序。
  例如列表包含 `com`、`google.com` 和 `www.google.com` 时，仅保留 `com`，
  另两个域名写入此文件。后缀匹配以完整标签为界，不会因 `example.com` 而移除 `notexample.com`。
- `ip.txt`：有效 IPv4/IPv6 地址按地址排序，以 CIDR 格式输出。
  连续且对齐的地址合并为最小的精确网段，不包含输入中不存在的 IP。
  单个地址输出为 `/32` 或 `/128`；IPv4 映射的 IPv6 地址统一转为 IPv4。
  无效 IP、带区域标识的 IPv6 地址和输入中的 CIDR 字符串会被忽略。

每行一条记录；即使没有结果，也会生成这三个文件。每个文件先写入临时文件再替换，
避免留下半写入的文件。程序会报告跳过的无效值数量。

### 开发与自动运行

- `crawler/client.go`：JSON API 分页、请求限速、重试与并发控制。
- `lists.go`：域名校验、父子域名去重和 IP 网段合并。
- `output.go`：输出文件写入；`main.go`：命令行入口。

```sh
go test -race ./...
go vet ./...
go build ./...
```

测试使用本地 HTTP 测试服务器，不需要访问 GreatFire。
GitHub Actions 在每周日 20:00 UTC、Pull Request 和手动触发时运行测试并抓取数据。
Pull Request 只验证和抓取，不上传产物、不创建 Release，也不向 Git 推送；
定时或手动运行成功后才发布三个列表文件并更新 `release` 分支。

## 使用本项目的项目

- [@Loyalsoldier/v2ray-rules-dat](https://github.com/Loyalsoldier/v2ray-rules-dat)
- [@Loyalsoldier/clash-rules](https://github.com/Loyalsoldier/clash-rules)
- [@Loyalsoldier/surge-rules](https://github.com/Loyalsoldier/surge-rules)
