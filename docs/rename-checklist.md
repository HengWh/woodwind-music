# Woodwind Music 更名记录

## 当前状态

- 已完成本地代码、应用名称、图标与发布配置修改。
- 已通过静态检查、124 项测试（2 项截图测试按需跳过），并生成 0.1.23 的 macOS 应用与 DMG。
- 远端仓库已重命名为 `HengWh/woodwind-music`，仓库 ID 保持为 `1452671750`。
- 本地 `origin` 的 fetch/push 地址已更新为 `git@github.com:HengWh/woodwind-music.git`，已通过 SSH 读取远端 HEAD 验证连接。
- 本地目录与项目已更新为 `/Users/henry/code/woodwind-music`，并确认 `../nsgo` 相对依赖正常解析。
- 0.1.23 尚未发布到 GitHub Releases。

## 命名约定

| 用途 | 名称 |
| --- | --- |
| 应用显示名、菜单、关于窗口、macOS 应用包 | Woodwind Music |
| 仓库、本地目录、发布文件前缀 | woodwind-music |
| 目标 GitHub 仓库 | HengWh/woodwind-music |
| Go 模块 | github.com/HengWh/woodwind-music |
| 首次使用新名称的版本 | 0.1.23 |
| 应用内部 ID | dev.oneo.nsgo.music（保留，维持已有数据与系统身份） |
| 图标 | assets/woodwind-icon-anime/woodwind-icon-anime-1024.png |

## 本地代码

应用显示名、Go 模块与所有内部导入、网络 User-Agent、环境变量、构建产物说明和发布目标均使用新名称。数据目录支持 `WOODWIND_MUSIC_DATA_DIR`，旧的 `NSGO_MUSIC_DATA_DIR` 作为兼容入口保留。

NSGo 框架、MyGo Music 上游说明、许可证与作者致谢继续保留。旧应用 ID 也是有意保留的兼容标识，无需移动现有用户数据。

## 远端仓库

已在原仓库中将 `nsgo-music` 改为 `woodwind-music`，保留原仓库的历史、Issues 和 Stars。不要重新使用旧仓库名。

本地远端已更新并验证可以读取，其他已有克隆可使用：

```sh
git remote set-url origin git@github.com:HengWh/woodwind-music.git
git remote -v
git ls-remote origin HEAD
```

`nsgo.yaml` 显式设置 `release.github: HengWh/woodwind-music`，生成的安装脚本会使用新地址。发布 `v0.1.23` 时需上传新名称的安装包、校验文件和安装脚本；旧版本发布记录可以保留。

GitHub 改名与重定向行为见 [官方说明](https://docs.github.com/en/repositories/creating-and-managing-repositories/renaming-a-repository)。

## 工作目录

本地目录已从 `/Users/henry/code/nsgo-music` 改为 `/Users/henry/code/woodwind-music`，项目已从新路径重新打开。与 `../nsgo` 的相对依赖已验证保持有效。

本地目录改名不会改变应用名称或用户数据位置。

## 验收

```sh
go tool nsgo check
go tool nsgo test
go tool nsgo build
```

检查应用包显示名为 `Woodwind Music`，内部 ID 保持不变，ICNS 为选中的二次元图标，DMG 前缀为 `woodwind-music`，安装脚本仓库为 `HengWh/woodwind-music`。确认旧曲库、歌单、设置与播放记录仍然可用。
