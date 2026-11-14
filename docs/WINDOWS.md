# Windows 使用与验证

本次提供 Windows x64 构建，目标为 Windows 10 / 11 x64。当前开发环境是 macOS：已完成交叉编译和共享逻辑回归，尚未在 Windows 真机验证完整播放流程。Windows ARM64 暂不作为本次交付目标。

## 安装与运行

- 安装版：运行 `woodwind-music-<版本>-windows-x64-setup.exe`，安装到当前用户目录。
- 便携版：解压 `woodwind-music-<版本>-windows-x64-portable.zip`，双击 `Woodwind Music.exe`。请保留同目录资源文件。
- 点击首页的“添加音乐文件夹”，或使用 `Ctrl+O`，导入自己的音乐。
- 关闭主窗口会隐藏到通知区域，音乐继续播放。单击托盘图标恢复窗口；右键可控制播放、打开设置或退出。
- 若通知区域图标创建失败，关闭窗口会正常退出，避免留下无法找回的后台进程。
- 设置在主窗口内打开，点击左上角返回，或按 `Esc` / `Ctrl+W` 返回曲库。
- 明确退出：托盘菜单“退出 Woodwind Music”、应用菜单退出，或 `Ctrl+Q`。

便携版指无需安装；它和安装版共用 `%APPDATA%\dev.oneo.nsgo.music` 中的曲库、歌单、喜欢和设置，音乐文件留在原目录。重装或更换程序目录不会主动清除这些数据。需要独立数据目录时可设置 `WOODWIND_MUSIC_DATA_DIR`。

本地构建尚未做 Windows 代码签名。自动更新和双击音乐文件关联尚未接入；升级时先从应用菜单或托盘退出，再运行新安装包或替换便携版文件。当前没有跨设备同步。

## 本次平台适配

- 沿用 WASAPI 共享模式音频输出，保留默认输出切换和设备丢失后的重开机制。
- 补充 Windows 通知区域图标、播放菜单、窗口隐藏与恢复，以及 Explorer 重启后的图标恢复。
- 补充系统媒体会话：歌曲、歌手、专辑、封面、播放状态、时间轴，以及播放／暂停、上一首／下一首和进度跳转命令。
- 修复盘符、中文／空格／`#`／`%` 路径及 UNC 网络共享地址的文件 URL 转换。
- 为封面渲染传入原生 Windows 路径，兼容当前 NSGo 图片加载器。
- 设置使用主窗口内页面，避免依赖当前 Windows 后端尚未提供的第二个原生窗口。

Windows 桌面服务集中在 `internal/winshell`；NSGo 补齐对应功能后，可逐步改用框架公共接口。

## 构建与自动检查

按根目录 README 安装 Go 并获取项目，NSGo 依赖会自动从远程下载，再执行：

```sh
go tool nsgo test
go tool nsgo check -target windows_x64
go tool nsgo build -target windows_x64
go run scripts/package-windows.go
```

完整安装包需要 NSIS 3：macOS 使用 `brew install makensis`；Windows 可使用 `choco install nsis`，并将 NSIS 目录加入 PATH。便携版和安装包位于 `dist/releases/`，附带 SHA-256 校验文件。

`.github/workflows/windows.yml` 会在 Windows runner 上运行测试、检查并上传安装包和便携版，不自动发布 GitHub Release。框架版本统一由 `go.mod` 固定，更新框架时应更新依赖并重新回归。

Windows 原生媒体会话测试需要已登录的桌面会话，在 PowerShell 中执行：

```powershell
$env:WOODWIND_WINDOWS_NATIVE = '1'
go test -v ./internal/winshell
Remove-Item Env:WOODWIND_WINDOWS_NATIVE
```

该测试检查真实系统媒体接口、封面、时间轴和回调，不播放声音。默认无窗口测试不会创建托盘或媒体会话。

## Windows 真机验收

以下项目仍需在 Windows 上实际运行验收，编译通过不代表已经通过这些检查：

- 首次安装、卸载、覆盖升级，以及便携版在中文路径下启动。
- 导入带中文、空格、`#`、`%` 的目录；有条件时测试 UNC 网络共享目录。
- MP3、FLAC、Ogg、WAV、M4A/AAC 的播放、暂停、拖动进度和连续切歌。
- 内嵌／同目录封面、LRC 歌词，以及歌单、喜欢和播放位置的退出后恢复。
- 播放中关闭窗口、从托盘恢复、托盘明确退出；重启 Explorer 后托盘图标恢复。
- 系统媒体面板显示信息与封面，键盘和蓝牙耳机上的媒体按键能控制播放。
- 扬声器／有线耳机／蓝牙输出切换、设备拔出重连、睡眠唤醒后继续播放。
- 100%、125%、150%、200% 缩放；最小窗口、最大化、深浅主题、中文输入搜索和设置操作。
