import CoreGraphics
import Foundation
import ImageIO
import PDFKit
import UIKit
import ZIPFoundation

/// Pictures through ImageIO, and PDFs through PDFKit.
enum ImageEngine {
    static func convert(_ input: URL, to id: String, options: ConvertOptions, to out: URL) throws {
        guard let src = CGImageSourceCreateWithURL(input as CFURL, nil), CGImageSourceGetCount(src) > 0 else {
            throw ConvertError.unreadable
        }
        guard let uti = Formats.imageUTIs[id], Formats.writableImages.contains(id) else {
            throw ConvertError.unsupported
        }
        let count = CGImageSourceGetCount(src)

        // An animated GIF stays animated when it stays a GIF.
        if id == "gif", count > 1 {
            try animatedGIF(src, count: count, scale: options.scale, to: out)
            return
        }

        let index = largestIndex(src)
        guard var image = render(src, index: index, scale: id == "ico" || id == "icns" ? 1 : options.scale) else {
            throw ConvertError.unreadable
        }
        switch id {
        case "ico":
            // ImageIO writes icons only of the standard sizes: the smallest that
            // holds the picture, 256 at most.
            let longest = max(image.width, image.height)
            image = squared(image, side: [16, 32, 48, 64, 128, 256].first { $0 >= longest } ?? 256)
        case "icns": image = squared(image, side: 512)
        case "jpg", "bmp": image = flattened(image)
        default: break
        }
        guard let dest = CGImageDestinationCreateWithURL(out as CFURL, uti as CFString, 1, nil) else {
            throw ConvertError.unsupported
        }
        var props: [CFString: Any] = [:]
        if ["jpg", "heic", "webp", "avif"].contains(id) {
            props[kCGImageDestinationLossyCompressionQuality] = options.quality
        }
        CGImageDestinationAddImage(dest, image, props as CFDictionary)
        guard CGImageDestinationFinalize(dest) else { throw ConvertError.failed("写入失败") }
    }

    /// The index of the largest picture in the file: icons hold several.
    static func largestIndex(_ src: CGImageSource) -> Int {
        var best = CGImageSourceGetPrimaryImageIndex(src), area = 0
        let count = CGImageSourceGetCount(src)
        guard count > 1 else { return best }
        for i in 0..<count {
            guard let p = CGImageSourceCopyPropertiesAtIndex(src, i, nil) as? [CFString: Any],
                  let w = p[kCGImagePropertyPixelWidth] as? Int, let h = p[kCGImagePropertyPixelHeight] as? Int
            else { continue }
            if w * h > area { area = w * h; best = i }
        }
        return best
    }

    /// The picture at index, upright, resized by scale.
    static func render(_ src: CGImageSource, index: Int, scale: Double) -> CGImage? {
        let props = CGImageSourceCopyPropertiesAtIndex(src, index, nil) as? [CFString: Any] ?? [:]
        let w = props[kCGImagePropertyPixelWidth] as? Int ?? 0
        let h = props[kCGImagePropertyPixelHeight] as? Int ?? 0
        let side = max(1, Int((Double(max(w, h)) * scale).rounded()))
        let opts: [CFString: Any] = [
            kCGImageSourceCreateThumbnailFromImageAlways: true,
            kCGImageSourceCreateThumbnailWithTransform: true,
            kCGImageSourceThumbnailMaxPixelSize: side,
            kCGImageSourceShouldCacheImmediately: true,
        ]
        return CGImageSourceCreateThumbnailAtIndex(src, index, opts as CFDictionary)
    }

    static func flattened(_ image: CGImage) -> CGImage {
        guard image.alphaInfo != .none, image.alphaInfo != .noneSkipLast, image.alphaInfo != .noneSkipFirst else { return image }
        let ctx = CGContext(data: nil, width: image.width, height: image.height, bitsPerComponent: 8, bytesPerRow: 0,
                            space: CGColorSpace(name: CGColorSpace.sRGB)!, bitmapInfo: CGImageAlphaInfo.noneSkipLast.rawValue)!
        ctx.setFillColor(CGColor(gray: 1, alpha: 1))
        ctx.fill(CGRect(x: 0, y: 0, width: image.width, height: image.height))
        ctx.draw(image, in: CGRect(x: 0, y: 0, width: image.width, height: image.height))
        return ctx.makeImage() ?? image
    }

    /// The picture fit into a transparent square: what icon formats hold.
    static func squared(_ image: CGImage, side: Int) -> CGImage {
        let ctx = CGContext(data: nil, width: side, height: side, bitsPerComponent: 8, bytesPerRow: 0,
                            space: CGColorSpace(name: CGColorSpace.sRGB)!, bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
        ctx.interpolationQuality = .high
        let s = Double(side) / Double(max(image.width, image.height))
        let w = Double(image.width) * s, h = Double(image.height) * s
        ctx.draw(image, in: CGRect(x: (Double(side) - w) / 2, y: (Double(side) - h) / 2, width: w, height: h))
        return ctx.makeImage() ?? image
    }

    static func animatedGIF(_ src: CGImageSource, count: Int, scale: Double, to out: URL) throws {
        guard let dest = CGImageDestinationCreateWithURL(out as CFURL, "com.compuserve.gif" as CFString, count, nil) else {
            throw ConvertError.unsupported
        }
        let fileProps = CGImageSourceCopyProperties(src, nil) as? [CFString: Any] ?? [:]
        CGImageDestinationSetProperties(dest, fileProps as CFDictionary)
        for i in 0..<count {
            guard let frame = render(src, index: i, scale: scale) else { continue }
            let p = CGImageSourceCopyPropertiesAtIndex(src, i, nil) as? [CFString: Any] ?? [:]
            var frameProps: [CFString: Any] = [:]
            if let gif = p[kCGImagePropertyGIFDictionary] { frameProps[kCGImagePropertyGIFDictionary] = gif }
            CGImageDestinationAddImage(dest, frame, frameProps as CFDictionary)
        }
        guard CGImageDestinationFinalize(dest) else { throw ConvertError.failed("写入失败") }
    }

    /// A picture as a one-page PDF, the page as large as the picture at 144 dpi.
    static func pdf(from input: URL, options: ConvertOptions, to out: URL) throws {
        guard let src = CGImageSourceCreateWithURL(input as CFURL, nil),
              let image = render(src, index: CGImageSourceGetPrimaryImageIndex(src), scale: options.scale)
        else { throw ConvertError.unreadable }
        let flat = flattened(image)
        let ui: UIImage
        if let data = UIImage(cgImage: flat).jpegData(compressionQuality: options.quality), let j = UIImage(data: data) {
            ui = j
        } else {
            ui = UIImage(cgImage: flat)
        }
        let page = CGRect(x: 0, y: 0, width: Double(flat.width) / 2, height: Double(flat.height) / 2)
        let fmt = UIGraphicsPDFRendererFormat()
        fmt.documentInfo = [kCGPDFContextCreator as String: "Formigo"]
        try UIGraphicsPDFRenderer(bounds: page, format: fmt).writePDF(to: out) { ctx in
            ctx.beginPage()
            ui.draw(in: page)
        }
    }

    /// A PDF's pages as pictures (several pages go into a ZIP), or its text.
    static func fromPDF(_ input: URL, to id: String, options: ConvertOptions, to out: URL, baseName: String) throws -> URL {
        guard let doc = PDFDocument(url: input) else { throw ConvertError.unreadable }
        if id == "txt" {
            try (doc.string ?? "").write(to: out, atomically: true, encoding: .utf8)
            return out
        }
        guard id == "png" || id == "jpg" else { throw ConvertError.unsupported }
        guard doc.pageCount > 0 else { throw ConvertError.unreadable }
        func page(_ i: Int) -> Data? {
            guard let p = doc.page(at: i) else { return nil }
            let box = p.bounds(for: .mediaBox)
            let s = 2.0 * options.scale
            let size = CGSize(width: box.width * s, height: box.height * s)
            let fmt = UIGraphicsImageRendererFormat()
            fmt.scale = 1
            fmt.opaque = id == "jpg"
            let img = UIGraphicsImageRenderer(size: size, format: fmt).image { ctx in
                UIColor.white.setFill()
                ctx.fill(CGRect(origin: .zero, size: size))
                ctx.cgContext.translateBy(x: 0, y: size.height)
                ctx.cgContext.scaleBy(x: s, y: -s)
                p.draw(with: .mediaBox, to: ctx.cgContext)
            }
            return id == "jpg" ? img.jpegData(compressionQuality: options.quality) : img.pngData()
        }
        if doc.pageCount == 1 {
            guard let d = page(0) else { throw ConvertError.unreadable }
            try d.write(to: out)
            return out
        }
        let zipURL = out.deletingPathExtension().appendingPathExtension("zip")
        try? FileManager.default.removeItem(at: zipURL)
        let archive = try Archive(url: zipURL, accessMode: .create)
        for i in 0..<doc.pageCount {
            guard let d = page(i) else { continue }
            try archive.addEntry(with: "\(baseName)-\(i + 1).\(id)", type: .file, uncompressedSize: Int64(d.count),
                                 compressionMethod: .none) { pos, size in
                d.subdata(in: Int(pos)..<Int(pos) + size)
            }
        }
        return zipURL
    }
}
