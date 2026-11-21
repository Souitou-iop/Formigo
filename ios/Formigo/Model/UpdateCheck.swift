import Foundation

/// The app's releases on GitHub, which the settings' 检查更新 reads.
enum UpdateCheck {
    /// What a check made of the newest release.
    enum State {
        /// A newer version, by its number.
        case available(String)
        case upToDate
        /// GitHub could not be reached.
        case offline
    }

    /// The page of the newest release, where its IPA and its notes are.
    static let releases = URL(string: "https://github.com/Souitou-iop/Formigo/releases/latest")!
    private static let latest = URL(string: "https://api.github.com/repos/Souitou-iop/Formigo/releases/latest")!

    /// Asks GitHub for the newest release, beside the version running.
    static func check(running: String) async -> State {
        var request = URLRequest(url: latest)
        request.timeoutInterval = 10
        request.setValue("application/vnd.github+json", forHTTPHeaderField: "Accept")
        request.setValue("Formigo", forHTTPHeaderField: "User-Agent")
        guard let (data, response) = try? await URLSession.shared.data(for: request),
              (response as? HTTPURLResponse)?.statusCode == 200
        else { return .offline }
        let tag = (try? JSONSerialization.jsonObject(with: data) as? [String: Any] ?? [:])?["tag_name"] as? String ?? ""
        guard !tag.isEmpty else { return .offline }
        return newer(tag, running: running) ? .available(String(tag.dropFirst(tag.hasPrefix("v") ? 1 : 0))) : .upToDate
    }

    /// Whether release runs ahead of running: the first version number that differs.
    static func newer(_ release: String, running: String) -> Bool {
        let a = release.drop(while: { $0 == "v" }).split(separator: ".").map { Int($0) ?? 0 }
        let b = running.split(separator: ".").map { Int($0) ?? 0 }
        for i in 0..<max(a.count, b.count) {
            let x = i < a.count ? a[i] : 0
            let y = i < b.count ? b[i] : 0
            if x != y { return x > y }
        }
        return false
    }
}
