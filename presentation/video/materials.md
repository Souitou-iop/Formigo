# materials

| file | source |
|---|---|
| assets/img/formigo-icon.png | 仓库 `iCOCN/Formigo.png`（真 Logo） |
| assets/img/desktop-hero-light.png | 仓库工作区 `docs/screenshots/`（新主题系统，中性黑白浅色） |
| assets/img/desktop-hero-dark.png、android-home.png | 仓库 `docs/screenshots/` |
| assets/img/android-neutral.jpg | 用户提供 `presentation/deck/assets/android-neutral.jpg`（中性黑白浅色） |
| assets/img/ios-neutral.png、ios-neutral-dark.png | 主会话用新构建在模拟器截的中性黑白浅色 / 深色首页 |
| assets/rec/desk-batch.mp4、desk-all.mp4、desk-opts.mp4、desk-themes.mp4 | 本机实录：`dist/darwin-universal/Formigo.app`（desk-themes 用当前工作区重新构建的 arm64 版重录，整页换色），`tools/winrec.swift` 只录 Formigo 窗口；样例文件来自 `testdata/seeds/`（改了中文文件名） |
| assets/rec/ios-flow.mp4 | 本机实录：`ios/` 工程在 iPhone Air 模拟器，`xcrun simctl io recordVideo`；照片是模拟器自带图库 |
| assets/poses/*.png | `presentation/mascot/poses/` 18 张生图姿势（原图在 `design/gopher/`），经 `tools/sync-poses.py` 统一成正方形、同一地平线 |
| 主题色 | `ios/Formigo/Model/Theme.swift`（深色值：薄荷 #3CCF9E、暖橘 #FF9447、地鼠天蓝 #5CB9F7、中性 #1A1B1F） |
| 数字与文案 | `README.md`、`CHANGELOG.md`（约 80 种格式、8 类、5 平台、HEIC → PNG → WebP、内置 FFmpeg/PDFium） |
| 字体 | Noto Sans SC、Inter（OFL，见 assets/fonts） |
| 声音 | 全部本地合成：配乐是 `tools/score.py` 写的 ABC 总谱，由 huashu-motion 的 `om sound` 用合成乐器演奏；效果音也是 `om sound` 合成。没有任何外部音频。Vertex AI 在本机只提供文本/语音/视频生成，没有音乐模型，所以没用 |
