# 简介

本项目用于爬取 **Greatfire Analyzer** 检测到的在中国大陆被屏蔽的域名。

## 下载地址

如果不希望自行生成域名列表，可直接下载使用下面域名列表：

**domains.txt**：[https://github.com/Loyalsoldier/cn-blocked-domain/raw/release/domains.txt](https://github.com/Loyalsoldier/cn-blocked-domain/raw/release/domains.txt)

## 项目使用方式

如果希望自行生成域名列表，按照下面步骤操作：

1. 安装 `git` 和 v1.14.0 或更新版本的 `Golang`
2. 下载项目代码，有两种方式：
   1. Git clone：`git clone https://github.com/Loyalsoldier/cn-blocked-domain.git`
   2. 用 Go 下载并安装代码：`go get -v github.com/Loyalsoldier/cn-blocked-domain`
3. 运行代码（分别对应第 2 步中的两种项目代码下载方式）：
   1. `go run *.go`
   2. `${GOPATH:-$(go env GOPATH)}/bin/cn-blocked-domain`

## 使用本项目的项目

- [@Loyalsoldier/v2ray-rules-dat](https://github.com/Loyalsoldier/v2ray-rules-dat)
- [@Loyalsoldier/clash-rules](https://github.com/Loyalsoldier/clash-rules)
- [@Loyalsoldier/surge-rules](https://github.com/Loyalsoldier/surge-rules)
