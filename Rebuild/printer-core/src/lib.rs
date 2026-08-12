//! printer-core: platform-independent business logic for the printer
//! installer. Shared by the standalone Tauri app and the future onboarding
//! app — no UI, no framework dependencies.

pub mod config;
pub mod flow;
pub mod i18n;
pub mod location;
pub mod printer;

pub use flow::{initial_state, load_config, InitialState};
