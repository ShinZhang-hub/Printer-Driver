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
    pub locations: Vec<String>,
    pub loc_ips: HashMap<String, Vec<String>>,
    pub loc_names: HashMap<String, Vec<String>>,
    pub conflict: HashMap<String, bool>,
    pub existing: Vec<ExistingPrinter>,
    pub has_driver_ppd: bool,
}

pub fn load_config() -> Config {
    // NOTE: config is READ-ONLY from the client side. Pushing config edits to
    // the server is intentionally not implemented (temporarily disabled).
    config::load_remote(config::EMBEDDED_CONFIG)
}

pub fn initial_state() -> InitialState {
    let cfg = load_config();
    let lang = i18n::detect();
    let strings = i18n::strings(&lang);

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
            printers
                .iter()
                .any(|p| printer::find_printer_by_ip(&p.ip).is_some()),
        );
    }

    let existing = printer::list_printers_with_ips()
        .into_iter()
        .map(|(name, ip)| ExistingPrinter { name, ip })
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

    InitialState {
        lang,
        strings,
        detected_location,
        detected_name: detected_name,
        detected_ip: detected_ip2,
        locations,
        loc_ips,
        loc_names,
        conflict,
        existing,
        has_driver_ppd: true,
    }
}
