import Foundation
import ImageIO
import UniformTypeIdentifiers

// The implementation of package heif on Apple's platforms, with ImageIO,
// which reads HEIC, HEIF and AVIF.

func makeDecoder() throws -> any Decoder { ImageIODecoder() }

enum HeifFailure: Error, CustomStringConvertible {
    case unreadable(String), unwritable(String)
    var description: String {
        switch self {
        case .unreadable(let p): return "cannot read the picture at \(p)"
        case .unwritable(let p): return "cannot write \(p)"
        }
    }
}

final class ImageIODecoder: Decoder {
    func toPNG(path: String, out: String) throws {
        guard let src = CGImageSourceCreateWithURL(URL(fileURLWithPath: path) as CFURL, nil),
              let props = CGImageSourceCopyPropertiesAtIndex(src, 0, nil) as? [CFString: Any] else {
            throw HeifFailure.unreadable(path)
        }
        let w = props[kCGImagePropertyPixelWidth] as? Int ?? 0
        let h = props[kCGImagePropertyPixelHeight] as? Int ?? 0
        // A thumbnail as large as the picture is the picture turned upright.
        let opts: [CFString: Any] = [
            kCGImageSourceCreateThumbnailFromImageAlways: true,
            kCGImageSourceCreateThumbnailWithTransform: true,
            kCGImageSourceThumbnailMaxPixelSize: max(w, h, 1),
            kCGImageSourceShouldCacheImmediately: true,
        ]
        guard let image = CGImageSourceCreateThumbnailAtIndex(src, 0, opts as CFDictionary) else {
            throw HeifFailure.unreadable(path)
        }
        guard let dst = CGImageDestinationCreateWithURL(URL(fileURLWithPath: out) as CFURL, UTType.png.identifier as CFString, 1, nil) else {
            throw HeifFailure.unwritable(out)
        }
        CGImageDestinationAddImage(dst, image, nil)
        if !CGImageDestinationFinalize(dst) {
            throw HeifFailure.unwritable(out)
        }
    }
}
