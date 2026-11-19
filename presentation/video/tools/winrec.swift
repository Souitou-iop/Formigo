// winrec: record one app's main window (and its own menus/popovers drawn over it)
// with ScreenCaptureKit, so other apps' windows never appear even if they overlap.
//
//   swiftc -O tools/winrec.swift -o <scratch>/winrec
//   [WINREC_MODE=window] [WINREC_WID=<window id>] <scratch>/winrec <bundle-id> <out.mov> [seconds]   (stops early on SIGINT/SIGTERM)
import AppKit
import AVFoundation
import CoreMedia
import Foundation
import ScreenCaptureKit

let args = CommandLine.arguments
guard args.count >= 3 else { fputs("usage: winrec <bundle-id> <out.mov> [seconds]\n", stderr); exit(2) }
let bundleID = args[1], outURL = URL(fileURLWithPath: args[2])
let seconds = args.count > 3 ? Double(args[3]) ?? 600 : 600
let windowOnly = ProcessInfo.processInfo.environment["WINREC_MODE"] == "window"

final class Recorder: NSObject, SCStreamOutput {
  let writer: AVAssetWriter
  let input: AVAssetWriterInput
  var started = false
  init(url: URL, width: Int, height: Int) throws {
    try? FileManager.default.removeItem(at: url)
    writer = try AVAssetWriter(outputURL: url, fileType: .mov)
    input = AVAssetWriterInput(mediaType: .video, outputSettings: [
      AVVideoCodecKey: AVVideoCodecType.h264, AVVideoWidthKey: width, AVVideoHeightKey: height,
      AVVideoCompressionPropertiesKey: [AVVideoAverageBitRateKey: 24_000_000, AVVideoExpectedSourceFrameRateKey: 30],
    ])
    input.expectsMediaDataInRealTime = true
    writer.add(input)
  }
  func stream(_ stream: SCStream, didOutputSampleBuffer sb: CMSampleBuffer, of type: SCStreamOutputType) {
    guard type == .screen, sb.isValid,
      let att = CMSampleBufferGetSampleAttachmentsArray(sb, createIfNecessary: false) as? [[SCStreamFrameInfo: Any]],
      let raw = att.first?[.status] as? Int, SCFrameStatus(rawValue: raw) == .complete else { return }
    if !started {
      writer.startWriting(); writer.startSession(atSourceTime: CMSampleBufferGetPresentationTimeStamp(sb)); started = true
    }
    if input.isReadyForMoreMediaData { input.append(sb) }
  }
}

_ = NSApplication.shared  // a window server connection, needed for window-only capture
let done = DispatchSemaphore(value: 0)
for sig in [SIGINT, SIGTERM] { signal(sig) { _ in done.signal() } }

Task {
  do {
    let content = try await SCShareableContent.excludingDesktopWindows(true, onScreenWindowsOnly: !windowOnly)
    guard let app = content.applications.first(where: { $0.bundleIdentifier == bundleID }) else { fputs("app not running\n", stderr); exit(1) }
    let wins = content.windows.filter { $0.owningApplication?.bundleIdentifier == bundleID && $0.windowLayer == 0 }
    let wantID = ProcessInfo.processInfo.environment["WINREC_WID"].flatMap { UInt32($0) }  // a particular window
    guard let win = wantID.flatMap({ id in wins.first { $0.windowID == id } }) ?? wins.max(by: { $0.frame.width * $0.frame.height < $1.frame.width * $1.frame.height }) else { fputs("no window\n", stderr); exit(1) }
    guard let display = content.displays.first(where: { $0.frame.intersects(win.frame) }) ?? content.displays.first else { exit(1) }
    // WINREC_MODE=window: the window alone (works while it sits on another Space, but
    // without its menus and tooltips); default: the app's windows on the display, cropped.
    let filter = windowOnly ? SCContentFilter(desktopIndependentWindow: win)
                            : SCContentFilter(display: display, including: [app], exceptingWindows: [])
    let scale = Int(filter.pointPixelScale.rounded())
    let cfg = SCStreamConfiguration()
    let rect = windowOnly ? CGRect(origin: .zero, size: win.frame.size)
      : CGRect(x: win.frame.minX - display.frame.minX, y: win.frame.minY - display.frame.minY, width: win.frame.width, height: win.frame.height)
    if !windowOnly { cfg.sourceRect = rect }
    cfg.width = Int(rect.width) * scale; cfg.height = Int(rect.height) * scale
    cfg.minimumFrameInterval = CMTime(value: 1, timescale: 30)
    cfg.showsCursor = false
    cfg.backgroundColor = .clear
    let rec = try Recorder(url: outURL, width: cfg.width, height: cfg.height)
    let stream = SCStream(filter: filter, configuration: cfg, delegate: nil)
    try stream.addStreamOutput(rec, type: .screen, sampleHandlerQueue: DispatchQueue(label: "rec"))
    try await stream.startCapture()
    print("recording \(cfg.width)x\(cfg.height) window \(win.windowID)"); fflush(stdout)
    _ = done.wait(timeout: .now() + seconds)
    try await stream.stopCapture()
    rec.input.markAsFinished()
    await rec.writer.finishWriting()
    print("saved \(outURL.path)"); exit(0)
  } catch { fputs("error: \(error)\n", stderr); exit(1) }
}
dispatchMain()
