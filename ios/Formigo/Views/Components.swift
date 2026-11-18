import SwiftUI

/// A kind's badge: its symbol on a sticker in its hue.
struct KindBadge: View {
    @Environment(\.palette) private var p
    var kind: FileKind
    var size: CGFloat = 30

    var body: some View {
        Image(systemName: kind.symbol)
            .font(.system(size: size * 0.46, weight: .bold))
            .foregroundStyle(p.hue(kind))
            .frame(width: size, height: size)
            .background(RoundedRectangle(cornerRadius: size * 0.32, style: .continuous).fill(p.tint(kind)))
            .sticker(p.dark ? p.surface : .white, radius: size * 0.32, lift: 2, width: 1.5)
            .accessibilityHidden(true)
    }
}

/// A format to choose: its name, and what it is good for.
struct FormatTile: View {
    @EnvironmentObject private var settings: AppSettings
    @Environment(\.palette) private var p
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    var id: String
    var selected: Bool
    var namespace: Namespace.ID
    var group: String
    var width: CGFloat? = 74
    var action: () -> Void

    var body: some View {
        let f = Formats.format(id)
        Button(action: action) {
            VStack(spacing: 1) {
                Text(f?.label ?? id.uppercased())
                    .font(.rounded(15, .heavy))
                    .tracking(0.3)
                Text(f?.note ?? "")
                    .font(.system(size: 10.5, weight: .medium))
                    .opacity(0.75)
                    .lineLimit(1)
            }
            .foregroundStyle(selected ? p.accentInk : p.ink)
            .frame(width: width, height: 52)
            .frame(maxWidth: width == nil ? .infinity : nil)
            .background {
                let shape = RoundedRectangle(cornerRadius: 14, style: .continuous)
                if selected {
                    ZStack {
                        shape.fill(p.outline).offset(y: 3)
                        shape.fill(p.accent)
                        shape.strokeBorder(p.outline, lineWidth: 2)
                    }
                    .matchedGeometryEffect(id: "pick-\(group)", in: namespace)
                } else {
                    shape.fill(p.surface).overlay(shape.strokeBorder(p.lineStrong, lineWidth: 1.5))
                }
            }
            .contentShape(Rectangle())
        }
        .buttonStyle(SoftPressStyle())
        .accessibilityLabel(f.map { "\($0.label)，\($0.note)" } ?? id)
        .accessibilityAddTraits(selected ? .isSelected : [])
    }
}

/// A horizontal strip of formats, the chosen one's highlight sliding between them.
struct FormatStrip: View {
    @EnvironmentObject private var settings: AppSettings
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    var choices: [String]
    var selected: String?
    var group: String
    var onPick: (String) -> Void
    @Namespace private var ns

    var body: some View {
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(spacing: 8) {
                ForEach(choices, id: \.self) { id in
                    FormatTile(id: id, selected: id == selected, namespace: ns, group: group) {
                        withAnimation(settings.animation(Motion.snap, reduceMotion: reduceMotion)) { onPick(id) }
                    }
                }
            }
            .padding(.horizontal, 16)
            .padding(.top, 2)
            .padding(.bottom, 6)
        }
        .scrollClipDisabled()
        .sensoryFeedback(.selection, trigger: selected)
    }
}

/// A grid of formats, for the sheet.
struct FormatGrid: View {
    @EnvironmentObject private var settings: AppSettings
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    var choices: [String]
    var selected: String?
    var group: String
    var onPick: (String) -> Void
    @Namespace private var ns

    var body: some View {
        LazyVGrid(columns: Array(repeating: GridItem(.flexible(), spacing: 8), count: 4), spacing: 10) {
            ForEach(choices, id: \.self) { id in
                FormatTile(id: id, selected: id == selected, namespace: ns, group: group, width: nil) {
                    withAnimation(settings.animation(Motion.snap, reduceMotion: reduceMotion)) { onPick(id) }
                }
            }
        }
        .sensoryFeedback(.selection, trigger: selected)
    }
}

