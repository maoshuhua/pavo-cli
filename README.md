# PAVO CLI

PAVO 桌面端 Agent 配套 CLI。项目只提供三项业务能力：

1. 邮箱密码登录。
2. 创建 conversation。
3. 以固定 `design` mode 发起并读取生成流。

仓库同时包含 `skills/pavo/`，npm 安装和更新时会把 PAVO Skill 安装到桌面端 Agent 的全局技能目录。

## 安装

需要 Node.js 16 或更高版本。发布后可通过 npm 安装预编译的 macOS、Linux 和 Windows 二进制，并把 PAVO Skill 注册到本机检测到的桌面端 Agent：

```bash
npx @pavo-dev/cli@latest install
pavo --version
```

在 npm Registry 发布前，也可以直接通过 GitHub Release 安装：

```bash
npx -y github:maoshuhua/pavo-cli#v0.1.1 install github:maoshuhua/pavo-cli#v0.1.1
pavo --version
```

安装完成后重启或新建一个桌面端 Agent 会话，使新 Skill 被重新加载。

也可以在源码目录构建：

```bash
go build -o pavo .
```

## 登录

交互式读取密码，不把密码写入 Shell 历史：

```bash
pavo login --email "user@example.com"
```

桌面端 Agent 的非交互场景可以显式传入密码：

```bash
pavo login --email "user@example.com" --password "PASSWORD"
```

也可以通过 `PAVO_PASSWORD` 提供本次登录密码。登录成功后，Access Token 保存在系统用户配置目录的 `pavo/config.json` 中；终端输出和错误日志不会输出 Token。

登录接口：

```http
POST https://api-pixa-test.kiwiar.com/api/v1/user/login
```

## 创建 conversation

```bash
pavo conversation create --prompt "生成美女图"
```

输出：

```json
{"conversation_id":"338562408542949376"}
```

CLI 自动构造以下请求：

```json
{
  "title": "[{\"type\":\"text\",\"content\":\"生成美女图\"}]",
  "folder_id": "",
  "kb_strict": false
}
```

接口：

```http
POST https://api-pixa-test.kiwiar.com/api/v1/chat/conversation
Authorization: Bearer <access_token>
```

## Stream

```bash
pavo stream \
  --conversation-id "338562408542949376" \
  --prompt "生成美女图"
```

请求中的 mode 固定为 `design`：

```json
{
  "conversation_id": "338562408542949376",
  "prompt": "生成美女图",
  "mode": "design"
}
```

CLI 兼容 SSE 和连续 JSON/NDJSON 响应。过程事件写入 stderr，收到 `GenerationSuccess` 后在 stdout 输出一个 JSON 对象：

```json
{
  "conversation_id": "338562408542949376",
  "event_id": "9d4bb9a2-c17c-4315-9111-a8de9c972f72",
  "message_id": "011E447FBFC5B900010A73439E00001DCA",
  "model_code": "agnes-image",
  "task_id": "pavo-15fb4f478964",
  "trace_id": "921a5aa69de756c77a21fb18bea92954",
  "results": [
    {
      "height": 2624,
      "message": "ok",
      "mimetype": "image/jpeg",
      "ratio": "9:16",
      "success": true,
      "thumbnail_url": "https://example.test/thumbnail.webp",
      "url": "https://example.test/image.jpg",
      "width": 1472
    }
  ]
}
```

需要诊断服务端事件时可添加 `--raw`，原始事件仍写入 stderr。

## 桌面端 Agent Skill

技能文件位于 `skills/pavo/SKILL.md`。Skill 严格按照以下顺序调用 CLI：

```text
pavo login
  → pavo conversation create
  → pavo stream
```

Skill 不会调用其他图像或视频服务，也不会增加下载、编辑、历史记录等未提供的 PAVO 能力。

## 配置

| 环境变量 | 说明 |
| --- | --- |
| `PAVO_API_BASE_URL` | API 基础地址，默认 `https://api-pixa-test.kiwiar.com` |
| `PAVO_ACCESS_TOKEN` | 临时覆盖本地保存的 Access Token |
| `PAVO_PASSWORD` | 非交互登录密码 |
| `PAVO_HTTP_TIMEOUT` | HTTP 和生成流超时，默认 `10m` |
| `PAVO_CONFIG_FILE` | 覆盖登录信息文件路径，主要用于测试 |
| `PAVO_CLI_DISABLE_UPDATE_CHECK=1` | 关闭 npm 版本检查 |

## 更新和发布

```bash
pavo update
```

更新命令通过 npm 更新 PAVO CLI，并重新安装 `pavo` Skill。GoReleaser 构建以下平台：

- macOS amd64/arm64
- Linux amd64/arm64
- Windows amd64/arm64

发布前需要确保 `package.json`、Go module、GitHub 仓库和 npm scope 均指向实际的 PAVO 发布位置。

## 开发验证

```bash
npm test
```

该命令执行 JavaScript 安装链路测试、Go 单元测试和 `go vet ./...`。
