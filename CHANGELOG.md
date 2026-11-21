# Changelog

## 0.5.0

### ✨ 新功能 / Features

- 三端都能检查更新了：桌面端每天在后台自动检查一次，发现新版本就弹出窗口展示发布说明，可一键下载安装（增量更新）并在退出后重启换新，也能跳过某个版本；入口在 macOS 应用菜单的「检查更新…」，三端设置 → 关于里也都有手动检查的按钮。Android 和 iOS 检查 GitHub 的最新发布版本，有新版时带你去下载页 / Every platform can now check for updates: the desktop app checks in the background once a day, shows the release notes of a new version in its window and installs it with a delta download, restarting into it as the app quits; a version can be skipped, and the entries are "Check for Updates" in the app menu on macOS and a button in Settings → About everywhere. Android and iOS read GitHub's latest release and take you to its page to download
- 桌面端的更新带签名校验：安装包由 CI 用 Ed25519 签名，应用用内置公钥验签后才替换自己，改不动的安装位置（如包管理器的目录）不提供自动更新 / Desktop updates are signed: CI signs each archive with Ed25519 and the app verifies it against a key built in before replacing itself; installs it cannot write to, such as a package manager's directory, offer no self-update

## 0.4.1

### 🐛 修复 / Fixes

- 桌面端改用内置解码器读取 JPEG XL 输入，恢复 JXL 转 PNG、JPG、WebP、PDF 等目标；捆绑的 FFmpeg 原本缺少 JPEG XL 解码器 / Desktop now decodes JPEG XL inputs with its built-in decoder, restoring conversions from JXL to the offered targets; the bundled FFmpeg had no JPEG XL decoder
- 修复 FFmpeg 进度解析读取诊断缓冲区时的数据竞态 / Fixed a data race while reading the diagnostics buffer used to parse FFmpeg progress

## 0.4.0

### 🎨 界面 / Design

- 主题从「只换强调色」扩展到整个界面：浅色下页面底色、卡片与高亮底色、贴纸描边、硬投影和文字都随主题染色（薄荷是深绿、暖橘是深棕、地鼠天蓝是海军蓝、中性黑白是石墨黑）；深色下页面和卡片带一点主题色。三端同步 / A theme now colors the whole window, not only its accent: in the light, the page, card and highlight fills, sticker edges, hard shadows and text take its tones (deep green for mint, deep brown for orange, navy for sky, graphite for neutral); in the dark, the page and cards take a faint tint of it. On every platform
- 「中性白」改名为「中性黑白」，与它黑色的强调色一致；页面底色改为不带蓝调的中性灰 / “中性白” is renamed “中性黑白” to match its black accent, and its page is a plain neutral grey rather than a bluish one

## 0.3.0

首个公开版本：桌面端支持约 80 种格式互转，Android 与 iOS 改为纯原生应用，三端统一换上四套主题和 OLED 纯黑模式。/ The first public release: the desktop app converts between some 80 formats, Android and iOS become fully native apps, and every platform gets four themes and a pure-black OLED mode.

### ✨ 新功能 / Features

- 支持的格式扩展到约 80 种，涵盖图片、视频、音频、文档、数据、字幕、压缩包、字体八类；需要多步转换时会自动规划最短路径 / Converts between some 80 formats across eight kinds (pictures, video, sound, documents, data, subtitles, archives and fonts), planning the shortest chain of steps when one step is not enough
- 桌面端内置 FFmpeg，视频与音频开箱即用；PDF 用内置的 PDFium 渲染成图片或提取文字；装有 LibreOffice 时额外支持 Word、Excel、PowerPoint 等 Office 格式 / The desktop app bundles FFmpeg for video and sound, renders PDF pages and reads their text with a built-in PDFium, and handles Word, Excel and PowerPoint when LibreOffice is installed
- 一次导入多种格式时按类型自动分组：每组单独选目标格式，也能用「全部转为」把整批文件转成同一种格式，单个文件还可以另选 / Files of different kinds dropped together are grouped by kind: each group picks its own format, “Convert all to” sets one format for the whole batch, and any single file can still take another
- 右键转换完成的文件，可以在访达或资源管理器中显示、直接打开，或定位原文件 / Right-click a converted file to show it in Finder or Explorer, open it, or show the original
- Android 改为 Kotlin + Jetpack Compose 原生应用，iOS 改为 SwiftUI 原生应用，各自带原生转换引擎 / Android is now a native Kotlin and Jetpack Compose app and iOS a native SwiftUI app, each with its own native conversion engine

### 🎨 界面 / Design

- 四套主题可在「设置 → 外观」切换：中性白（默认）、薄荷、暖橘、地鼠天蓝；深色模式改用中性石墨灰背景 / Four themes in Settings → Appearance: neutral (the default), mint, orange and gopher sky; dark mode now sits on neutral graphite
- 新增 OLED 纯黑模式，深色模式下可开启 / A pure-black OLED mode, offered in dark mode
- 首页的小地鼠周围有文件环绕旋转，点一下地鼠会跳起来，把文件踢着多转一圈 / Files orbit the gopher on the home screen; tap it and it hops, kicking them on round the orbit
- 设置页的预览卡片会循环演示一次转换，跟随当前主题和动画设置 / The preview card in Settings plays a conversion on a loop, in the current theme and at the chosen motion setting
- Android 底栏换成液态玻璃样式，指示器可以拖动；点地鼠和「从相册选择」「选择文件」按钮带震动反馈（iOS 同样有按键震动） / Android's bottom bar is Liquid Glass with a draggable indicator; tapping the gopher and the “Choose from Photos” and “Choose files” buttons gives haptic feedback, as the buttons do on iOS

### 🔧 其他 / Other

- 移除「界面宽度」与「界面密度」设置，布局随窗口自动调整 / Removes the interface width and density settings: the layout follows the window
- 桌面端的手机界面代码随移动端原生化一并移除 / The desktop code's phone interface goes, now that the mobile apps are native
