# MyGo Music · NSGo 复刻

用 [NSGo](https://github.com/oneo-me/nsgo) 1:1 复刻 [MyGo Music](https://github.com/egoist/mygo-music) 的原生桌面音乐播放器。界面、交互与功能以 mygo-music 为准，UI 框架由 MyGo 换成 NSGo。

> 状态：初始化中。音频输出、播放器、本地曲库、歌词与 i18n 内核已移植；窗口与页面正在按上游逐个复刻。

## 运行

需要 Go 1.26 或更高版本，并把 NSGo 与本项目并列放在 `../nsgo`（`go.mod` 通过 `replace` 指向本地 NSGo）。

```sh
go tool nsgo dev        # 开发运行，保存后热重载
go tool nsgo check      # vet 并检查 TSX 类型
go tool nsgo build      # 按 nsgo.yaml 的目标打包
go test ./internal/...  # 单元测试
```

应用名、id、版本和构建目标在 `nsgo.yaml`。

## 结构

| 路径 | 内容 |
| --- | --- |
| `main.go` | 入口，交给 `internal/app` |
| `internal/app` | 窗口、页面、播放条、设置等 UI（复刻中） |
| `internal/player` | 播放队列、流式解码、缓存、淡入淡出、会话恢复 |
| `internal/audio` | 各平台音频输出，以及 MP3、FLAC、Vorbis、AAC、ALAC、WAV 解码 |
| `internal/local` | 本地文件源：扫描、标签、封面、播放列表、喜欢、最近播放 |
| `internal/music` | 播放器与视图共用的音乐模型和源接口 |
| `internal/lyrics` | LRC 解析与逐字歌词 |
| `internal/i18n` | 英文 / 简体中文 |
| `assets` | 应用图标 |

## 复刻约定

- 交互、布局、文案和设置项以 mygo-music 为准；NSGo 缺少的能力先补 NSGo，再在 app 中使用。
- 移植核心包时只改 import path 与平台抽象，不顺手重构播放、解码逻辑。
- 页面复刻完成的标志：与上游同样可操作，同样的空状态和错误提示，并有对应的 `nsgotest` 交互测试。

## 参考

- 上游：<https://github.com/egoist/mygo-music>（AGPL-3.0）
- 框架：<https://github.com/oneo-me/nsgo>（Apache-2.0）

上游为 AGPL-3.0，本项目作为其复刻/衍生作品，对外分发前请确认许可证与署名要求。
