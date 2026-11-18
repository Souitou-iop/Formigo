import AVFoundation
import Foundation
import ImageIO

/// Video through AVFoundation: re-encoding, the audio track, and GIFs.
enum VideoEngine {
    static func convert(_ input: URL, to id: String, options: ConvertOptions, to out: URL,
                        progress: @escaping ProgressHandler) async throws {
        let asset = AVURLAsset(url: input)
        switch id {
        case "mp4", "mov", "hevc":
            let preset = presetName(id: id, resolution: options.videoResolution)
            try await export(asset, preset: preset, type: id == "mov" ? .mov : .mp4, to: out, progress: progress)
        case "m4a":
            let tracks = try await asset.loadTracks(withMediaType: .audio)
            guard !tracks.isEmpty else { throw ConvertError.noAudioTrack }
            if options.mono {
                let tmp = out.deletingLastPathComponent().appendingPathComponent(UUID().uuidString + ".m4a")
                try await export(asset, preset: AVAssetExportPresetAppleM4A, type: .m4a, to: tmp) { progress($0 * 0.7) }
                defer { try? FileManager.default.removeItem(at: tmp) }
                var o = options
                o.mono = true
                try AudioEngine.convert(tmp, to: "m4a", options: o, to: out) { progress(0.7 + $0 * 0.3) }
            } else {
                try await export(asset, preset: AVAssetExportPresetAppleM4A, type: .m4a, to: out, progress: progress)
            }
        case "gif":
            try await gif(asset, scale: options.scale, to: out, progress: progress)
        default:
            throw ConvertError.unsupported
        }
    }

    static func presetName(id: String, resolution: String) -> String {
        if id == "hevc" {
            return resolution == "original" ? AVAssetExportPresetHEVCHighestQuality : AVAssetExportPresetHEVC1920x1080
        }
        switch resolution {
        case "1080": return AVAssetExportPreset1920x1080
        case "720": return AVAssetExportPreset1280x720
        default: return AVAssetExportPresetHighestQuality
        }
    }

    static func export(_ asset: AVAsset, preset: String, type: AVFileType, to out: URL,
                       progress: @escaping ProgressHandler) async throws {
        guard let session = AVAssetExportSession(asset: asset, presetName: preset) else {
            throw ConvertError.unsupported
        }
        session.outputURL = out
        session.outputFileType = type
        session.shouldOptimizeForNetworkUse = true
        let ticker = Task {
            while !Task.isCancelled {
                progress(Double(session.progress))
                try? await Task.sleep(nanoseconds: 200_000_000)
            }
        }
        defer { ticker.cancel() }
        await withCheckedContinuation { (c: CheckedContinuation<Void, Never>) in
            session.exportAsynchronously { c.resume() }
        }
        switch session.status {
        case .completed: return
        case .cancelled: throw CancellationError()
        default: throw ConvertError.failed(session.error?.localizedDescription ?? "导出失败")
        }
    }

    /// The video as an animated GIF at up to 10 frames a second, 480 points wide.
    static func gif(_ asset: AVAsset, scale: Double, to out: URL, progress: @escaping ProgressHandler) async throws {
        let duration = try await asset.load(.duration).seconds
        guard duration.isFinite, duration > 0 else { throw ConvertError.unreadable }
        let frames = max(2, min(150, Int(duration * 10)))
        let step = duration / Double(frames)
        let gen = AVAssetImageGenerator(asset: asset)
        gen.appliesPreferredTrackTransform = true
        let side = 480 * scale
        gen.maximumSize = CGSize(width: side, height: side)
        gen.requestedTimeToleranceBefore = CMTime(seconds: step / 2, preferredTimescale: 600)
        gen.requestedTimeToleranceAfter = CMTime(seconds: step / 2, preferredTimescale: 600)

        guard let dest = CGImageDestinationCreateWithURL(out as CFURL, "com.compuserve.gif" as CFString, frames, nil) else {
            throw ConvertError.unsupported
        }
        CGImageDestinationSetProperties(dest, [kCGImagePropertyGIFDictionary: [kCGImagePropertyGIFLoopCount: 0]] as CFDictionary)
        let frameProps = [kCGImagePropertyGIFDictionary: [kCGImagePropertyGIFDelayTime: step]] as CFDictionary
        for i in 0..<frames {
            try Task.checkCancellation()
            let t = CMTime(seconds: Double(i) * step, preferredTimescale: 600)
            if let (image, _) = try? await gen.image(at: t) {
                CGImageDestinationAddImage(dest, image, frameProps)
            }
            progress(Double(i + 1) / Double(frames))
        }
        guard CGImageDestinationFinalize(dest) else { throw ConvertError.failed("写入失败") }
    }
}
