//! UI-facing state builder. Keeps all business logic in the core so the
//! standalone Tauri app and the future onboarding app share it unchanged.

use std::collections::HashMap;

use crate::config::{self, Config};
use crate::i18n;
use crate::location;
use crate::printer;

#[derive(Debug, Clone, serde::Serialize)]
pub struct ExistingPrinter {
    pub name: String,
    pub ip: String,
}

/// Everything the confirm dialog needs to render.
#[derive(Debug, Clone, serde::Serialize)]
pub struct InitialState {
    pub lang: String,
    pub strings: HashMap<String, String>,
    pub detected_location: Option<String>,
    pub detected_name: String,
    pub detected_ip: String,
    pub local_ip: String,
    pub default_printer: String,
    pub locations: Vec<String>,
    pub loc_ips: HashMap<String, Vec<String>>,
    pub loc_names: HashMap<String, Vec<String>>,
    pub conflict: HashMap<String, bool>,
    pub existing: Vec<ExistingPrinter>,
    pub has_driver_ppd: bool,
}

pub fn load_config() -> Config {
    // Config is cached in-process: the UI reads it instantly at startup
    // (seeded from the embedded copy), and `config::refresh_config()` swaps
    // in a fresh remote copy in the background.
    config::shared_snapshot()
}

pub fn initial_state() -> InitialState {
    let cfg = load_config();
    let lang = i18n::detect();
    let strings = i18n::strings(&lang);

    let local_ip_str = location::local_ip()
        .map(|ip| ip.to_string())
        .unwrap_or_default();

    let detected_ip = location::detected_local_ip(&cfg)
        .map(|ip| ip.to_string())
        .unwrap_or_default();
    let detected_location = if detected_ip.is_empty() {
        None
    } else {
        cfg.match_location(
            detected_ip
                .parse()
                .unwrap_or(std::net::Ipv4Addr::UNSPECIFIED),
        )
        .map(|l| l.name.clone())
    };

    let mut locations = Vec::new();
    for l in &cfg.locations {
        locations.push(l.name.clone());
    }

    // ONE `lpstat -v` call feeds both the existing-printer list and the
    // per-location conflict checks. Spawning a subprocess per printer (as
    // before) paid CUPS cold-start multiple times on startup.
    let by_ip = printer::printers_by_ip();

    let mut loc_ips: HashMap<String, Vec<String>> = HashMap::new();
    let mut loc_names: HashMap<String, Vec<String>> = HashMap::new();
    let mut conflict: HashMap<String, bool> = HashMap::new();
    for loc in &cfg.locations {
        let printers = loc.all_printers();
        loc_ips.insert(
            loc.name.clone(),
            printers.iter().map(|p| p.ip.clone()).collect(),
        );
        loc_names.insert(
            loc.name.clone(),
            printers.iter().map(|p| p.name.clone()).collect(),
        );
        conflict.insert(
            loc.name.clone(),
            printers.iter().any(|p| by_ip.contains_key(&p.ip)),
        );
    }

    let existing = by_ip
        .iter()
        .map(|(ip, name)| ExistingPrinter {
            name: name.clone(),
            ip: ip.clone(),
        })
        .collect();

    let detected = detected_location.clone().and_then(|l| {
        cfg.location_by_name(&l)
            .map(|loc| loc.all_printers())
            .map(|ps| {
                (
                    ps.first().map(|p| p.name.clone()).unwrap_or_default(),
                    ps.first().map(|p| p.ip.clone()).unwrap_or_default(),
                )
            })
    });
    let (detected_name, detected_ip2) = detected.unwrap_or_default();

    let default_printer = printer::default_printer();

    InitialState {
        lang,
        strings,
        detected_location,
        detected_name,
        detected_ip: detected_ip2,
        local_ip: local_ip_str,
        default_printer,
        locations,
        loc_ips,
        loc_names,
        conflict,
        existing,
        has_driver_ppd: true,
    }
}
