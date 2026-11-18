# Changelog

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
