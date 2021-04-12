# 简介

本项目用于爬取 **Greatfire Analyzer** 检测到的在中国大陆被屏蔽的域名和 IP 地址。

## 下载地址

如果不希望自行爬取列表，可直接下载使用下面列表：

- **domains.txt**：[https://github.com/Loyalsoldier/cn-blocked-domain/raw/release/domains.txt](https://github.com/Loyalsoldier/cn-blocked-domain/raw/release/domains.txt)
- **ip.txt**：[https://github.com/Loyalsoldier/cn-blocked-domain/raw/release/ip.txt](https://github.com/Loyalsoldier/cn-blocked-domain/raw/release/ip.txt)

## 项目使用方式

如果希望自行爬取列表，按照下面步骤操作：

1. 安装 `git` 和 v1.14.0 或更新版本的 `Golang`
2. 克隆项目代码：`git clone https://github.com/Loyalsoldier/cn-blocked-domain.git`
3. 进入项目根目录：`cd cn-blocked-domain`
4. 运行：`go run ./`

更多配置项，可查看项目根目录内的默认配置文件 `config.yaml`。

## 使用本项目的项目

- [@Loyalsoldier/v2ray-rules-dat](https://github.com/Loyalsoldier/v2ray-rules-dat)
- [@Loyalsoldier/clash-rules](https://github.com/Loyalsoldier/clash-rules)
- [@Loyalsoldier/surge-rules](https://github.com/Loyalsoldier/surge-rules)
