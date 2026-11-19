import SwiftUI
import UIKit

extension Color {
    init(hex: UInt32, alpha: Double = 1) {
        self.init(.sRGB, red: Double((hex >> 16) & 0xFF) / 255, green: Double((hex >> 8) & 0xFF) / 255,
                  blue: Double(hex & 0xFF) / 255, opacity: alpha)
    }
}

/// What a theme tints beyond its main color. In the light appearance: the
/// page, raised and highlighted fills, and the deep ink of text, sticker edges
/// and their hard shadows. In the dark one: the page and the surfaces on it,
/// faintly.
struct Tones {
    var bg: UInt32, panel: UInt32 = 0, surface: UInt32 = 0, raised: UInt32, tint: UInt32
    var ink: UInt32 = 0, muted: UInt32 = 0, faint: UInt32 = 0
}

/// A theme the user may choose: a main color, flat as the icon's, and the
/// tones it gives the rest of the app.
struct Accent: Identifiable {
    let id: String
    let name: String
    let light: UInt32
    let dark: UInt32
    let day: Tones
    let night: Tones

    // The same table is in the desktop's settings.go and Android's Theme.kt.
    static let all = [
        Accent(id: "mint", name: "薄荷", light: 0x11996A, dark: 0x3CCF9E,
               day: Tones(bg: 0xF0F7F3, raised: 0xE5F4EC, tint: 0xDBF0E5, ink: 0x0B3626, muted: 0x4F6B5F, faint: 0x86A094),
               night: Tones(bg: 0x101614, panel: 0x151C19, surface: 0x19211E, raised: 0x212B27, tint: 0x1C3129)),
        Accent(id: "orange", name: "暖橘", light: 0xE2681C, dark: 0xFF9447,
               day: Tones(bg: 0xFBF5F0, raised: 0xFCEBDF, tint: 0xFBE2D0, ink: 0x3D1D0A, muted: 0x7A5C4A, faint: 0xAB9283),
               night: Tones(bg: 0x161210, panel: 0x1C1714, surface: 0x211B17, raised: 0x2B231E, tint: 0x382619)),
        Accent(id: "sky", name: "地鼠天蓝", light: 0x1F7FE0, dark: 0x5CB9F7,
               day: Tones(bg: 0xF2F6FC, raised: 0xE9F3FE, tint: 0xE3F2FE, ink: 0x14213D, muted: 0x56627E, faint: 0x8E99B2),
               night: Tones(bg: 0x10141A, panel: 0x151A21, surface: 0x192027, raised: 0x212933, tint: 0x1C2C3B)),
        Accent(id: "neutral", name: "中性黑白", light: 0x1A1B1F, dark: 0xECECEE,
               day: Tones(bg: 0xF4F4F5, raised: 0xEDEDEE, tint: 0xE7E7E9, ink: 0x1A1B1F, muted: 0x5F6168, faint: 0x95979E),
               night: Tones(bg: 0x111214, panel: 0x16171A, surface: 0x1A1B1F, raised: 0x222328, tint: 0x24262B)),
    ]

    static let defaultID = "neutral"

    /// The theme by its id; an id it does not know falls back to the default.
    static func by(_ id: String) -> Accent { all.first { $0.id == id } ?? all.first { $0.id == defaultID }! }

    /// The text on a fill of this color: white or graphite, whichever stands
    /// out more.
    static func ink(on hex: UInt32) -> Color {
        func lin(_ c: UInt32) -> Double {
            let v = Double(c) / 255
            return v <= 0.03928 ? v / 12.92 : pow((v + 0.055) / 1.055, 2.4)
        }
        func lum(_ h: UInt32) -> Double {
            0.2126 * lin((h >> 16) & 0xFF) + 0.7152 * lin((h >> 8) & 0xFF) + 0.0722 * lin(h & 0xFF)
        }
        let l = lum(hex), graphite: UInt32 = 0x101113
        let onWhite = 1.05 / (l + 0.05)
        let onGraphite = (l + 0.05) / (lum(graphite) + 0.05)
        return onWhite >= onGraphite ? .white : Color(hex: graphite)
    }
}

/// Formigo's look, taken from its icon: a sky-blue gopher outlined in navy.
/// Fills are flat, edges are navy outlines, and what can be pressed casts a
/// hard shadow, as a sticker standing off the page.
struct Palette {
    var dark: Bool
    var bg, panel, surface, raised: Color
    var line, lineStrong, outline: Color
    var ink, muted, faint: Color
    var accent, accentInk: Color
    var sky, skyTint: Color
    var ok, bad: Color

    static func make(dark: Bool, oled: Bool, accent: Accent) -> Palette {
        var p: Palette
        if dark {
            p = Palette(dark: true,
                        bg: Color(hex: 0x111214), panel: Color(hex: 0x16171A), surface: Color(hex: 0x1A1B1F), raised: Color(hex: 0x222328),
                        line: Color(hex: 0xFFFFFF, alpha: 0.09), lineStrong: Color(hex: 0xFFFFFF, alpha: 0.18), outline: Color(hex: 0x050506),
                        ink: Color(hex: 0xECECEE), muted: Color(hex: 0x9A9CA3), faint: Color(hex: 0x686A72),
                        accent: Color(hex: accent.dark), accentInk: Accent.ink(on: accent.dark),
                        sky: Color(hex: 0x5CB9F7), skyTint: Color(hex: 0x24262B),
                        ok: Color(hex: 0xA3D65C), bad: Color(hex: 0xFF6B74))
            let t = accent.night
            p.bg = Color(hex: t.bg)
            p.panel = Color(hex: t.panel)
            p.surface = Color(hex: t.surface)
            p.raised = Color(hex: t.raised)
            p.skyTint = Color(hex: t.tint)
            if oled {
                // Pure black for OLED panels; cards a very deep grey so they still read.
                p.bg = .black
                p.panel = Color(hex: 0x0A0A0C)
                p.surface = Color(hex: 0x111114)
                p.raised = Color(hex: 0x1A1A1E)
                p.skyTint = Color(hex: 0x16171A)
                p.outline = Color(hex: 0x2C2C33) // edges stay visible on black
                p.line = Color(hex: 0xFFFFFF, alpha: 0.08)
                p.lineStrong = Color(hex: 0xFFFFFF, alpha: 0.16)
            }
        } else {
            p = Palette(dark: false,
                        bg: Color(hex: 0xF2F6FC), panel: .white, surface: .white, raised: Color(hex: 0xE9F3FE),
                        line: Color(hex: 0x14213D, alpha: 0.11), lineStrong: Color(hex: 0x14213D, alpha: 0.2), outline: Color(hex: 0x14213D),
                        ink: Color(hex: 0x14213D), muted: Color(hex: 0x56627E), faint: Color(hex: 0x8E99B2),
                        accent: Color(hex: accent.light), accentInk: Accent.ink(on: accent.light),
                        sky: Color(hex: 0x5CB9F7), skyTint: Color(hex: 0xE3F2FE),
                        ok: Color(hex: 0x5F9E1F), bad: Color(hex: 0xEF4F5A))
            let t = accent.day
            p.bg = Color(hex: t.bg)
            p.raised = Color(hex: t.raised)
            p.skyTint = Color(hex: t.tint)
            p.outline = Color(hex: t.ink)
            p.ink = Color(hex: t.ink)
            p.muted = Color(hex: t.muted)
            p.faint = Color(hex: t.faint)
            p.line = Color(hex: t.ink, alpha: 0.11)
            p.lineStrong = Color(hex: t.ink, alpha: 0.2)
        }
        return p
    }

    /// The color of a kind of file, from the icon's documents.
    func hue(_ kind: FileKind) -> Color {
        switch kind {
        case .image: Color(hex: dark ? 0xA597FF : 0x7A68F0)
        case .audio: Color(hex: dark ? 0x3FD39F : 0x1FA874)
        case .data: Color(hex: dark ? 0x5CB9F7 : 0x2492E0)
        case .document: Color(hex: dark ? 0xFFBF55 : 0xE8930F)
        case .video: Color(hex: dark ? 0xFF7A7F : 0xE8484F)
        case .subtitle: Color(hex: dark ? 0x4FD6D0 : 0x149E99)
        case .archive: Color(hex: dark ? 0xA3AEC8 : 0x6B7895)
        }
    }

    /// A kind's color as a sticker's fill.
    func tint(_ kind: FileKind) -> Color { hue(kind).opacity(dark ? 0.18 : 0.14) }
}

private struct PaletteKey: EnvironmentKey {
    static let defaultValue = Palette.make(dark: false, oled: false, accent: Accent.by(Accent.defaultID))
}

extension EnvironmentValues {
    var palette: Palette {
        get { self[PaletteKey.self] }
        set { self[PaletteKey.self] = newValue }
    }
}

/// The motion vocabulary, from the desktop app's springs.
enum Motion {
    /// Layout: panels arriving, rows moving into place.
    static let settle = Animation.spring(duration: 0.52, bounce: 0.15)
    /// Direct responses to touch.
    static let snap = Animation.spring(duration: 0.28, bounce: 0.25)
    /// Small rewards, as a check mark landing.
    static let pop = Animation.spring(duration: 0.48, bounce: 0.5)
    /// The gopher's jump.
    static let hop = Animation.spring(duration: 0.56, bounce: 0.55)
}

/// The icon's sticker: a navy outline and a hard shadow below it.
struct Sticker: ViewModifier {
    @Environment(\.palette) private var p
    var fill: Color
    var radius: CGFloat = 16
    var lift: CGFloat = 3
    var width: CGFloat = 2

    func body(content: Content) -> some View {
        let shape = RoundedRectangle(cornerRadius: radius, style: .continuous)
        content
            .background(shape.fill(fill))
            .overlay(shape.strokeBorder(p.outline, lineWidth: width))
            .background(shape.fill(p.outline).offset(y: lift))
    }
}

extension View {
    func sticker(_ fill: Color, radius: CGFloat = 16, lift: CGFloat = 3, width: CGFloat = 2) -> some View {
        modifier(Sticker(fill: fill, radius: radius, lift: lift, width: width))
    }
}

/// A sticker button that sinks onto its shadow as it is pressed. With
/// haptics, it clicks like a key: a firm tap going down, a soft one coming up.
struct StickerButtonStyle: ButtonStyle {
    @Environment(\.palette) private var p
    @Environment(\.isEnabled) private var enabled
    var fill: Color
    var radius: CGFloat = 16
    var lift: CGFloat = 4
    var haptics = false

    func makeBody(configuration: Configuration) -> some View {
        let down = configuration.isPressed
        let shape = RoundedRectangle(cornerRadius: radius, style: .continuous)
        configuration.label
            .background(shape.fill(fill))
            .overlay(shape.strokeBorder(p.outline, lineWidth: 2))
            .offset(y: down ? lift : 0)
            .background(shape.fill(p.outline).offset(y: lift))
            .scaleEffect(down ? 0.985 : 1)
            .opacity(enabled ? 1 : 0.5)
            .animation(Motion.snap, value: down)
            .onChange(of: down) { _, pressed in
                guard haptics else { return }
                if pressed {
                    KeyHaptics.down.impactOccurred(intensity: 0.7)
                } else {
                    KeyHaptics.up.impactOccurred(intensity: 0.5)
                }
            }
    }
}

/// The generators of a key's click, made once and kept warm.
@MainActor
enum KeyHaptics {
    static let down: UIImpactFeedbackGenerator = {
        let g = UIImpactFeedbackGenerator(style: .rigid)
        g.prepare()
        return g
    }()
    static let up: UIImpactFeedbackGenerator = {
        let g = UIImpactFeedbackGenerator(style: .soft)
        g.prepare()
        return g
    }()
}

/// A quiet button that shrinks a little under the finger.
struct SoftPressStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .scaleEffect(configuration.isPressed ? 0.94 : 1)
            .animation(Motion.snap, value: configuration.isPressed)
    }
}

extension Font {
    static func rounded(_ size: CGFloat, _ weight: Font.Weight = .bold) -> Font {
        .system(size: size, weight: weight, design: .rounded)
    }
}
