# Alfred Translate

一个用 Go 写的 Alfred 翻译 Workflow。默认关键词是 `tr`。

## 为什么用 Go

Alfred Workflow 可以用 Shell、Python、Node.js、Ruby、Go 等语言实现。这个项目选择 Go，原因是：

- 编译后是单个 macOS 可执行文件，分发时不依赖用户机器上的 Python/Node 运行时。
- 启动速度快，适合 Alfred Script Filter 这种输入时频繁触发的场景。
- 标准库已经覆盖 HTTP、JSON、缓存和 CLI 参数，不需要引入依赖。

Python/Node 更适合快速原型；如果只给自己用且机器环境稳定，也可以选它们。但如果要做成可长期使用、可分享的 Alfred 插件，Go 更稳。

## 功能

- `tr hello`：自动翻译成中文。
- `tr 你好`：自动翻译成英文。
- `Enter`：复制译文。
- `Cmd + Enter`：用美式英语语音朗读原文，Alfred 窗口保持打开，方便重复听。
- 默认使用 Google Translate 的非官方公开接口，个人使用方便，但稳定性不如官方 API。
- 如果设置了 `DEEPL_AUTH_KEY`，`ALFRED_TRANSLATE_PROVIDER=auto` 会优先使用 DeepL。

## 配置

Workflow 变量：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `ALFRED_TRANSLATE_PROVIDER` | `auto` | `auto`、`google` 或 `deepl` |
| `ALFRED_TRANSLATE_SOURCE` | `auto` | 源语言 |
| `ALFRED_TRANSLATE_TARGET` | `auto` | 目标语言；`auto` 时中文翻英文，其他翻中文 |
| `ALFRED_TRANSLATE_VOICE` | `Samantha` | macOS `say` 使用的美式英语语音 |
| `ALFRED_TRANSLATE_RATE` | 空 | 可选语速，传给 `say -r` |
| `DEEPL_AUTH_KEY` | 空 | DeepL API key |
| `DEEPL_API_URL` | `https://api-free.deepl.com/v2/translate` | DeepL endpoint，付费版可改 |

## 本地开发

```bash
go test ./...
go run ./cmd/alfred-translate translate hello
go run ./cmd/alfred-translate filter hello
go run ./cmd/alfred-translate speak hello
```

## 打包

Apple Silicon：

```bash
./scripts/build-workflow.sh
```

Intel Mac：

```bash
GOARCH=amd64 ./scripts/build-workflow.sh
```

生成文件：

```text
dist/AlfredTranslate.alfredworkflow
```

双击 `.alfredworkflow` 文件导入 Alfred。Finder 可能会隐藏扩展名，文件类型显示为 Alfred Workflow 是正常的。
