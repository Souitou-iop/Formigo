import SwiftUI

struct SettingsScreen: View {
    @EnvironmentObject private var settings: AppSettings
    @Environment(\.palette) private var p
    @State private var confirmReset = false

    var body: some View {
        NavigationStack {
            Form {
                Section("外观") {
                    Picker("外观", selection: $settings.appearance) {
                        Text("跟随系统").tag("system")
                        Text("浅色").tag("light")
                        Text("深色").tag("dark")
                    }
                    .pickerStyle(.segmented)
                    .labelsHidden()
                    Toggle(isOn: $settings.oled) {
                        Label("OLED 纯黑", systemImage: "moon.stars.fill")
                    }
                    accents
                }
                .listRowBackground(p.surface)

                Section("动画") {
                    Picker("动画", selection: $settings.motion) {
                        Text("完整").tag("full")
                        Text("安静").tag("calm")
                        Text("关闭").tag("off")
                    }
                    .pickerStyle(.segmented)
                    .labelsHidden()
                    Toggle(isOn: $settings.mascot) {
                        Label {
                            Text("小地鼠")
                        } icon: {
                            MascotIcon()
                        }
                    }
                }
                .listRowBackground(p.surface)

                Section("默认格式") {
                    ForEach(FileKind.allCases) { kind in
                        Picker(selection: Binding(get: { settings.defaultTarget(kind) },
                                                  set: { settings.setDefaultTarget(kind, $0) })) {
                            ForEach(kindTargets(kind), id: \.self) { id in
                                Text(Formats.format(id)?.label ?? id).tag(id)
                            }
                        } label: {
                            Label {
                                Text(kind.title)
                            } icon: {
                                Image(systemName: kind.symbol).foregroundStyle(p.hue(kind))
                            }
                        }
                        .tint(p.accent)
                        // The menu keeps the tint it was made with: make it again with the theme.
                        .id("\(kind.rawValue)-\(settings.accent)")
                    }
                }
                .listRowBackground(p.surface)

                Section("转换") {
                    Stepper(value: $settings.workers, in: 1...8) {
                        LabeledContent("同时转换", value: "\(settings.workers)")
                    }
                    LabeledContent("文件名后缀") {
                        TextField("-converted", text: $settings.suffix)
                            .multilineTextAlignment(.trailing)
                            .autocorrectionDisabled()
                            .textInputAutocapitalization(.never)
                    }
                    Toggle("完成后通知", isOn: $settings.notify)
                        .onChange(of: settings.notify) { _, on in if on { Queue.requestNotifications() } }
                }
                .listRowBackground(p.surface)

                Section {
                    Button("恢复默认设置", role: .destructive) { confirmReset = true }
                } footer: {
                    Text("Formigo \(Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "")")
                        .frame(maxWidth: .infinity)
                        .padding(.top, 12)
                }
                .listRowBackground(p.surface)
            }
            .scrollContentBackground(.hidden)
            .background(p.bg.ignoresSafeArea())
            .navigationTitle("设置")
            .confirmationDialog("恢复默认设置？", isPresented: $confirmReset, titleVisibility: .visible) {
                Button("恢复默认", role: .destructive) { settings.reset() }
            }
        }
    }

    /// The accent colors, six stickers.
    private var accents: some View {
        HStack {
            Text("主题色")
            Spacer()
            HStack(spacing: 10) {
                ForEach(Accent.all) { a in
                    let on = Accent.by(settings.accent).id == a.id
                    Button {
                        settings.accent = a.id
                    } label: {
                        Circle()
                            .fill(Color(hex: p.dark ? a.dark : a.light))
                            .frame(width: 26, height: 26)
                            .overlay(Circle().strokeBorder(p.outline, lineWidth: 2))
                            .overlay {
                                if on {
                                    Image(systemName: "checkmark").font(.system(size: 11, weight: .heavy))
                                        .foregroundStyle(Accent.ink(on: p.dark ? a.dark : a.light))
                                }
                            }
                            .padding(3)
                            .overlay(Circle().strokeBorder(on ? Color(hex: p.dark ? a.dark : a.light) : .clear, lineWidth: 2))
                            .scaleEffect(on ? 1.06 : 1)
                            .animation(Motion.pop, value: on)
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel(a.name)
                    .accessibilityAddTraits(on ? .isSelected : [])
                }
            }
        }
    }

    /// Every target a kind's files may go to.
    private func kindTargets(_ kind: FileKind) -> [String] {
        var out: [String] = []
        for s in Set(Formats.sources.values.filter { $0.kind == kind }.map(\.id)).sorted() {
            for t in Formats.targets(for: Formats.Source(kind: kind, id: s)) where !out.contains(t) { out.append(t) }
        }
        let order = Formats.all.map(\.id)
        return out.sorted { order.firstIndex(of: $0)! < order.firstIndex(of: $1)! }
    }
}

/// The gopher's face, cut from the app's own mascot, as a small square icon.
struct MascotIcon: View {
    var side: CGFloat = 28

    var body: some View {
        // The head and shoulders sit in a 270 px square near the top of the
        // 512 px picture, centred on (250, 195).
        let scale = side / 270
        Image("Mascot")
            .resizable()
            .frame(width: 512 * scale, height: 512 * scale)
            .offset(x: (256 - 250) * scale, y: (256 - 195) * scale)
            .frame(width: side, height: side)
            .clipShape(RoundedRectangle(cornerRadius: side * 0.26, style: .continuous))
            .accessibilityHidden(true)
    }
}
