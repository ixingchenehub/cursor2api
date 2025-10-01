# Cursor2API

> 将 Cursor AI 能力封装为标准 OpenAI API 格式的高性能代理服务

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Code Quality](https://img.shields.io/badge/golangci--lint-passing-brightgreen)](https://golangci-lint.run/)

---

## ✨ 项目简介

将 Cursor AI 能力封装为标准 OpenAI API 格式的高性能代理服务。

### 核心特性

- ✅ **OpenAI 协议兼容** - 无缝对接 OpenAI SDK
- ✅ **自动 AntiBot 绕过** - 无需手动处理验证
- ✅ **Chrome TLS 指纹模拟** - 绕过指纹识别
- ✅ **流式/非流式双模式** - SSE 流式 + JSON 响应
- ✅ **客户端取消即时感知** - 秒级响应断开

---

## 🚀 快速开始

### 前置依赖

**1. 部署 x-is-human-api 服务**

本项目依赖 [gopkg-dev/x-is-human-api](https://github.com/gopkg-dev/x-is-human-api) 进行 AntiBot 参数解析。

> **为什么需要这个服务?**
>
> Cursor 使用**动态混淆的 JavaScript** 代码生成 AntiBot 参数,必须在 JS 运行时环境中反混淆和 AST 解析。Node.js 是最佳选择:
>
> - 原生 JS 执行环境,无需虚拟机
> - 成熟的 AST 工具链(Babel、@swc/core)
> - 高效完成动态反混淆和参数提取

```bash
# 使用 Docker 快速部署
docker pull ghcr.io/karen/x-is-human-api:latest
docker run -d -p 3000:3000 --name x-is-human-api ghcr.io/karen/x-is-human-api:latest
```

**2. 获取最新 JS_URL**

访问 [https://cursor.com/cn/learn](https://cursor.com/cn/learn),在浏览器开发者工具中找到 JS 文件 URL。

### 一键启动

```bash
# 1. 克隆项目
git clone https://github.com/gopkg-dev/cursor2api.git
cd cursor2api

# 2. 修改 docker-compose.yml 中的 JS_URL
vim docker-compose.yml

# 3. 启动所有服务
docker-compose up -d
```

**服务启动后访问:**

- 🏥 Health: <http://localhost:3001/health>
- 🤖 API: <http://localhost:3001/v1/chat/completions>
- 📋 Models: <http://localhost:3001/v1/models>

---

## 📖 API 使用指南

### 端点总览

| 端点 | 方法 | 说明 |
|------|------|------|
| `/health` | GET | 健康检查 |
| `/v1/models` | GET | 获取可用模型列表 |
| `/v1/chat/completions` | POST | 聊天完成(支持流式) |

### 1. 健康检查

```bash
curl http://localhost:3001/health
```

<details>
<summary>查看响应</summary>

```json
{
  "status": "healthy",
  "timestamp": "2025-10-01T12:00:00Z"
}
```

</details>

### 2. 获取模型列表

```bash
curl http://localhost:3001/v1/models
```

<details>
<summary>查看响应</summary>

```json
{
  "object": "list",
  "data": [
    {
      "id": "anthropic/claude-4.5-sonnet",
      "object": "model",
      "created": 1234567890,
      "owned_by": "anthropic"
    }
  ]
}
```

</details>

### 3. 聊天完成(非流式)

```bash
curl -X POST http://localhost:3001/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "anthropic/claude-4.5-sonnet",
    "messages": [{"role": "user", "content": "你好"}],
    "stream": false
  }'
```

### 4. 聊天完成(流式)

```bash
curl -N -X POST http://localhost:3001/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "anthropic/claude-4.5-sonnet",
    "messages": [{"role": "user", "content": "讲一个笑话"}],
    "stream": true
  }'
```

### 5. 多轮对话

```bash
curl -X POST http://localhost:3001/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "anthropic/claude-4.5-sonnet",
    "messages": [
      {"role": "user", "content": "我叫张三"},
      {"role": "assistant", "content": "你好张三!"},
      {"role": "user", "content": "我叫什么?"}
    ],
    "conversation_id": "user-session-001"
  }'
```

> **注意:** 必须手动传递完整的 `messages` 历史记录

---

## 🏗️ 项目结构

```bash
cursor2api/
├── config/          # 配置加载
├── handler/         # HTTP 处理器
├── models/          # AntiBot 管理器
├── service/         # Cursor API 服务
├── types/           # 类型定义
├── utils/           # 工具函数
├── middleware/      # 中间件
├── ssestream/       # SSE 流处理
├── logger/          # 日志系统
├── main.go          # 入口文件
├── Dockerfile       # Docker 镜像
├── docker-compose.yml
└── Makefile
```

---

## 🛠️ 技术栈

| 技术 | 版本 | 用途 |
|------|------|------|
| **Go** | 1.25+ | 核心语言 |
| **[imroc/req](https://github.com/imroc/req)** | v3.55+ | HTTP 客户端 + TLS 指纹 |
| **[refraction-networking/utls](https://github.com/refraction-networking/utls)** | v1.8+ | TLS 指纹模拟 |
| **[json-iterator](https://github.com/json-iterator/go)** | v1.1+ | 高性能 JSON |

---

## 🤝 贡献指南

欢迎提交 Issue 和 Pull Request!

**贡献流程:**

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交代码 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

**代码规范:**

- 遵循 Go 官方代码风格
- 运行 `make golangci-lint` 确保代码质量
- 添加必要的注释和文档

---

## 🔒 安全说明

### 白帽安全研究声明

**本项目仅用于防御性安全研究和教育目的。**

✅ **允许用途:**

- 安全机制分析与研究
- 反爬虫技术学习
- 构建防御系统
- 学术研究与教育

❌ **禁止用途:**

- 未经授权的访问
- 大规模数据爬取
- 绕过合法访问限制
- 任何非法用途

### 数据安全

- **不记录对话内容** - 服务本身不存储任何数据
- **不上传隐私信息** - 仅处理必要的 API 交互
- **建议使用 HTTPS** - 生产环境启用 TLS 加密

---

## 📜 许可证

MIT License - 详见 [LICENSE](LICENSE) 文件

---

**⚠️ 免责声明:** 本项目属于安全研究工具,使用者需遵守当地法律法规及目标网站服务条款。作者不对滥用行为承担任何责任。
