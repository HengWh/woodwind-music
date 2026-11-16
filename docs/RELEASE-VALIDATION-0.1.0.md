# 0.1.0 发布验证

日期：2026-11-16。构建主机：macOS Apple Silicon，Go 1.27.3，远程依赖 NSGo v0.1.0；未使用相邻框架工作区替换。

## 本次结果

| 项目 | 结果 |
| --- | --- |
| 完整无窗口回归 | 250 通过、0 失败、10 跳过 |
| macOS / Windows / Android 静态检查 | 全部通过 |
| macOS 原生冒烟 | 干净安装、启动及本地文件播放：3 通过、0 失败 |
| macOS arm64 DMG | 构建通过，应用签名及镜像校验通过 |
| Windows x64 | 安装程序、便携包构建通过；完整真机验收未完成，按实验性支持发布 |
| Android arm64 APK | 完整后台宿主构建通过，v3 签名与 16 KiB 对齐检查通过 |
| Android 清单 | versionName 0.1.0，versionCode 131，debuggable=false，allowBackup=false，包含 PlaybackService |
| 发布包隐私 | APK、macOS 应用及 Windows 便携包文件清单检查通过；只包含批准的图标资源，不含插件和账号存储 |
| 新安装状态 | 无窗口回归覆盖三平台环境，默认仅有本地音乐源，无插件、无账号；macOS 原生复测通过 |
| 安装脚本 | 下载前缀已统一为 wood-wind-music，与实际附件一致 |

完整测试中跳过的为可选截图及平台专用项目。原生冒烟初次选择了仅适用于模拟宿主的安卓桥接测试，以及依赖英文文案但未固定语言的测试：前者在 macOS 原生环境不适用，后者已固定测试语言并复测通过。安卓桥接已由完整无窗口回归覆盖。

本次没有连接到 Android 真机，未在新版框架下重新声称后台、熄屏、蓝牙或来电场景已通过；此前设备记录见 ANDROID-VALIDATION.md 和 MOBILE-POLISH-2026-11-15.md。Windows 编译和共享逻辑检查不代替真机验收。

## 包与数据边界

- 构建只复制 resources/ 中三项批准的图标资源。第三方插件代码和 plugin-data/ 账号存储仅由各安装实例的数据目录加载，没有从构建主机或手机提取用户数据。
- APK 清单只允许程序、框架、图标、网络配置和签名元数据；macOS 应用及 Windows 便携包同样按文件清单检查。
- 保留稳定应用 ID 和现有 Android 签名。内部构建号独立于显示版本，131 高于私用 0.1.30 的版本码 130。
- 新安装无预装第三方插件与登录态；覆盖升级保留用户自己的本机数据，不执行清除或迁移账号操作。
- 框架生成的不完整 Android 基础包已从发布目录移除，仅交付包含后台服务的完整 APK。

## 最终 SHA-256

| 文件 | SHA-256 |
| --- | --- |
| wood-wind-music-0.1.0-macos-arm64.dmg | `06eace5d36c52f53dc36f8f2b7f1bffe9190dc08815cde561f5859ddedec4bcf` |
| wood-wind-music-0.1.0-windows-x64-setup.exe | `886847c62188c7200607574d9f1a3c97d7fc08d04d0da3c8f2beeffb4f87bf11` |
| wood-wind-music-0.1.0-windows-x64-portable.zip | `39b7861984f4e315323aa4a356d0d21d901919f9e3c199c6ba87754eb06b45e5` |
| wood-wind-music-0.1.0-android-arm64.apk | `e50a6c1d11c246fdb7b6cfff2b8b7f27e194a741aab84e176e649e51c6ceb6cb` |

本地日志与机器可读清单位于已忽略的 build/release-0.1.0/，不随二进制包分发。重新构建后需重新运行 scripts/prepare-release.py，并更新上述哈希。
