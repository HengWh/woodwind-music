# WoodWind 二次元图标

保留耳机、松树、两片风吹叶子与冰蓝底板。改用二次元插画风格：深蓝色描边、清晰的赛璐璐明暗、薄荷绿与翠绿树冠、淡紫阴影和绘画式高光。

由内置图像生成工具完成风格重绘，再整理透明边缘与标准尺寸。

本版为 PNG 位图；原 SVG 与写实版 `../woodwind-icon-v2/` 均保留。此版本已选为应用图标，正式版与开发版统一使用。

在项目根目录运行 `go run ./icon`，会从 1024 像素主图刷新应用 PNG 与 macOS ICNS；运行 `go run ./icon/tray` 刷新配套的耳机与松树菜单栏轮廓。

## 文件

- `woodwind-icon-anime-1024.png`：1024 像素主图，透明外边距。
- `woodwind-icon-anime-{512,256,128,64,32}.png`：小尺寸导出。
- `preview-light-dark.png`：深浅背景和小尺寸预览。
- `generation-prompt.txt`：完整生成提示词。
