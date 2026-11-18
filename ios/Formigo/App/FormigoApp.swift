import SwiftUI

@main
struct FormigoApp: App {
    @StateObject private var settings: AppSettings
    @StateObject private var queue: Queue

    init() {
        let s = AppSettings()
        _settings = StateObject(wrappedValue: s)
        _queue = StateObject(wrappedValue: Queue(settings: s))
    }

    var body: some Scene {
        WindowGroup {
            RootView()
                .environmentObject(settings)
                .environmentObject(queue)
        }
    }
}

struct RootView: View {
    @EnvironmentObject private var settings: AppSettings
    @Environment(\.colorScheme) private var system

    var body: some View {
        ThemedRoot()
            .preferredColorScheme(settings.colorScheme)
    }
}

/// Everything below the color scheme, so the palette follows it.
private struct ThemedRoot: View {
    @EnvironmentObject private var settings: AppSettings
    @EnvironmentObject private var queue: Queue
    @Environment(\.colorScheme) private var scheme
    @State private var tab = 0

    var body: some View {
        let p = Palette.make(dark: scheme == .dark, oled: settings.oled, accent: Accent.by(settings.accent))
        TabView(selection: $tab) {
            ConvertScreen()
                .tabItem { Label("转换", systemImage: "arrow.triangle.2.circlepath") }
                .tag(0)
            SettingsScreen()
                .tabItem { Label("设置", systemImage: "gearshape") }
                .tag(1)
        }
        .environment(\.palette, p)
        .tint(p.accent)
        .transaction { t in
            if settings.motion == "off" { t.disablesAnimations = true; t.animation = nil }
        }
        .overlay(alignment: .top) { ToastView(text: queue.toast).environment(\.palette, p) }
        .onAppear(perform: setTabBarAppearance)
        .onChange(of: settings.oled) { setTabBarAppearance() }
        .onChange(of: scheme) { setTabBarAppearance() }
    }

    /// Before iOS 26 the tab bar is opaque: give it the palette's background,
    /// so OLED black reaches the bottom of the screen. From iOS 26 the bar is
    /// Liquid Glass and is left as the system draws it.
    private func setTabBarAppearance() {
        if #available(iOS 26, *) { return }
        let a = UITabBarAppearance()
        a.configureWithDefaultBackground()
        if scheme == .dark && settings.oled { a.configureWithOpaqueBackground(); a.backgroundColor = .black }
        UITabBar.appearance().standardAppearance = a
        UITabBar.appearance().scrollEdgeAppearance = a
    }
}

struct ToastView: View {
    @Environment(\.palette) private var p
    var text: String?

    var body: some View {
        ZStack {
            if let text {
                Text(text)
                    .font(.rounded(14, .semibold))
                    .foregroundStyle(p.ink)
                    .padding(.horizontal, 16).padding(.vertical, 10)
                    .glassCapsule(fallback: p.surface)
                    .transition(.move(edge: .top).combined(with: .opacity))
            }
        }
        .padding(.top, 8)
        .animation(Motion.snap, value: text)
    }
}

extension View {
    /// Liquid Glass where the system has it, a material capsule before.
    @ViewBuilder
    func glassCapsule(fallback: Color, tint: Color? = nil, interactive: Bool = false) -> some View {
        if #available(iOS 26, *) {
            let g: Glass = tint.map { Glass.regular.tint($0) } ?? .regular
            self.glassEffect(interactive ? g.interactive() : g, in: .capsule)
        } else {
            self.background(Capsule().fill(tint ?? .clear))
                .background(.regularMaterial, in: Capsule())
                .overlay(Capsule().strokeBorder(.white.opacity(0.15)))
                .shadow(color: .black.opacity(0.12), radius: 12, y: 4)
        }
    }
}
