import Foundation
import Photos
import SwiftUI
import UserNotifications

struct QueueItem: Identifiable, Equatable {
    enum Status: Equatable {
        case ready
        case working(Double)
        case done(URL)
        case failed(String)
    }

    let id = UUID()
    let url: URL
    let source: Formats.Source
    let size: Int64
    /// What this file can become.
    let targets: [String]
    /// A format chosen for this file alone; nil follows its group.
    var override: String?
    var status: Status = .ready

    var name: String { url.lastPathComponent }
    var ext: String { url.pathExtension.uppercased() }
    var kind: FileKind { source.kind }
    var output: URL? { if case .done(let u) = status { u } else { nil } }
    var isWorking: Bool { if case .working = status { true } else { false } }
}

/// The files to convert, grouped by kind, and their run.
@MainActor
final class Queue: ObservableObject {
    @Published var items: [QueueItem] = []
    /// Each kind's chosen format.
    @Published var groupTargets: [FileKind: String] = [:]
    /// A format for every file of a mixed batch, where it applies.
    @Published var allTarget: String?
    @Published var running = false
    /// Kinds just added, for the "转换为" sheet.
    @Published var freshKinds: [FileKind] = []
    @Published var showFresh = false
    @Published var toast: String?
    /// Files from the photo library being loaded.
    @Published var importing = false
    /// Bumped as a run finishes, for the gopher's cheer.
    @Published var finishedRuns = 0

    let settings: AppSettings
    private let inbox: URL
    private let outbox: URL

    init(settings: AppSettings) {
        self.settings = settings
        let base = FileManager.default.temporaryDirectory.appendingPathComponent("Formigo", isDirectory: true)
        inbox = base.appendingPathComponent("Inbox", isDirectory: true)
        outbox = base.appendingPathComponent("Output", isDirectory: true)
        try? FileManager.default.createDirectory(at: inbox, withIntermediateDirectories: true)
        try? FileManager.default.createDirectory(at: outbox, withIntermediateDirectories: true)
    }

    // MARK: Groups

    /// The kinds in the queue, in the order they came.
    var kinds: [FileKind] {
        var seen: [FileKind] = []
        for it in items where !seen.contains(it.kind) { seen.append(it.kind) }
        return seen
    }

    func items(of kind: FileKind) -> [QueueItem] { items.filter { $0.kind == kind } }

    /// What a kind's files can become: every target of any of them, in order.
    func groupChoices(_ kind: FileKind) -> [String] {
        var out: [String] = []
        for it in items where it.kind == kind {
            for t in it.targets where !out.contains(t) { out.append(t) }
        }
        return out.sorted { Formats.order($0) < Formats.order($1) }
    }

    /// The upper-case extensions of a kind's files, the first few.
    func extensions(_ kind: FileKind) -> String {
        var exts: [String] = []
        for it in items where it.kind == kind && !exts.contains(it.ext) { exts.append(it.ext) }
        if exts.count > 3 { exts = Array(exts.prefix(3)) + ["…"] }
        return exts.joined(separator: "、")
    }

    var isMixed: Bool { kinds.count > 1 }

    /// Targets every file of the queue can become, for a mixed batch.
    var commonTargets: [String] {
        guard var common = items.first?.targets else { return [] }
        for it in items.dropFirst() { common = common.filter(it.targets.contains) }
        return common
    }

    /// The group's format as shown: the batch's when it applies, else the
    /// kind's own.
    func groupTarget(_ kind: FileKind) -> String {
        let choices = groupChoices(kind)
        if let a = allTarget, choices.contains(a) { return a }
        if let t = groupTargets[kind], choices.contains(t) { return t }
        let d = settings.defaultTarget(kind)
        return choices.contains(d) ? d : (choices.first ?? d)
    }

    /// The format a file goes to.
    func target(of item: QueueItem) -> String {
        if let o = item.override, item.targets.contains(o) { return o }
        if let a = allTarget, item.targets.contains(a) { return a }
        let g = groupTarget(item.kind)
        if item.targets.contains(g) { return g }
        let d = settings.defaultTarget(item.kind)
        return item.targets.contains(d) ? d : (item.targets.first ?? d)
    }

    func targetsInUse(_ kind: FileKind) -> Set<String> {
        Set(items(of: kind).map(target(of:)))
    }

    func setGroupTarget(_ kind: FileKind, _ id: String) {
        // Choosing one group's format ends the batch-wide choice; the other
        // groups keep what it gave them.
        if let a = allTarget {
            for k in kinds where k != kind && groupChoices(k).contains(a) { groupTargets[k] = a }
            allTarget = nil
        }
        groupTargets[kind] = id
        retarget { $0.kind == kind && $0.override == nil }
    }

    func setAllTarget(_ id: String?) {
        allTarget = id
        retarget { $0.override == nil }
    }

    func setOverride(_ itemID: UUID, _ id: String?) {
        guard let i = items.firstIndex(where: { $0.id == itemID }) else { return }
        items[i].override = id
        if items[i].output != nil || { if case .failed = items[i].status { true } else { false } }() {
            items[i].status = .ready
        }
    }

    /// A kind's files convert again after its options change.
    func invalidate(_ kind: FileKind) {
        retarget { $0.kind == kind }
    }

    /// Files done or failed whose format changed convert again.
    private func retarget(_ match: (QueueItem) -> Bool) {
        guard !running else { return }
        for i in items.indices where match(items[i]) {
            if case .working = items[i].status { continue }
            items[i].status = .ready
        }
    }

    // MARK: Adding

    /// Adds files, copied into the app's inbox. Returns how many were taken.
    @discardableResult
    func add(_ urls: [URL], copy: Bool = true) -> Int {
        var added = 0
        var newKinds: [FileKind] = []
        let known = Set(kinds)
        for url in urls {
            guard let source = Formats.source(for: url) else { continue }
            var local = url
            if copy {
                let scoped = url.startAccessingSecurityScopedResource()
                defer { if scoped { url.stopAccessingSecurityScopedResource() } }
                let dir = inbox.appendingPathComponent(UUID().uuidString, isDirectory: true)
                try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
                local = dir.appendingPathComponent(url.lastPathComponent)
                do { try FileManager.default.copyItem(at: url, to: local) } catch { continue }
            }
            let size = (try? local.resourceValues(forKeys: [.fileSizeKey]).fileSize).map(Int64.init) ?? 0
            items.append(QueueItem(url: local, source: source, size: size, targets: Formats.targets(for: source)))
            if !known.contains(source.kind) && !newKinds.contains(source.kind) { newKinds.append(source.kind) }
            added += 1
        }
        if added < urls.count { show(added == 0 ? "这些文件的格式暂不支持" : "已跳过不支持的文件") }
        if !newKinds.isEmpty {
            freshKinds = newKinds
            showFresh = true
        }
        return added
    }

    /// A place in the inbox for a file coming from the photo library.
    func inboxURL(named name: String) -> URL {
        let dir = inbox.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir.appendingPathComponent(name)
    }

    func remove(_ id: UUID) {
        guard let it = items.first(where: { $0.id == id }), !it.isWorking else { return }
        items.removeAll { $0.id == id }
        try? FileManager.default.removeItem(at: it.url.deletingLastPathComponent())
        if items.isEmpty { allTarget = nil }
    }

    func clear() {
        guard !running else { return }
        for it in items { try? FileManager.default.removeItem(at: it.url.deletingLastPathComponent()) }
        items.removeAll()
        allTarget = nil
        try? FileManager.default.removeItem(at: outbox)
        try? FileManager.default.createDirectory(at: outbox, withIntermediateDirectories: true)
    }

    // MARK: Running

    var pending: [QueueItem] {
        items.filter { if case .done = $0.status { false } else { true } }
    }

    var outputs: [URL] { items.compactMap(\.output) }

    var doneCount: Int { outputs.count }

    var progress: Double {
        guard !items.isEmpty else { return 0 }
        let total = items.reduce(0.0) { sum, it in
            switch it.status {
            case .done, .failed: sum + 1
            case .working(let p): sum + p
            case .ready: sum
            }
        }
        return total / Double(items.count)
    }

    func start() {
        guard !running else { return }
        let ids = pending.map(\.id)
        guard !ids.isEmpty else { return }
        running = true
        let workers = max(1, min(8, settings.workers))
        // Keep going for a while if the app is sent to the background.
        var bg = UIBackgroundTaskIdentifier.invalid
        bg = UIApplication.shared.beginBackgroundTask(withName: "convert") { UIApplication.shared.endBackgroundTask(bg) }
        Task {
            defer { if bg != .invalid { UIApplication.shared.endBackgroundTask(bg) } }
            await withTaskGroup(of: Void.self) { group in
                var next = ids.makeIterator()
                for _ in 0..<workers {
                    if let id = next.next() { group.addTask { await self.convert(id) } }
                }
                while await group.next() != nil {
                    if let id = next.next() { group.addTask { await self.convert(id) } }
                }
            }
            running = false
            finishedRuns += 1
            notifyDone()
        }
    }

    private func convert(_ id: UUID) async {
        guard let i = items.firstIndex(where: { $0.id == id }) else { return }
        let item = items[i]
        let target = target(of: item)
        let options = settings.options
        let base = item.url.deletingPathExtension().lastPathComponent + settings.suffix
        let dir = outbox.appendingPathComponent(item.id.uuidString, isDirectory: true)
        try? FileManager.default.removeItem(at: dir)
        update(id) { $0.status = .working(0) }
        do {
            let out = try await Engine.convert(item.url, to: target, options: options, into: dir, baseName: base) { p in
                Task { @MainActor in self.update(id) { if case .working = $0.status { $0.status = .working(p) } } }
            }
            update(id) { $0.status = .done(out) }
        } catch {
            update(id) { $0.status = .failed(error.localizedDescription) }
        }
    }

    private func update(_ id: UUID, _ change: (inout QueueItem) -> Void) {
        guard let i = items.firstIndex(where: { $0.id == id }) else { return }
        change(&items[i])
    }

    // MARK: After

    func show(_ message: String) {
        toast = message
        Task {
            try? await Task.sleep(nanoseconds: 2_200_000_000)
            if toast == message { toast = nil }
        }
    }

    /// Outputs that the Photos library takes.
    var photoOutputs: [URL] {
        outputs.filter { Formats.isPhotoLibraryImage($0.pathExtension) || Formats.isPhotoLibraryVideo($0.pathExtension) }
    }

    func saveToPhotos() async {
        let urls = photoOutputs
        guard !urls.isEmpty else { return }
        let status = await PHPhotoLibrary.requestAuthorization(for: .addOnly)
        guard status == .authorized || status == .limited else {
            show("没有相册权限")
            return
        }
        do {
            try await PHPhotoLibrary.shared().performChanges {
                for u in urls {
                    if Formats.isPhotoLibraryVideo(u.pathExtension) {
                        PHAssetChangeRequest.creationRequestForAssetFromVideo(atFileURL: u)
                    } else {
                        PHAssetChangeRequest.creationRequestForAssetFromImage(atFileURL: u)
                    }
                }
            }
            show("已存到相册 \(urls.count) 项")
        } catch {
            show("存到相册失败")
        }
    }

    private func notifyDone() {
        guard settings.notify, UIApplication.shared.applicationState != .active else { return }
        let failed = items.filter { if case .failed = $0.status { true } else { false } }.count
        let content = UNMutableNotificationContent()
        content.title = "Formigo"
        content.body = failed > 0 ? "已转换 \(doneCount) 个文件，\(failed) 个失败" : "已转换 \(doneCount) 个文件"
        content.sound = .default
        UNUserNotificationCenter.current().add(UNNotificationRequest(identifier: UUID().uuidString, content: content, trigger: nil))
    }

    static func requestNotifications() {
        UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound]) { _, _ in }
    }
}

extension Int64 {
    var bytesText: String { ByteCountFormatter.string(fromByteCount: self, countStyle: .file) }
}
