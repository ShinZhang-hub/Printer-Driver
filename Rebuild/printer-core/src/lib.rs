//! printer-core: platform-independent business logic for the printer
//! installer. Shared by the standalone Tauri app and the future onboarding
//! app — no UI, no framework dependencies.

pub mod config;
pub mod driver;
pub mod flow;
pub mod i18n;
pub mod location;
#[cfg(target_os = "macos")]
pub mod mac_driver;
pub mod printer;

#[cfg(target_os = "windows")]
pub mod win_installer;

pub use flow::{initial_state, load_config, InitialState};
