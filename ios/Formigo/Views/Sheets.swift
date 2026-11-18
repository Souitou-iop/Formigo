import SwiftUI

/// What was recognized in the files just added, by kind, each with the
/// formats it can become, a tap away from converting.
struct ConvertToSheet: View {
    @EnvironmentObject private var queue: Queue
    @Environment(\.palette) private var p
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        let kinds = queue.freshKinds.filter { !queue.items(of: $0).isEmpty }
        VStack(alignment: .leading, spacing: 0) {
            Text("转换为")
                .font(.rounded(26, .heavy))
                .tracking(-0.3)
                .foregroundStyle(p.ink)
                .padding(.horizontal, 20)
                .padding(.top, 26)
                .padding(.bottom, 8)
            ScrollView {
                VStack(alignment: .leading, spacing: 24) {
                    if kinds.count > 1 && !queue.commonTargets.isEmpty {
                        VStack(alignment: .leading, spacing: 10) {
                            HStack(spacing: 10) {
                                Image(systemName: "square.stack.3d.up.fill")
                                    .font(.system(size: 12, weight: .bold))
                                    .foregroundStyle(p.accent)
                                    .frame(width: 26, height: 26)
                                    .sticker(p.skyTint, radius: 9, lift: 2, width: 1.5)
                                Text("全部转为").font(.rounded(15, .heavy)).foregroundStyle(p.ink)
                                Spacer()
                                Text("\(queue.items.count) 个").font(.system(size: 12)).foregroundStyle(p.faint)
                            }
                            FormatGrid(choices: queue.commonTargets, selected: queue.allTarget, group: "all") { id in
                                queue.setAllTarget(queue.allTarget == id ? nil : id)
                            }
                        }
                    }
                    ForEach(kinds) { kind in
                        VStack(alignment: .leading, spacing: 10) {
                            HStack(spacing: 10) {
                                KindBadge(kind: kind, size: 26)
                                Text(kind.title).font(.rounded(15, .heavy)).foregroundStyle(p.ink)
                                Spacer()
                                Text("\(queue.items(of: kind).count) 个 · \(queue.extensions(kind))")
                                    .font(.system(size: 12)).monospacedDigit()
                                    .foregroundStyle(p.faint)
                                    .lineLimit(1)
                            }
                            let choices = queue.groupChoices(kind)
                            let all = queue.allTarget.flatMap { choices.contains($0) ? $0 : nil }
                            FormatGrid(choices: choices, selected: all == nil ? queue.groupTarget(kind) : nil, group: kind.rawValue) { id in
                                queue.setGroupTarget(kind, id)
                            }
                        }
                    }
                }
                .padding(.horizontal, 20)
                .padding(.top, 8)
                .padding(.bottom, 16)
            }
            HStack(spacing: 10) {
                Button { dismiss() } label: {
                    Text("稍后")
                        .font(.rounded(16, .bold))
                        .foregroundStyle(p.ink)
                        .padding(.horizontal, 22).frame(height: 52)
                }
                .buttonStyle(StickerButtonStyle(fill: p.surface, radius: 15, lift: 3))
                Button {
                    dismiss()
                    queue.start()
                } label: {
                    Label("转换 \(queue.pending.count) 个文件", systemImage: "arrow.triangle.2.circlepath")
                        .font(.rounded(16, .heavy))
                        .foregroundStyle(p.accentInk)
                        .frame(maxWidth: .infinity).frame(height: 52)
                }
                .buttonStyle(StickerButtonStyle(fill: p.accent, radius: 15, lift: 4))
                .disabled(queue.running)
            }
            .padding(.horizontal, 20)
            .padding(.top, 8)
            .padding(.bottom, 14)
        }
        .presentationDetents(detents(kinds))
        .presentationDragIndicator(.visible)
    }

    private func detents(_ kinds: [FileKind]) -> Set<PresentationDetent> {
        var h: CGFloat = 26 + 46 + 16 + 82
        for (i, k) in kinds.enumerated() {
            let rows = CGFloat((queue.groupChoices(k).count + 3) / 4)
            h += 36 + rows * 52 + (rows - 1) * 10 + (i > 0 ? 24 : 0)
        }
        if kinds.count > 1 && !queue.commonTargets.isEmpty {
            h += 36 + CGFloat((queue.commonTargets.count + 3) / 4) * 62 + 24
        }
        return [.height(min(h + 20, 720)), .large]
    }
}

/// The options of the formats a kind's files go to: only those that matter.
struct OptionsSheet: View {
    @EnvironmentObject private var queue: Queue
    @EnvironmentObject private var settings: AppSettings
    @Environment(\.palette) private var p
    @Environment(\.dismiss) private var dismiss
    var kind: FileKind

    var body: some View {
        let targets = queue.targetsInUse(kind)
        let o = OptionSet.relevant(kind: kind, targets: targets)
        let labels = targets.compactMap { Formats.format($0)?.label }.sorted().joined(separator: "、")
        NavigationStack {
            Form {
                if o.quality {
                    Section("画质") {
                        HStack {
                            Slider(value: $settings.quality, in: 0.1...1, step: 0.05)
                            Text("\(Int(settings.quality * 100))")
                                .font(.system(size: 15, weight: .semibold)).monospacedDigit()
                                .foregroundStyle(p.muted)
                                .frame(width: 38, alignment: .trailing)
                        }
                    }
                    .listRowBackground(p.surface)
                }
                if o.scale {
                    Section("尺寸") {
                        Picker("尺寸", selection: $settings.scale) {
                            Text("原始").tag(1.0)
                            Text("75%").tag(0.75)
                            Text("50%").tag(0.5)
                            Text("25%").tag(0.25)
                        }
                        .pickerStyle(.segmented)
                        .labelsHidden()
                    }
                    .listRowBackground(p.surface)
                }
                if o.resolution {
                    Section("分辨率") {
                        Picker("分辨率", selection: $settings.resolution) {
                            Text("原始").tag("original")
                            Text("1080p").tag("1080")
                            Text("720p").tag("720")
                        }
                        .pickerStyle(.segmented)
                        .labelsHidden()
                    }
                    .listRowBackground(p.surface)
                }
                if o.bitDepth || o.mono {
                    Section("声音") {
                        if o.bitDepth {
                            Picker("位深", selection: $settings.bitDepth) {
                                Text("16 位").tag(16)
                                Text("24 位").tag(24)
                            }
                        }
                        if o.mono { Toggle("单声道", isOn: $settings.mono) }
                    }
                    .listRowBackground(p.surface)
                }
                if o.indent {
                    Section("缩进") {
                        Picker("缩进", selection: $settings.indent) {
                            Text("2 空格").tag(2)
                            Text("4 空格").tag(4)
                            Text("紧凑").tag(0)
                        }
                        .pickerStyle(.segmented)
                        .labelsHidden()
                    }
                    .listRowBackground(p.surface)
                }
            }
            .scrollContentBackground(.hidden)
            .background(p.bg)
            .navigationTitle("\(kind.title)选项")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    Text("→ \(labels)").font(.system(size: 13, weight: .bold)).foregroundStyle(p.muted)
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("完成", systemImage: "checkmark") { dismiss() }
                }
            }
        }
        .presentationDetents([.medium, .large])
        .onChange(of: settings.options) { queue.invalidate(kind) }
    }
}
