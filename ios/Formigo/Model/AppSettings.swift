import SwiftUI

/// What Formigo remembers between runs, in UserDefaults through @AppStorage.
/// Each kind's default format is kept as JSON.
final class AppSettings: ObservableObject {
    @AppStorage("appearance") var appearance = "system" // system, light, dark
    @AppStorage("oled") var oled = false
    @AppStorage("accent") var accent = Accent.defaultID
    @AppStorage("motion") var motion = "full" // full, calm, off
    @AppStorage("mascot") var mascot = true
    @AppStorage("targets") var targetsJSON = ""
    @AppStorage("workers") var workers = AppSettings.defaultWorkers
    @AppStorage("suffix") var suffix = ""
    @AppStorage("notify") var notify = true

    @AppStorage("quality") var quality = 0.85
    @AppStorage("scale") var scale = 1.0
    @AppStorage("depth") var bitDepth = 16
    @AppStorage("mono") var mono = false
    @AppStorage("indent") var indent = 2
    @AppStorage("resolution") var resolution = "original"

    static var defaultWorkers: Int { max(1, min(4, ProcessInfo.processInfo.activeProcessorCount / 2)) }

    var colorScheme: ColorScheme? {
        switch appearance {
        case "light": .light
        case "dark": .dark
        default: nil
        }
    }

    var options: ConvertOptions {
        ConvertOptions(quality: quality, scale: scale, bitDepth: bitDepth, mono: mono, indent: indent, videoResolution: resolution)
    }

    /// The format a kind's files go to unless the user chooses.
    func defaultTarget(_ kind: FileKind) -> String {
        let stored = (try? JSONDecoder().decode([String: String].self, from: Data(targetsJSON.utf8))) ?? [:]
        return stored[kind.rawValue] ?? kind.defaultTarget
    }

    func setDefaultTarget(_ kind: FileKind, _ id: String) {
        var stored = (try? JSONDecoder().decode([String: String].self, from: Data(targetsJSON.utf8))) ?? [:]
        stored[kind.rawValue] = id
        targetsJSON = String(decoding: (try? JSONEncoder().encode(stored)) ?? Data(), as: UTF8.self)
    }

    func reset() {
        let d = UserDefaults.standard
        for k in ["appearance", "oled", "accent", "motion", "mascot", "targets", "workers", "suffix", "notify",
                  "quality", "scale", "depth", "mono", "indent", "resolution"] {
            d.removeObject(forKey: k)
        }
        objectWillChange.send()
    }

    // MARK: Motion

    /// An animation as the motion setting and the system allow: springs in
    /// full, a short fade in calm or with reduced motion, none when off.
    func animation(_ spring: Animation, reduceMotion: Bool) -> Animation? {
        switch motion {
        case "off": return nil
        case "calm": return .easeOut(duration: 0.2)
        default: return reduceMotion ? .easeOut(duration: 0.2) : spring
        }
    }

    /// Whether decorative motion plays: the gopher's hop.
    func playful(reduceMotion: Bool) -> Bool { motion != "off" && !reduceMotion }
}
