import PhotosUI
import SwiftUI
import UniformTypeIdentifiers

/// A file from the photo library, copied into the app's inbox.
struct PickedFile: Transferable {
    let url: URL

    static var transferRepresentation: some TransferRepresentation {
        FileRepresentation(importedContentType: .movie) { try PickedFile(copying: $0.file) }
        FileRepresentation(importedContentType: .image) { try PickedFile(copying: $0.file) }
    }

    init(copying file: URL) throws {
        let dir = FileManager.default.temporaryDirectory
            .appendingPathComponent("Formigo/Inbox/\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        url = dir.appendingPathComponent(file.lastPathComponent)
        try FileManager.default.copyItem(at: file, to: url)
    }
}

struct ConvertScreen: View {
    @EnvironmentObject private var queue: Queue
    @EnvironmentObject private var settings: AppSettings
    @Environment(\.palette) private var p
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var picking = false
    @State private var importing = false
    @State private var picked: [PhotosPickerItem] = []
    @State private var optionsKind: FileKind?

    var body: some View {
        NavigationStack {
            ZStack {
                p.bg.ignoresSafeArea()
                if queue.items.isEmpty {
                    EmptyStage(pickPhotos: { picking = true }, pickFiles: { importing = true })
                        .transition(.opacity.combined(with: .scale(scale: 0.985)))
                } else {
                    QueueList(optionsKind: $optionsKind)
                        .transition(.opacity)
                }
            }
            .animation(settings.animation(Motion.settle, reduceMotion: reduceMotion), value: queue.items.isEmpty)
            .navigationTitle(queue.items.isEmpty ? "" : "Formigo")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                if !queue.items.isEmpty {
                    ToolbarItem(placement: .topBarLeading) {
                        Button("清空", systemImage: "trash") { withAnimation(settings.animation(Motion.settle, reduceMotion: reduceMotion)) { queue.clear() } }
                            .disabled(queue.running)
                    }
                    ToolbarItem(placement: .topBarTrailing) {
                        Menu {
                            Button("从相册选择", systemImage: "photo.on.rectangle") { picking = true }
                            Button("选择文件", systemImage: "folder") { importing = true }
                        } label: {
                            Label("添加", systemImage: "plus")
                        }
                    }
                }
            }
            .safeAreaInset(edge: .bottom) {
                if queue.importing {
                    HStack(spacing: 10) {
                        ProgressView().tint(p.accent)
                        Text("正在导入").font(.rounded(15, .bold)).foregroundStyle(p.ink)
                    }
                    .padding(.horizontal, 22).frame(height: 48)
                    .glassCapsule(fallback: p.surface)
                    .padding(.bottom, 8)
                    .transition(.blurReplace)
                } else if !queue.items.isEmpty {
                    RunBar().padding(.bottom, 8)
                }
            }
        }
        .photosPicker(isPresented: $picking, selection: $picked, matching: .any(of: [.images, .videos]),
                      preferredItemEncoding: .current)
        .onChange(of: picked) { _, items in
            guard !items.isEmpty else { return }
            Task { await load(items) }
        }
        .fileImporter(isPresented: $importing, allowedContentTypes: [.item], allowsMultipleSelection: true) { result in
            if case .success(let urls) = result {
                withAnimation(settings.animation(Motion.settle, reduceMotion: reduceMotion)) { _ = queue.add(urls) }
            }
        }
        .sheet(isPresented: $queue.showFresh) {
            ConvertToSheet().themed(p)
        }
        .sheet(item: $optionsKind) { kind in
            OptionsSheet(kind: kind).themed(p)
        }
    }

    private func load(_ items: [PhotosPickerItem]) async {
        queue.importing = true
        defer { queue.importing = false }
        var urls: [URL] = []
        for item in items {
            if let f = try? await item.loadTransferable(type: PickedFile.self) { urls.append(f.url) }
        }
        picked = []
        withAnimation(settings.animation(Motion.settle, reduceMotion: reduceMotion)) { _ = queue.add(urls, copy: false) }
    }
}

extension View {
    /// Sheets get the palette and accent of the window.
    func themed(_ p: Palette) -> some View {
        environment(\.palette, p)
            .tint(p.accent)
            .presentationBackground(p.bg)
    }
}

/// The empty window: the gopher, and where files come from.
struct EmptyStage: View {
    @EnvironmentObject private var settings: AppSettings
    @Environment(\.palette) private var p
    var pickPhotos: () -> Void
    var pickFiles: () -> Void

    var body: some View {
        VStack(spacing: 0) {
            Spacer()
            OrbitHero(width: 340)
                .padding(.bottom, 6)
            Text("Formigo")
                .font(.rounded(32, .heavy))
                .tracking(-0.5)
                .foregroundStyle(p.ink)
            Spacer()
            VStack(spacing: 14) {
                Button(action: pickPhotos) {
                    Label("从相册选择", systemImage: "photo.on.rectangle")
                        .font(.rounded(17, .heavy))
                        .foregroundStyle(p.accentInk)
                        .frame(maxWidth: .infinity, minHeight: 56)
                }
                .buttonStyle(StickerButtonStyle(fill: p.accent, haptics: true))
                Button(action: pickFiles) {
                    Label("选择文件", systemImage: "folder")
                        .font(.rounded(17, .heavy))
                        .foregroundStyle(p.ink)
                        .frame(maxWidth: .infinity, minHeight: 56)
                }
                .buttonStyle(StickerButtonStyle(fill: p.surface, haptics: true))
            }
            .frame(maxWidth: 420)
            .padding(.horizontal, 24)
            .padding(.bottom, 28)
        }
    }
}

/// The queue: files by kind, each kind under what it becomes.
struct QueueList: View {
    @EnvironmentObject private var queue: Queue
    @EnvironmentObject private var settings: AppSettings
    @Environment(\.palette) private var p
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @Binding var optionsKind: FileKind?

    var body: some View {
        List {
            if queue.isMixed && !queue.commonTargets.isEmpty {
                AllTargetBar()
                    .plainRow(top: 8, bottom: 6)
            }
            ForEach(queue.kinds) { kind in
                GroupHead(kind: kind, optionsKind: $optionsKind)
                    .plainRow(top: 14, bottom: 2, horizontal: 0)
                ForEach(queue.items(of: kind)) { item in
                    FileRow(item: item)
                        .plainRow(top: 4, bottom: 4)
                        .swipeActions(edge: .trailing, allowsFullSwipe: true) {
                            if !item.isWorking {
                                Button(role: .destructive) {
                                    withAnimation(settings.animation(Motion.settle, reduceMotion: reduceMotion)) { queue.remove(item.id) }
                                } label: { Label("移除", systemImage: "trash") }
                            }
                            if let out = item.output {
                                ShareLink(item: out) { Label("分享", systemImage: "square.and.arrow.up") }
                                    .tint(p.accent)
                            }
                        }
                }
            }
            Color.clear.frame(height: 12).plainRow(top: 0, bottom: 0)
        }
        .listStyle(.plain)
        .scrollContentBackground(.hidden)
        .background(p.bg)
        .animation(settings.animation(Motion.settle, reduceMotion: reduceMotion), value: queue.items.map(\.id))
    }
}

extension View {
    func plainRow(top: CGFloat, bottom: CGFloat, horizontal: CGFloat = 16) -> some View {
        listRowInsets(EdgeInsets(top: top, leading: horizontal, bottom: bottom, trailing: horizontal))
            .listRowBackground(Color.clear)
            .listRowSeparator(.hidden)
    }
}

/// A mixed batch's shared format: what every file can become.
struct AllTargetBar: View {
    @EnvironmentObject private var queue: Queue
    @Environment(\.palette) private var p

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(spacing: 10) {
                Image(systemName: "square.stack.3d.up.fill")
                    .font(.system(size: 14, weight: .bold))
                    .foregroundStyle(p.accent)
                    .frame(width: 30, height: 30)
                    .sticker(p.skyTint, radius: 10, lift: 2, width: 1.5)
                Text("全部转为")
                    .font(.rounded(16, .heavy))
                    .foregroundStyle(p.ink)
                Spacer()
            }
            FormatStrip(choices: queue.commonTargets, selected: queue.allTarget, group: "all") { id in
                queue.setAllTarget(queue.allTarget == id ? nil : id)
            }
            .padding(.horizontal, -16)
        }
    }
}

/// A kind's head: what was recognized, and the strip of what it becomes.
struct GroupHead: View {
    @EnvironmentObject private var queue: Queue
    @Environment(\.palette) private var p
    var kind: FileKind
    @Binding var optionsKind: FileKind?

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(spacing: 10) {
                KindBadge(kind: kind, size: 30)
                (Text(kind.title).font(.rounded(16, .heavy)).foregroundColor(p.ink)
                    + Text("  ·  \(queue.items(of: kind).count) 个  ·  \(queue.extensions(kind))")
                    .font(.system(size: 12.5, weight: .medium)).foregroundColor(p.faint))
                    .lineLimit(1)
                    .contentTransition(.numericText())
                Spacer(minLength: 4)
                if OptionSet.relevant(kind: kind, targets: queue.targetsInUse(kind)).any {
                    Button { optionsKind = kind } label: {
                        Label("选项", systemImage: "slider.horizontal.3")
                            .font(.system(size: 13, weight: .semibold))
                            .foregroundStyle(p.ink)
                            .padding(.horizontal, 12).frame(height: 32)
                            .background(Capsule().fill(p.surface))
                            .overlay(Capsule().strokeBorder(p.line, lineWidth: 1.5))
                    }
                    .buttonStyle(SoftPressStyle())
                    .disabled(queue.running)
                }
            }
            .padding(.horizontal, 16)
            FormatStrip(choices: queue.groupChoices(kind), selected: queue.allTarget == nil || !queue.groupChoices(kind).contains(queue.allTarget!) ? queue.groupTarget(kind) : nil, group: kind.rawValue) { id in
                queue.setGroupTarget(kind, id)
            }
            .disabled(queue.running)
        }
    }
}

/// A file's row: the file, where it goes, and how it is going.
struct FileRow: View {
    @EnvironmentObject private var queue: Queue
    @EnvironmentObject private var settings: AppSettings
    @Environment(\.palette) private var p
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    var item: QueueItem
    @State private var flash = false
    @State private var shake: CGFloat = 0

    var body: some View {
        let failed: Bool = { if case .failed = item.status { true } else { false } }()
        HStack(spacing: 12) {
            // The file's kind: a tile in its hue, with its extension.
            Text(item.ext.prefix(4))
                .font(.rounded(10.5, .heavy))
                .foregroundStyle(p.hue(item.kind))
                .frame(width: 42, height: 44)
                .background(RoundedRectangle(cornerRadius: 11, style: .continuous).fill(p.tint(item.kind)))
                .sticker(p.dark ? p.surface : .white, radius: 11, lift: 2, width: 1.5)
            VStack(alignment: .leading, spacing: 3) {
                Text(item.name)
                    .font(.system(size: 15, weight: .semibold))
                    .foregroundStyle(p.ink)
                    .lineLimit(1)
                    .truncationMode(.middle)
                status
            }
            Spacer(minLength: 4)
            if let out = item.output {
                ShareLink(item: out) {
                    Image(systemName: "square.and.arrow.up")
                        .font(.system(size: 15, weight: .semibold))
                        .foregroundStyle(p.accent)
                        .frame(width: 34, height: 34)
                        .background(Circle().fill(p.skyTint))
                }
                .buttonStyle(SoftPressStyle())
                .transition(.scale.combined(with: .opacity))
            }
            TargetChip(item: item)
        }
        .padding(.horizontal, 12)
        .frame(minHeight: 68)
        .background(RoundedRectangle(cornerRadius: 16, style: .continuous).fill(flash ? p.ok.opacity(0.18) : p.surface))
        .overlay(RoundedRectangle(cornerRadius: 16, style: .continuous).strokeBorder(failed ? p.bad : p.line, lineWidth: 1.5))
        .offset(x: shake)
        .onChange(of: item.output) { _, out in
            guard out != nil, settings.motion != "off" else { return }
            withAnimation(.easeOut(duration: 0.09)) { flash = true }
            withAnimation(.easeInOut(duration: 1).delay(0.1)) { flash = false }
        }
        .onChange(of: failed) { _, f in
            guard f, settings.playful(reduceMotion: reduceMotion) else { return }
            withAnimation(.spring(duration: 0.38, bounce: 0.7, blendDuration: 0)) { shake = -7 }
            withAnimation(.spring(duration: 0.38, bounce: 0.7).delay(0.08)) { shake = 0 }
        }
        .sensoryFeedback(.success, trigger: item.output != nil)
        .animation(settings.animation(Motion.pop, reduceMotion: reduceMotion), value: item.status)
    }

    @ViewBuilder
    private var status: some View {
        switch item.status {
        case .ready:
            Text(item.size.bytesText)
                .font(.system(size: 12)).monospacedDigit()
                .foregroundStyle(p.faint)
        case .working(let v):
            HStack(spacing: 8) {
                ProgressView(value: max(0.02, v))
                    .tint(p.accent)
                    .frame(maxWidth: 120)
                Text("\(Int(v * 100))%")
                    .font(.system(size: 11.5, weight: .semibold)).monospacedDigit()
                    .foregroundStyle(p.muted)
                    .contentTransition(.numericText())
            }
        case .done(let url):
            HStack(spacing: 4) {
                Image(systemName: "checkmark.circle.fill")
                Text(fileSize(url).bytesText)
            }
            .font(.system(size: 12, weight: .semibold)).monospacedDigit()
            .foregroundStyle(p.ok)
        case .failed(let msg):
            Label(msg, systemImage: "exclamationmark.circle.fill")
                .font(.system(size: 12, weight: .semibold))
                .foregroundStyle(p.bad)
                .lineLimit(1)
        }
    }

    private func fileSize(_ url: URL) -> Int64 {
        var isDir: ObjCBool = false
        FileManager.default.fileExists(atPath: url.path, isDirectory: &isDir)
        guard isDir.boolValue else {
            return Int64((try? url.resourceValues(forKeys: [.fileSizeKey]).fileSize) ?? 0)
        }
        let files = FileManager.default.enumerator(at: url, includingPropertiesForKeys: [.fileSizeKey])?.allObjects as? [URL] ?? []
        return files.reduce(0) { $0 + Int64((try? $1.resourceValues(forKeys: [.fileSizeKey]).fileSize) ?? 0) }
    }
}

/// A file's format, a chip whose menu chooses another for this file alone.
struct TargetChip: View {
    @EnvironmentObject private var queue: Queue
    @Environment(\.palette) private var p
    var item: QueueItem

    var body: some View {
        let current = queue.target(of: item)
        let own = item.override != nil
        let hue = p.hue(item.kind)
        Menu {
            Section {
                ForEach(item.targets, id: \.self) { id in
                    let f = Formats.format(id)
                    Button {
                        queue.setOverride(item.id, id)
                    } label: {
                        if own && id == current {
                            Label("\(f?.label ?? id) · \(f?.note ?? "")", systemImage: "checkmark")
                        } else {
                            Text("\(f?.label ?? id) · \(f?.note ?? "")")
                        }
                    }
                }
            }
            Section {
                Button {
                    queue.setOverride(item.id, nil)
                } label: {
                    if own { Text("跟随分组") } else { Label("跟随分组", systemImage: "checkmark") }
                }
            }
        } label: {
            HStack(spacing: 4) {
                Text(Formats.format(current)?.label ?? current.uppercased())
                    .font(.system(size: 12.5, weight: .heavy))
                    .tracking(0.3)
                    .contentTransition(.interpolate)
                Image(systemName: "chevron.up.chevron.down")
                    .font(.system(size: 9, weight: .bold))
            }
            .foregroundStyle(own ? (p.dark ? Color.black : .white) : hue)
            .padding(.horizontal, 10)
            .frame(height: 30)
            .background(Capsule().fill(own ? hue : p.tint(item.kind)))
            .overlay(Capsule().strokeBorder(hue, lineWidth: 1.5))
        }
        .disabled(queue.running)
        .accessibilityLabel("转换为 \(Formats.format(current)?.label ?? current)")
    }
}

/// The run's button above the tab bar, a glass capsule; once done, sharing
/// and saving everything.
struct RunBar: View {
    @EnvironmentObject private var queue: Queue
    @EnvironmentObject private var settings: AppSettings
    @Environment(\.palette) private var p
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        let pending = queue.pending.count
        ZStack {
            if queue.running {
                HStack(spacing: 10) {
                    ProgressView().tint(p.accent)
                    Text("正在转换  \(queue.doneCount)/\(queue.items.count)")
                        .font(.rounded(16, .heavy)).monospacedDigit()
                        .foregroundStyle(p.ink)
                        .contentTransition(.numericText())
                }
                .padding(.horizontal, 24).frame(height: 54)
                .glassCapsule(fallback: p.surface)
                .transition(.blurReplace)
            } else if pending > 0 {
                Button { queue.start() } label: {
                    Label("转换 \(pending) 个文件", systemImage: "arrow.triangle.2.circlepath")
                        .font(.rounded(17, .heavy))
                        .foregroundStyle(p.accentInk)
                        .padding(.horizontal, 28).frame(height: 54)
                        .contentShape(Capsule())
                }
                .buttonStyle(SoftPressStyle())
                .glassCapsule(fallback: p.accent, tint: p.accent, interactive: true)
                .transition(.blurReplace)
            } else {
                HStack(spacing: 10) {
                    ShareLink(items: queue.outputs) {
                        Label("全部分享", systemImage: "square.and.arrow.up")
                            .font(.rounded(16, .heavy))
                            .lineLimit(1)
                            .fixedSize()
                            .foregroundStyle(p.accentInk)
                            .padding(.horizontal, 22).frame(height: 54)
                            .contentShape(Capsule())
                    }
                    .buttonStyle(SoftPressStyle())
                    .glassCapsule(fallback: p.accent, tint: p.accent, interactive: true)
                    if !queue.photoOutputs.isEmpty {
                        Button { Task { await queue.saveToPhotos() } } label: {
                            Label("存到相册", systemImage: "photo.badge.arrow.down")
                                .font(.rounded(16, .heavy))
                                .lineLimit(1)
                                .fixedSize()
                                .foregroundStyle(p.ink)
                                .padding(.horizontal, 22).frame(height: 54)
                                .contentShape(Capsule())
                        }
                        .buttonStyle(SoftPressStyle())
                        .glassCapsule(fallback: p.surface, interactive: true)
                    }
                }
                .transition(.blurReplace)
            }
        }
        .animation(settings.animation(Motion.settle, reduceMotion: reduceMotion), value: queue.running)
        .animation(settings.animation(Motion.settle, reduceMotion: reduceMotion), value: pending == 0)
    }
}
