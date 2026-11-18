// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "Heif",
    platforms: [.macOS(.v12)],
    products: [.library(name: "Heif", type: .dynamic, targets: ["Heif"])],
    dependencies: [],
    targets: [
        .target(name: "Heif"),
    ],
    swiftLanguageModes: [.v5]
)
