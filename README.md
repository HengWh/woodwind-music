# nsgo-music

基于 [NSGo](https://github.com/nsgo-dev/nsgo) 的原生桌面音乐播放器，由 [MyGo Music](https://github.com/egoist/mygo-music) 复刻与移植而来。

NSGo 为本项目提供窗口、界面渲染和交互框架；MyGo Music 是界面设计、交互、文案及播放、曲库、歌词和队列等核心实现的来源。本项目将 UI 框架由 MyGo 迁移到 NSGo，并完成相应的平台适配与体验调整。

应用显示名称统一为 `nsgo-music`。感谢两个项目的作者与贡献者，详细说明见[致谢](#致谢)与[许可证](#许可证)。

> 当前专注于本地音乐播放和已有 UI 的打磨，暂不扩展插件、播客等功能。主要播放流程已接通，界面仍在对照上游完善；具体范围见[已实现](#已实现)和[与上游的差距](#与上游的差距)。

## 当前状态与进展

- 设置：对齐侧栏入口、分组卡片、下拉菜单和外观切换控件；修复点击设置后窗口被侧栏刷新关闭的问题，支持重复打开时复用窗口、关闭后重新打开。
- 歌词：调整字体、行距、翻译和当前行样式，加入逐字高亮、缩放与淡入淡出；手动浏览后约 4 秒恢复跟随，点击歌词跳转到对应位置。
- 布局：修复长歌词把播放页撑高的问题，调整窄窗口下的封面、控制区和底部播放条，避免控件被挤出窗口。
- 外观：播放页背景随封面取色，播放控制使用上游的实心图标；深浅主题切换会更新整个主窗口。

当前 UI 验收重点是主要页面布局与交互一致，以及深浅主题、长文本、无歌词和窄窗口下的稳定显示。

## 界面预览

![首页](docs/home.png)

<details>
<summary>查看专辑、正在播放与设置界面</summary>

![专辑](docs/album.png)

![正在播放](docs/now-playing.png)

![设置](docs/settings.png)

</details>

更多布局示例见[窄窗口歌词预览](docs/lyrics-narrow.png)。

## 开发与运行

从源码开发需要 Go 1.26 或更高版本。当前打包目标为 **macOS Apple Silicon（arm64）**；打包配置使用 Apple `.icon` 图标，需要完整 Xcode 中的 `actool`。

### 环境准备

NSGo 与本项目应放在同一级目录：

```text
workspace/
├── nsgo/
└── nsgo-music/
```

`go.mod` 使用 `github.com/nsgo-dev/nsgo`，并通过 `replace` 指向 `../nsgo`。首次准备目录时可以运行：

```sh
git clone https://github.com/nsgo-dev/nsgo.git
git clone https://github.com/HengWh/nsgo-music.git
cd nsgo-music
```

### 常用命令

以下命令在项目根目录执行：

```sh
go tool nsgo dev        # 开发运行，保存后热重载
go tool nsgo test       # 默认无窗口运行全部测试
go tool nsgo check      # vet 并检查目录
go tool nsgo build      # 按 nsgo.yaml 的目标打包
```

### 构建产物

应用名、id、版本和构建目标在 `nsgo.yaml`，当前版本为 `0.1.22`。构建产物包括：

```text
dist/nsgo-music-0.1.22-macos_arm64/nsgo-music.app
dist/releases/nsgo-music-0.1.22-macos-arm64.dmg
```

### Xcode 设置

若 `dev` 报 `the icon needs Xcode's actool`，是 `xcode-select` 指向了 CommandLineTools。选择 Xcode 后即可：

```sh
sudo xcode-select -s /Applications/Xcode.app/Contents/Developer
```

不修改全局设置时，也可以只给这条命令指定 Xcode：

```sh
DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer go tool nsgo dev
```

同样的 `DEVELOPER_DIR` 前缀也适用于 `build` 和真实窗口截图命令。

## 测试与验证

**日常回归优先使用 NSGo 的无窗口测试**，无需启动真实应用窗口：

```sh
go tool nsgo test
go tool nsgo check
```

设置窗口的回归测试使用桌面多窗口模型，覆盖实际按钮事件、侧栏刷新、重复打开、隐藏后唤回和关闭后重开：

```sh
go tool nsgo test -run TestSettingsWindowSurvivesSidebarUpdates
```

其他 UI 测试覆盖设置菜单的鼠标与键盘操作、主题切换、歌词逐字高亮、手动浏览后恢复跟随、点击跳转，以及主窗口和播放条的尺寸变化。

最近一次验证（2026-11-12）：无窗口测试 **123 项通过、0 项失败、2 项跳过**；跳过项为需手动启用的截图测试。静态检查、专项真实窗口验证和 macOS arm64 打包也已通过。

### 真实窗口截图

需要目视对照上游时再运行截图测试。`TestShots` 覆盖首页、专辑、歌手、播放页、舞台、封面墙、队列、搜索和歌单等页面：

```sh
NSGO_SHOTS=1 go tool nsgo test -target macos -run TestShots
open .nsgo/test/screenshots/macos
```

设置与歌词的专项截图使用固定播放位置，覆盖深浅主题、下拉菜单、长文本、无歌词与窄窗口：

```sh
(cd internal/app && NSGO_UI_SHOTS=1 go tool nsgo test -target macos -run TestUIShots)
open internal/app/.nsgo/test/screenshots/macos
```

测试使用独立的临时应用数据。仓库中保留了[设置预览](docs/settings.png)、[播放页预览](docs/now-playing.png)和[窄窗口歌词预览](docs/lyrics-narrow.png)。

## 功能与移植范围

### 已实现

- 本地文件：MP3、FLAC、Ogg Vorbis、WAV、M4A（ALAC/FLAC/AAC）与 AAC，含标签、内嵌封面、同目录封面、`.lrc` 歌词
- 首页、最近播放、专辑/歌手/歌曲页、搜索结果（歌曲/专辑/歌手/歌单）
- 歌单：新建、重命名、删除、添加/移除歌曲、上下移动
- 喜欢的音乐
- 播放器：队列、随机、单曲/列表循环、进度拖动、音量、暂停淡出、响度归一化、磁盘缓存、会话恢复
- Now Playing：封面、歌词、舞台（Flow / Steps / Tilt 与 Sparks）、接下来、封面墙；歌词逐字高亮、当前行缩放与淡入淡出、点击跳转、手动浏览后恢复跟随，背景随封面取色
- 首页主推卡片、专辑/歌手/歌单卡片、歌手页歌曲/专辑切换、查看全部、搜索历史下拉与设置页侧栏/搜索
- 侧栏、返回/前进、搜索历史、空状态
- 设置窗口：侧栏分页与搜索（通用、播放、资料库、缓存、实验性功能）、跟随系统/English/简体中文、外观、应用图标（程序坞与菜单栏）、音质、音量均衡、重新扫描与添加/移除目录、缓存上限与清除
- macOS：隐藏标题栏、自绘顶部区域、菜单栏图标、媒体键与系统“正在播放”
- 英文与简体中文，跟随系统语言

普通 LRC 的逐字进度沿用上游逻辑，将每行时长分配给行内文字。音质设置会传给支持音质选择的音乐源。

### 与上游的差距

这些上游能力还没移植：

- 插件系统（JS 宿主、安装/更新、插件源、登录窗口、评论、播客、电台）
- 播客字幕与本地语音识别（sherpa-onnx）
- 雾效、进度条烟雾、封面飞入与封面墙的自由拖动等细节动画
- 自动更新与勾选通知等发布相关功能

刘海迷你播放器明确不在移植范围内。实验性功能面板中的雾效、进度条烟雾和封面飞入设置仍保留，相关效果尚未完成移植。

移植约定：应用名称使用 `nsgo-music`，其余交互、布局、文案和设置项以 mygo-music 为准；NSGo 缺少的能力先补 NSGo，再在 app 中使用。核心包只改 import path 与平台抽象，不顺手重构播放与解码逻辑。

## 项目结构

| 路径 | 内容 |
| --- | --- |
| `main.go` | 入口，交给 `internal/app` |
| `internal/app` | 窗口、页面、播放条、Now Playing、设置与菜单 |
| `internal/player` | 播放队列、流式解码、缓存、淡入淡出、会话恢复 |
| `internal/audio` | 各平台音频输出，以及 MP3、FLAC、Vorbis、AAC、ALAC、WAV 解码 |
| `internal/local` | 本地文件源：扫描、标签、封面、播放列表、喜欢、最近播放 |
| `internal/music` | 播放器与视图共用的音乐模型和源接口 |
| `internal/lyrics` | LRC 解析与逐字歌词 |
| `internal/nowplaying` | macOS 的 MPNowPlayingInfoCenter 与 MPRemoteCommandCenter |
| `internal/i18n` | 英文 / 简体中文 |
| `resources`、`icon` | 应用图标与生成工具 |
| `docs` | 供目视核对的界面截图 |

## 致谢

`nsgo-music` 的实现离不开以下两个开源项目：

- **[NSGo](https://github.com/nsgo-dev/nsgo)**：为本项目提供原生桌面应用框架，承载窗口、界面渲染和交互。感谢 NSGo 作者与贡献者的开发和维护，让使用 Go 构建原生应用成为可能。
- **[MyGo Music](https://github.com/egoist/mygo-music)**：是本项目复刻与移植的基础。界面设计、交互、文案，以及音频播放、曲库、歌词和队列等核心实现均源自该项目。感谢其作者与贡献者对产品细节的打磨，以及开放源码供大家学习和继续构建。

本项目的工作主要是将 MyGo Music 的界面迁移到 NSGo，并完成相应的平台适配与体验调整。感谢两个项目分享设计与实现，也欢迎大家了解、使用和支持原项目。

## 许可证

上游 mygo-music 为 AGPL-3.0，本项目作为其复刻/衍生作品同样以 AGPL-3.0 发布，见 [LICENSE](LICENSE) 与 [NOTICE](NOTICE)。NSGo 为 Apache-2.0；`internal/audio` 来自 MIT 许可的 MyGo，见 [LICENSES/MyGo-MIT.txt](LICENSES/MyGo-MIT.txt)。
