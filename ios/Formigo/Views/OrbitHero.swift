import SwiftUI
import UIKit

/// The empty window's hero, as the desktop's: documents ride a tilted orbit
/// round the gopher, the far half behind it and the near half in front. A tap
/// on the gopher makes it hop and kicks the documents on round.
struct OrbitHero: View {
    @EnvironmentObject private var settings: AppSettings
    @Environment(\.palette) private var p
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    var width: CGFloat = 340

    @State private var boops = 0
    @State private var boopAt: Date?
    @State private var arrived: Date?
    @State private var wake = false // re-evaluates the pause once a kick ends

    private var still: Bool { reduceMotion || settings.motion == "off" }

    var body: some View {
        let _ = wake
        let w = width, h = width * 0.86
        let side = w * 0.74
        let paused = still && !kicking(at: Date()) && arrived != nil && !opening(at: Date())
        TimelineView(.animation(minimumInterval: 1.0 / 60, paused: paused)) { tl in
            let now = tl.date
            let o = frame(now)
            ZStack {
                Canvas { ctx, size in draw(ctx, size, o, front: false) }
                    .padding(-Self.bleed)
                Group {
                    if settings.mascot {
                        OrbitGopher(side: side, hops: boops, playful: !still)
                    } else {
                        Image(systemName: "arrow.triangle.2.circlepath")
                            .font(.system(size: side * 0.22, weight: .bold))
                            .foregroundStyle(p.accentInk)
                            .frame(width: side * 0.46, height: side * 0.46)
                            .sticker(p.accent, radius: side * 0.14, lift: 6, width: 3)
                    }
                }
                .contentShape(Rectangle())
                .onTapGesture(perform: boop)
                .offset(y: -w * 0.02)
                Canvas { ctx, size in draw(ctx, size, o, front: true) }
                    .padding(-Self.bleed)
                    .allowsHitTesting(false)
            }
            .frame(width: w, height: h)
        }
        .frame(width: w, height: h)
        .sensoryFeedback(.impact(weight: .light), trigger: boops)
        .onAppear { if arrived == nil { arrived = Date() } }
        .accessibilityElement()
        .accessibilityLabel("小地鼠")
        .accessibilityAddTraits(.isButton)
        .accessibilityAction { boop() }
    }

    private func boop() {
        boops += 1
        boopAt = Date()
        if still {
            DispatchQueue.main.asyncAfter(deadline: .now() + 1) { wake.toggle() }
        }
    }

    private func kicking(at now: Date) -> Bool {
        guard let boopAt else { return false }
        return now.timeIntervalSince(boopAt) < 0.95
    }

    private func opening(at now: Date) -> Bool {
        guard let arrived, !still else { return false }
        return now.timeIntervalSince(arrived) < 1.1
    }

    /// Where the orbit stands at a moment.
    struct Frame {
        var turn: Double
        var dash: Double
        var open: Double
        var flash: Double
    }

    private func frame(_ now: Date) -> Frame {
        // It spreads from 0.6 of its size as the view arrives.
        var open = 1.0
        if !still, let arrived {
            let t = max(0, now.timeIntervalSince(arrived) - 0.12) / 0.9
            open = Ease.outExpoish(min(1, t))
        }
        let secs = still ? 0 : now.timeIntervalSinceReferenceDate.truncatingRemainder(dividingBy: 1e6)
        var turn = secs / Orbit.period * 2 * .pi
        var flash = 0.0
        if boops > 0 {
            var kick = 1.0
            if let boopAt {
                let t = now.timeIntervalSince(boopAt)
                let k = min(1, t / 0.9)
                kick = 1 - pow(1 - k, 3)
                flash = max(0, 1 - t / 0.7)
            }
            turn += Orbit.kick * (Double(boops) - 1 + kick)
        }
        return Frame(turn: turn, dash: secs * -14, open: open, flash: flash)
    }

    /// How far the canvases reach past the stage, so documents at the ends
    /// of the orbit are not cut off.
    static let bleed: CGFloat = 40

    private func draw(_ ctx: GraphicsContext, _ canvas: CGSize, _ f: Frame, front: Bool) {
        var ctx = ctx
        ctx.translateBy(x: Self.bleed, y: Self.bleed)
        let size = CGSize(width: canvas.width - 2 * Self.bleed, height: canvas.height - 2 * Self.bleed)
        var o = Orbit(in: size)
        let grow = 0.6 + 0.4 * f.open
        o.rx *= grow
        o.ry *= grow

        // The track, dashed and quiet, flowing as time goes.
        var track = Path()
        var started = false
        let steps = 96
        for i in 0...steps {
            let a = Double(i) / Double(steps) * 2 * .pi
            let (pt, near) = o.at(a)
            if near != front { started = false; continue }
            if started { track.addLine(to: pt) } else { track.move(to: pt); started = true }
        }
        ctx.stroke(track, with: .color(p.lineStrong.opacity(f.open)),
                   style: StrokeStyle(lineWidth: 2, lineCap: .round, dash: [2, 7], dashPhase: f.dash))

        // The documents, smaller and paler on the far side.
        for s in Satellite.all {
            let a = s.phase * 2 * .pi + f.turn
            let (pt, near) = o.at(a)
            if near != front { continue }
            let depth = (sin(a) + 1) / 2
            let scale = (0.62 + 0.44 * depth) * (0.5 + 0.5 * f.open)
            drawDoc(ctx, at: pt, scale: scale, s, alpha: (0.7 + 0.3 * depth) * f.open)
        }

        // The icon's three strokes of surprise.
        if front && f.flash > 0.02 {
            let c = CGPoint(x: size.width * 0.73, y: size.height * 0.14)
            for deg in [-62.0, -30, 2] {
                let r = deg * .pi / 180
                let l0 = 10.0, l1 = 10 + 14 * f.flash
                var st = Path()
                st.move(to: CGPoint(x: c.x + cos(r) * l0, y: c.y + sin(r) * l0))
                st.addLine(to: CGPoint(x: c.x + cos(r) * l1, y: c.y + sin(r) * l1))
                ctx.stroke(st, with: .color(p.accent.opacity(f.flash)), style: StrokeStyle(lineWidth: 5, lineCap: .round))
            }
        }
    }

    /// A document sticker: a page with a folded corner in its kind's tint,
    /// outlined, its format written on it.
    private func drawDoc(_ ctx: GraphicsContext, at c: CGPoint, scale: Double, _ s: Satellite, alpha: Double) {
        let w = 46 * scale, h = 54 * scale, fold = 13 * scale, rr = 7 * scale
        let l = c.x - w / 2, t = c.y - h / 2
        let k = p.hue(s.kind)
        var page = Path()
        page.move(to: CGPoint(x: l + rr, y: t))
        page.addLine(to: CGPoint(x: l + w - fold, y: t))
        page.addLine(to: CGPoint(x: l + w, y: t + fold))
        page.addLine(to: CGPoint(x: l + w, y: t + h - rr))
        page.addQuadCurve(to: CGPoint(x: l + w - rr, y: t + h), control: CGPoint(x: l + w, y: t + h))
        page.addLine(to: CGPoint(x: l + rr, y: t + h))
        page.addQuadCurve(to: CGPoint(x: l, y: t + h - rr), control: CGPoint(x: l, y: t + h))
        page.addLine(to: CGPoint(x: l, y: t + rr))
        page.addQuadCurve(to: CGPoint(x: l + rr, y: t), control: CGPoint(x: l, y: t))
        page.closeSubpath()

        // The hard shadow, then the page, its outline and its fold.
        ctx.fill(page.offsetBy(dx: 0, dy: 3 * scale), with: .color(p.outline.opacity(alpha)))
        let face = p.dark ? p.surface.mixed(with: k, 0.25) : Color.white.mixed(with: k, 0.16)
        ctx.fill(page, with: .color(face.opacity(alpha)))
        ctx.stroke(page, with: .color(p.outline.opacity(alpha)), style: StrokeStyle(lineWidth: 2 * scale + 0.4, lineJoin: .round))
        var f = Path()
        f.move(to: CGPoint(x: l + w - fold, y: t))
        f.addLine(to: CGPoint(x: l + w - fold, y: t + fold - 2 * scale))
        f.addQuadCurve(to: CGPoint(x: l + w - fold + 2 * scale, y: t + fold), control: CGPoint(x: l + w - fold, y: t + fold))
        f.addLine(to: CGPoint(x: l + w, y: t + fold))
        f.closeSubpath()
        ctx.fill(f, with: .color(k.opacity(0.55 * alpha)))
        ctx.stroke(f, with: .color(p.outline.opacity(alpha)), style: StrokeStyle(lineWidth: 1.6 * scale + 0.3, lineJoin: .round))
        let label = Text(s.label)
            .font(.system(size: 10.5 * scale, weight: .heavy, design: .rounded))
            .foregroundStyle(k.mixed(with: p.dark ? .white : p.outline, p.dark ? 0.2 : 0.35).opacity(alpha))
        ctx.draw(label, at: CGPoint(x: c.x, y: c.y + 3 * scale))
    }
}

/// The gopher of the hero: it hops when tapped, as the desktop's does: up
/// for 170 ms, then a bouncing spring back down.
private struct OrbitGopher: View, Equatable {
    var side: CGFloat
    var hops: Int
    var playful: Bool

    struct Pose {
        var y: CGFloat = 0
        var rotate: Double = 0
        var scale: CGFloat = 1
    }

    var body: some View {
        Image("Mascot")
            .resizable()
            .scaledToFit()
            .frame(width: side, height: side)
            .keyframeAnimator(initialValue: Pose(), trigger: playful ? hops : 0) { content, v in
                content
                    .scaleEffect(v.scale, anchor: .bottom)
                    .rotationEffect(.degrees(v.rotate), anchor: .bottom)
                    .offset(y: v.y)
            } keyframes: { _ in
                let up = Spring(duration: 0.2), land = Spring(duration: 0.62, bounce: 0.55)
                KeyframeTrack(\.y) {
                    SpringKeyframe(-30, duration: 0.17, spring: up)
                    SpringKeyframe(0, duration: 0.73, spring: land)
                }
                KeyframeTrack(\.rotate) {
                    SpringKeyframe(-9, duration: 0.17, spring: up)
                    SpringKeyframe(0, duration: 0.73, spring: land)
                }
                KeyframeTrack(\.scale) {
                    SpringKeyframe(1.08, duration: 0.17, spring: up)
                    SpringKeyframe(1, duration: 0.73, spring: land)
                }
            }
    }
}

/// The tilted ellipse the documents ride, centered at 60% of the stage.
struct Orbit {
    static let period = 38.0
    static let kick = 1.4

    var cx, cy, rx, ry, tilt: Double

    init(in size: CGSize) {
        cx = size.width / 2
        cy = size.height * 0.6
        rx = size.width * 0.48
        ry = size.width * 0.14
        tilt = -9 * .pi / 180
    }

    /// The point at angle a, and whether it is on the near side.
    func at(_ a: Double) -> (CGPoint, Bool) {
        let ex = rx * cos(a), ey = ry * sin(a)
        let s = sin(tilt), c = cos(tilt)
        return (CGPoint(x: cx + ex * c - ey * s, y: cy + ex * s + ey * c), sin(a) > 0)
    }
}

struct Satellite {
    var label: String
    var kind: FileKind
    var phase: Double

    static let all = [
        Satellite(label: "JPG", kind: .image, phase: 0),
        Satellite(label: "MP3", kind: .audio, phase: 0.125),
        Satellite(label: "JSON", kind: .data, phase: 0.25),
        Satellite(label: "MD", kind: .document, phase: 0.375),
        Satellite(label: "PNG", kind: .image, phase: 0.5),
        Satellite(label: "FLAC", kind: .audio, phase: 0.625),
        Satellite(label: "YAML", kind: .data, phase: 0.75),
        Satellite(label: "HTML", kind: .document, phase: 0.875),
    ]
}

enum Ease {
    /// cubic-bezier(0.16, 1, 0.3, 1), solved for y at x.
    static func outExpoish(_ x: Double) -> Double {
        if x <= 0 { return 0 }
        if x >= 1 { return 1 }
        let x1 = 0.16, y1 = 1.0, x2 = 0.3, y2 = 1.0
        func bx(_ t: Double) -> Double { 3 * (1 - t) * (1 - t) * t * x1 + 3 * (1 - t) * t * t * x2 + t * t * t }
        func by(_ t: Double) -> Double { 3 * (1 - t) * (1 - t) * t * y1 + 3 * (1 - t) * t * t * y2 + t * t * t }
        var lo = 0.0, hi = 1.0
        for _ in 0..<30 {
            let mid = (lo + hi) / 2
            if bx(mid) < x { lo = mid } else { hi = mid }
        }
        return by((lo + hi) / 2)
    }
}

extension Color {
    /// This color moved a fraction t of the way to another, in sRGB.
    func mixed(with other: Color, _ t: Double) -> Color {
        var r1: CGFloat = 0, g1: CGFloat = 0, b1: CGFloat = 0, a1: CGFloat = 0
        var r2: CGFloat = 0, g2: CGFloat = 0, b2: CGFloat = 0, a2: CGFloat = 0
        UIColor(self).getRed(&r1, green: &g1, blue: &b1, alpha: &a1)
        UIColor(other).getRed(&r2, green: &g2, blue: &b2, alpha: &a2)
        let f = CGFloat(t)
        return Color(.sRGB, red: Double(r1 + (r2 - r1) * f), green: Double(g1 + (g2 - g1) * f),
                     blue: Double(b1 + (b2 - b1) * f), opacity: Double(a1 + (a2 - a1) * f))
    }
}
