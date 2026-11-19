# Formigo 宣传片 · treatment（第二版）

- 用途：小组展示 + 课程「作品展示视频」（约 2–3 分钟、录屏、展示功能）。1920×1080 · 30fps · 约 2 分 0 秒 · ≤20 MB。
- 结构：动效开场（0–21 s）→ 实机录屏主体（21–90 s，bento 卡片包着录屏，右侧步骤字幕跟着录屏逐条点亮）→ 动效收尾（90 s–末）。
- 风格：浅色 `#F5F5F7` 底、白卡无投影；贴纸文件卡有海军蓝描边与硬投影；地鼠是生图位图姿势（`assets/poses/`）。
- 音乐：`tools/score.py` 写的 ABC 总谱（F 大调、120 BPM、一小节 2 s），`om sound` 本地合成：木琴主旋律 + 钟琴、尤克里里反拍、拨弦、颤音琴、暖铺底、贝斯、鼓。两次 drop（feed、bento），录屏段降一档，privacy 处几乎静音，结尾四音动机收束。无外部音频（Vertex 上没有音乐模型，只能做语音，所以退回本地合成）。

| 时间 | 场景 | 内容 |
|---|---|---|
| 0–6 | intro | peek 从土洞探头 + 文件环绕 → wave，「Formigo / 把文件丢给小地鼠。」→ dive 钻洞 |
| 6–12 | chaos | 14 张格式贴纸砸下；「格式太多，工具太杂。」→「还要先传到网上？」think→sleep；全部吸进地鼠 |
| 12–21 | feed | HEIC→PNG、MKV→MP4、FLAC→MP3 吞入吐出；juggle 连转三个，cheer 停稳再切 |
| 21–25 | demo | 「实际用一遍。」laptop 地鼠 + 桌面端 / 移动端 / 实机录屏 |
| 25–33 | d1 录屏 | 7 个文件拖入 → 按类型分组（默认 PNG）→ 点 WebP → 一键转换，完成即切走 |
| 33–46 | d2 录屏 | 图片 + PDF →「全部转为」JPG → 完成；圆环标出 ↗ 打开所在文件夹 |
| 46–56 | d5 录屏 | 选项跟着格式走：PNG 只有尺寸 → JPG 多出画质 → ICO 不显示选项 |
| 56–74 | d3 录屏 | 设置页一镜到底：薄荷 → 暖橘 → 地鼠天蓝 → 中性黑白 → 深色 → 浅色；色点圆环跟着走 |
| 74–90 | d4 录屏 | iOS 模拟器：从相册选两张照片 → 选 JPG 转换 → 分享面板一出就切 |
| 90–100 | bento | 80+ 种格式、8 大类、5 个平台、内置 FFmpeg/PDFium；HEIC→PNG→WebP 路线卡，地鼠冲浪（不出卡片） |
| 100–106 | privacy | 「文件不上传。」shield 地鼠，云朵上传被划掉 |
| 106–114.5 | platforms/themes | 「五个平台，三种语言，统一设计。」桌面 / Android / iOS 先浅色后一起切深色（iOS 中性黑白）；四张主题卡被地鼠逐张点亮 |
| 114.5–末 | end | cheer → bow + Logo +「文件不上传，转换都在你的设备上完成。」 |

## 录屏怎么来的

- 桌面端：`dist/darwin-universal/Formigo.app`，`open -g` 后台启动，文件用 `open -g -a Formigo.app <文件>`（等同拖到 Dock 图标）加入；点击用后台控制。录制用 `tools/winrec.swift`（ScreenCaptureKit，只抓 Formigo 自己的窗口，不抓光标，别的应用窗口不会入画）。窗口左上角 macOS 的「正在共享」标记在剪辑时盖掉/裁掉。
- iOS：`ios/` 工程构建到 iPhone Air 模拟器，`xcrun simctl io booted recordVideo` 录制；设置里打开「小地鼠」、浅色、中性黑白。
- 剪辑：`assets/rec/*.mp4` 是剪好的片段（去掉等待，末尾定格 1.5 s），`*-last.jpg` 是末帧，垫在卡片底下。
- 「打开所在文件夹」没有真的点：点了会把访达窗口推到前台、抢用户焦点，且单窗口录制拍不到访达，所以只用圆环标出按钮（圆环放在录屏卡片里，跟着卡片一起推近）。
- 设置页是一镜到底的实录，只在两次切换之间的静止处剪掉等待，不拼截图，顶部预览卡的动画是真实的。

## 重新渲染

```sh
python3 tools/sync-poses.py                    # 姿势图更新后重跑
python3 tools/score.py                         # 改了总谱后重跑，写回 index.html
export OM_CHROME=<Chrome 可执行文件>             # 本机用的是 Playwright 的 Chrome for Testing
om sound .                                     # 合成 assets/sound.wav（中间文件，渲染后可删）
om render . --blur                             # → out/video-v1.mp4
ffmpeg -i out/video-v1.mp4 -c:v libx264 -preset slow -crf 21 -pix_fmt yuv420p -profile:v high -tune animation \
  -c:a aac -b:a 160k -movflags +faststart formigo-promo.mp4
om still . --at 3.6                            # 封面帧 → 改名 formigo-promo-poster.png
```
署名已关（`data-om-signature="off"`）。

姿势 → 用处：peek/wave/dive 开场；surprise/think/sleep 痛点；point/surprise/run/throw/juggle/cheer 丢文件；laptop 演示标题；point/magnifier/cheer、think/point/throw、palette、devices/run/throw 四段录屏；juggle 80+ 卡；surf 路线卡；shield 隐私；devices 平台；run/palette 主题卡；cheer/bow 结尾。
