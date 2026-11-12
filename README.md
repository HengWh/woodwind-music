# nsgo-music · NSGo 原生音乐播放器

用 [NSGo](https://github.com/nsgo-dev/nsgo) 复刻 [MyGo Music](https://github.com/egoist/mygo-music) 的原生桌面音乐播放器：界面、交互和文案以 mygo-music 为准，UI 框架由 MyGo 换成 NSGo，音频、曲库、歌词与播放队列沿用上游实现。

> 状态：可持续运行的本地音乐播放器，界面仍在按上游继续复刻。播放、曲库、歌词、队列、设置、中英文、菜单栏与媒体键已接通；插件、播客与评论尚未移植，刘海迷你播放器按要求不移植，详见[与上游的差距](#与上游的差距)。

当前优先打磨已有 UI，暂不扩展插件等功能。应用显示名称统一为 `nsgo-music`；MyGo Music 仅用于说明上游来源。

![首页](docs/home.png)

![专辑](docs/album.png)

![正在播放](docs/now-playing.png)

![设置](docs/settings.png)

## 运行

需要 Go 1.26 或更高版本，并把 NSGo 与本项目并列放在 `../nsgo`（`go.mod` 通过 `replace` 指向本地 NSGo）。

```sh
go tool nsgo dev        # 开发运行，保存后热重载
go tool nsgo test       # 全部测试
go tool nsgo check      # vet 并检查目录
go tool nsgo build      # 按 nsgo.yaml 的目标打包
```

应用名、id、版本和构建目标在 `nsgo.yaml`。

若 `dev` 报 `the icon needs Xcode's actool`，是 `xcode-select` 指向了 CommandLineTools。选择 Xcode 后即可：

```sh
sudo xcode-select -s /Applications/Xcode.app/Contents/Developer
```

不修改全局设置时，也可以只给这条命令指定 Xcode：

```sh
DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer go tool nsgo dev
```

### 截图

`TestShots` 会在目标上跑真实窗口并保存每个页面的截图，用于对照上游：

```sh
NSGO_SHOTS=1 go tool nsgo test -target macos -run TestShots
open .nsgo/test/screenshots/macos
```

设置与歌词的专项截图使用固定播放位置，覆盖深浅主题、下拉菜单、长文本、无歌词与窄窗口：

```sh
cd internal/app
NSGO_UI_SHOTS=1 go tool nsgo test -target macos -run TestUIShots
open .nsgo/test/screenshots/macos
```

## 已实现

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

## 结构

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

## 与上游的差距

这些上游能力还没移植：

- 插件系统（JS 宿主、安装/更新、插件源、登录窗口、评论、播客、电台）
- 刘海迷你播放器（按要求不移植）
- 播客字幕与本地语音识别（sherpa-onnx）
- 雾效、进度条烟雾、封面飞入与封面墙的自由拖动等细节动画
- 自动更新与勾选通知等发布相关功能

移植约定：交互、布局、文案和设置项以 mygo-music 为准；NSGo 缺少的能力先补 NSGo，再在 app 中使用。核心包只改 import path 与平台抽象，不顺手重构播放与解码逻辑。

## 许可证

上游 mygo-music 为 AGPL-3.0，本项目作为其复刻/衍生作品同样以 AGPL-3.0 发布，见 [LICENSE](LICENSE) 与 [NOTICE](NOTICE)。NSGo 为 Apache-2.0；`internal/audio` 来自 MIT 许可的 MyGo，见 [LICENSES/MyGo-MIT.txt](LICENSES/MyGo-MIT.txt)。

- 上游：<https://github.com/egoist/mygo-music>
- 框架：<https://github.com/nsgo-dev/nsgo>
