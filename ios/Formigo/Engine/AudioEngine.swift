import AVFoundation
import Foundation

/// Sound through AVAudioFile and AVAudioConverter: anything Core Audio reads
/// (MP3 and FLAC included) to AAC, ALAC or PCM.
enum AudioEngine {
    static func convert(_ input: URL, to id: String, options: ConvertOptions, to out: URL,
                        progress: @escaping ProgressHandler) throws {
        let inFile: AVAudioFile
        do { inFile = try AVAudioFile(forReading: input) } catch { throw ConvertError.unreadable }
        let inFormat = inFile.processingFormat
        let channels: AVAudioChannelCount = options.mono ? 1 : min(inFormat.channelCount, 2)
        var rate = inFormat.sampleRate
        if id == "m4a" { rate = min(rate, 48000) }

        let settings = fileSettings(id: id, rate: rate, channels: channels, depth: options.bitDepth)
        let outFile: AVAudioFile
        do {
            outFile = try AVAudioFile(forWriting: out, settings: settings, commonFormat: .pcmFormatFloat32, interleaved: false)
        } catch {
            throw ConvertError.failed("无法写入 \(id.uppercased())")
        }
        let outFormat = outFile.processingFormat
        guard let converter = AVAudioConverter(from: inFormat, to: outFormat) else { throw ConvertError.unsupported }
        converter.downmix = true

        let chunk: AVAudioFrameCount = 16384
        let ratio = outFormat.sampleRate / inFormat.sampleRate
        guard let outBuf = AVAudioPCMBuffer(pcmFormat: outFormat, frameCapacity: AVAudioFrameCount(Double(chunk) * ratio) + 1024)
        else { throw ConvertError.unsupported }
        let total = max(1, Double(inFile.length))
        var finished = false

        while true {
            outBuf.frameLength = 0
            var err: NSError?
            let status = converter.convert(to: outBuf, error: &err) { count, inStatus in
                if finished {
                    inStatus.pointee = .endOfStream
                    return nil
                }
                guard let buf = AVAudioPCMBuffer(pcmFormat: inFormat, frameCapacity: count) else {
                    inStatus.pointee = .endOfStream
                    return nil
                }
                do { try inFile.read(into: buf, frameCount: count) } catch { buf.frameLength = 0 }
                if buf.frameLength == 0 {
                    finished = true
                    inStatus.pointee = .endOfStream
                    return nil
                }
                inStatus.pointee = .haveData
                return buf
            }
            if status == .error { throw ConvertError.failed(err?.localizedDescription ?? "转换失败") }
            if outBuf.frameLength > 0 { try outFile.write(from: outBuf) }
            progress(min(0.99, Double(inFile.framePosition) / total))
            if status == .endOfStream || (status == .inputRanDry && finished) { break }
        }
    }

    static func fileSettings(id: String, rate: Double, channels: AVAudioChannelCount, depth: Int) -> [String: Any] {
        var s: [String: Any] = [AVSampleRateKey: rate, AVNumberOfChannelsKey: channels]
        switch id {
        case "m4a":
            s[AVFormatIDKey] = kAudioFormatMPEG4AAC
            s[AVEncoderBitRateKey] = channels == 1 ? 128_000 : 256_000
        case "alac":
            s[AVFormatIDKey] = kAudioFormatAppleLossless
            s[AVEncoderBitDepthHintKey] = depth
        default:
            s[AVFormatIDKey] = kAudioFormatLinearPCM
            s[AVLinearPCMBitDepthKey] = depth
            s[AVLinearPCMIsFloatKey] = false
            s[AVLinearPCMIsBigEndianKey] = id == "aiff"
            s[AVLinearPCMIsNonInterleaved] = false
        }
        return s
    }
}
