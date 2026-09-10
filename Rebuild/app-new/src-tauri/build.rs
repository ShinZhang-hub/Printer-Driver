fn main() {
    // Run the whole app elevated: UAC prompts once at launch, and all
    // in-app install/remove operations then run silently (is_elevated() is
    // true, so run_elevated() short-circuits without per-op UAC).
    // NOTE: must go through tauri_build's own manifest slot — a separate
    // embed-resource manifest would lose to tauri-winres's default manifest.
    let mut windows = tauri_build::WindowsAttributes::new();
    windows = windows.app_manifest(include_str!("app.manifest"));
    let attrs = tauri_build::Attributes::new().windows_attributes(windows);
    tauri_build::try_build(attrs).expect("failed to run tauri build script");
}
