# 简介

本项目通过 **GreatFire Analyzer** 的 JSON API（`https://en.greatfire.org/api/tags?status=blocked`）获取在中国大陆被屏蔽的域名和 IP 地址，并进行校验、去重与聚合。

## 下载地址

如果不希望自行爬取列表，可直接下载使用下面列表：

- **domains.txt**：[https://github.com/Loyalsoldier/cn-blocked-domain/raw/release/domains.txt](https://github.com/Loyalsoldier/cn-blocked-domain/raw/release/domains.txt)
- **ip.txt**：[https://github.com/Loyalsoldier/cn-blocked-domain/raw/release/ip.txt](https://github.com/Loyalsoldier/cn-blocked-domain/raw/release/ip.txt)
- **deduplicated-domains.txt**：[https://github.com/Loyalsoldier/cn-blocked-domain/raw/release/deduplicated-domains.txt](https://github.com/Loyalsoldier/cn-blocked-domain/raw/release/deduplicated-domains.txt)（因父域名已存在而被移除的子域名）

## 项目使用方式

如果希望自行爬取列表，按照下面步骤操作：

1. 安装 `git` 和 v1.26 或更新版本的 `Golang`
2. 克隆项目代码：`git clone https://github.com/Loyalsoldier/cn-blocked-domain.git`
3. 进入项目根目录：`cd cn-blocked-domain`
4. 运行：`go run ./`

命令行参数：

- `-parallel`：同时请求的最大页数，默认 `10`（每个请求间隔 100ms）
- `-outdir`：输出目录，默认 `publish`

例如：`go run ./ -parallel 5 -outdir ./output`

## 使用本项目的项目

- [@Loyalsoldier/v2ray-rules-dat](https://github.com/Loyalsoldier/v2ray-rules-dat)
- [@Loyalsoldier/clash-rules](https://github.com/Loyalsoldier/clash-rules)
- [@Loyalsoldier/surge-rules](https://github.com/Loyalsoldier/surge-rules)
