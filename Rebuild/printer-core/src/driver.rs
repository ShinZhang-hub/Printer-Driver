//! Windows driver package support: embedded INF resources, driver model
//! matching (ported from the Go `drvpack` implementation) and runtime
//! unpacking so `pnputil` / `printui` can reference real files.

use std::collections::HashMap;
use std::path::{Path, PathBuf};

/// A model installation record parsed from a Windows .inf file.
#[derive(Debug, Clone)]
pub struct InfEntry {
    /// Absolute path of the .inf once unpacked ("" while still embedded).
    pub inf_file: String,
    /// Human model name, `%STR%` tokens already resolved.
    pub model_name: String,
    /// The `[xxx.NTx86...]` section id that installs this device.
    pub install_section: String,
    /// Hardware id from the model line (e.g. `USBPRINT\...`).
    pub hardware_id: String,
}

/// Unpack the embedded INF driver package into a fresh temp dir and return
/// the directory. Caller is responsible for cleaning it up.
pub fn unpack_embedded_drivers() -> Result<PathBuf, String> {
    let dir = std::env::temp_dir().join(format!(
        "printer-installer-drv-{}",
        std::process::id()
    ));
    let _ = std::fs::remove_dir_all(&dir);
    std::fs::create_dir_all(&dir).map_err(|e| e.to_string())?;

    for (rel, data) in MANIFEST {
        let p = dir.join(rel);
        if let Some(parent) = p.parent() {
            let _ = std::fs::create_dir_all(parent);
        }
        std::fs::write(&p, data).map_err(|e| e.to_string())?;
    }
    Ok(dir)
}

/// Parse every .inf under `dir` (non-recursive) and return all model entries.
pub fn parse_inf_dir(dir: &Path) -> Vec<InfEntry> {
    let mut all = Vec::new();
    let Ok(entries) = std::fs::read_dir(dir) else {
        return all;
    };
    for e in entries.flatten() {
        let name = e.file_name().to_string_lossy().to_lowercase();
        if !name.ends_with(".inf") {
            continue;
        }
        all.extend(parse_inf(&e.path()));
    }
    all
}

/// Parse a single .inf (Manufacturer / model-section / Strings).
fn parse_inf(path: &Path) -> Vec<InfEntry> {
    let Ok(text) = std::fs::read_to_string(path) else {
        return Vec::new();
    };
    let mut sec: Section = Section::Unknown;
    let mut strings: HashMap<String, String> = HashMap::new();
    let mut models: Vec<InfEntry> = Vec::new();

    for raw in text.lines() {
        let line = raw.trim();
        if line.is_empty() || line.starts_with(';') {
            continue;
        }
        if line.starts_with('[') && line.ends_with(']') {
            let name = line[1..line.len() - 1].trim().to_lowercase();
            sec = match name.as_str() {
                "manufacturer" => Section::Manufacturer,
                "strings" => Section::Strings,
                n if n.contains(".ntamd64") || n.contains(".ntx86") => Section::Model,
                _ => Section::Unknown,
            };
            continue;
        }
        match sec {
            Section::Strings => {
                if let Some(eq) = line.find('=') {
                    let key = line[..eq].trim().trim_matches('%');
                    let val = line[eq + 1..].trim().trim_matches('"');
                    strings.insert(key.to_string(), val.to_string());
                }
            }
            Section::Model => {
                // "FF Apeos C2571" = FF_A_PLW, USBPRINT\FFApeosC2571
                let (Some(open), Some(close)) = (line.find('"'), line.rfind('"')) else {
                    continue;
                };
                let model = &line[open + 1..close];
                let rhs = line[close + 1..].trim();
                if rhs.starts_with('=') {
                    let rest = rhs[1..].trim();
                    let mut parts = rest.splitn(2, ',');
                    let install = parts.next().unwrap_or("").trim().to_string();
                    let hwid = parts
                        .next()
                        .unwrap_or("")
                        .trim()
                        .split_whitespace()
                        .next()
                        .unwrap_or("")
                        .to_string();
                    models.push(InfEntry {
                        inf_file: path.to_string_lossy().to_string(),
                        model_name: resolve(&model, &strings),
                        install_section: install,
                        hardware_id: hwid,
                    });
                }
            }
            _ => {}
        }
    }
    models
}

fn resolve(s: &str, strings: &HashMap<String, String>) -> String {
    let mut out = s.to_string();
    let mut start = 0;
    while let Some(i) = out[start..].find('%') {
        let abs = start + i;
        if let Some(j) = out[abs + 1..].find('%') {
            let end = abs + 1 + j;
            let key = &out[abs + 1..end];
            if let Some(v) = strings.get(key) {
                out.replace_range(abs..=end, v);
                start = abs + v.len();
                continue;
            }
        }
        start = abs + 1;
    }
    out
}

#[derive(Clone, Copy, PartialEq)]
enum Section {
    Unknown,
    Manufacturer,
    Model,
    Strings,
}

// ---- model matching (port of internal/drvpack/match.go) ----

/// Strip brand prefixes like "FF ", "Fujifilm ", "HP " ...
fn normalize_model(s: &str) -> String {
    let mut low = s.trim().to_lowercase();
    for p in [
        "ff ", "fujifilm ", "hp ", "canon ", "ricoh ", "kyocera ", "brother ", "epson ",
    ] {
        if low.starts_with(p) {
            low = low[p.len()..].to_string();
            break;
        }
    }
    low.split_whitespace().collect::<Vec<_>>().join(" ")
}

fn extract_numbers(s: &str) -> String {
    s.chars()
        .filter(|c| c.is_ascii_digit())
        .collect::<String>()
}

fn match_score(inf_model: &str, snmp_model: &str) -> i32 {
    let a = normalize_model(inf_model);
    let b = normalize_model(snmp_model);
    if a.is_empty() || b.is_empty() {
        return 0;
    }
    if a == b {
        return 100;
    }
    let a_nums = extract_numbers(&a);
    let b_nums = extract_numbers(&b);
    if !a_nums.is_empty() && a_nums == b_nums {
        if contains_words(&a, &b) || contains_words(&b, &a) {
            return 80;
        }
        return 60;
    }
    if a.contains(&b) || b.contains(&a) {
        return 50;
    }
    0
}

fn contains_words(a: &str, b: &str) -> bool {
    let words: Vec<&str> = b.split_whitespace().collect();
    if words.is_empty() {
        return false;
    }
    let hits = words.iter().filter(|w| a.contains(*w)).count();
    hits * 2 >= words.len()
}

/// Find the best inf entry for a model name. Exact match wins, then fuzzy.
pub fn find_model<'a>(entries: &'a [InfEntry], model_name: &str) -> Option<&'a InfEntry> {
    let clean = normalize_model(model_name);
    for e in entries {
        if normalize_model(&e.model_name) == clean {
            return Some(e);
        }
    }
    let mut best: Option<&InfEntry> = None;
    let mut best_score = 0;
    for e in entries {
        let s = match_score(&e.model_name, model_name);
        if s > best_score {
            best_score = s;
            best = Some(e);
        }
    }
    best
}

// Injected at compile time by build.rs. Declares `pub static DRV_*` byte
// arrays plus `pub static MANIFEST: &[(&str, &[u8])] = [(path, bytes), ..]`.
include!(concat!(env!("OUT_DIR"), "/drv_embedded.rs"));

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn manifest_has_inf_files() {
        assert!(!MANIFEST.is_empty());
        let has_inf = MANIFEST.iter().any(|(n, _)| n.ends_with(".INF"));
        assert!(has_inf, "embedded drivers must include at least one INF");
    }

    #[test]
    fn unpack_and_parse_inf() {
        let dir = unpack_embedded_drivers().unwrap();
        let entries = parse_inf_dir(&dir);
        let _ = std::fs::remove_dir_all(&dir);
        assert!(!entries.is_empty(), "INF dir should yield model entries");
    }

    #[test]
    fn match_model_exact_and_fuzzy() {
        let entries = vec![
            InfEntry {
                inf_file: "x.INF".into(),
                model_name: "FF Apeos C2571".into(),
                install_section: "FF_A_PLW".into(),
                hardware_id: "USBPRINT\\X".into(),
            },
            InfEntry {
                inf_file: "y.INF".into(),
                model_name: "Fujifilm Apeos C3070".into(),
                install_section: "FF_A_PWM".into(),
                hardware_id: "USBPRINT\\Y".into(),
            },
        ];
        // exact (brand stripped => "apeos c2571" vs "apeos c3070")
        assert!(find_model(&entries, "FF Apeos C2571").is_some());
        // numeric core "3070" fuzzy match on the second entry
        let found = find_model(&entries, "Apeos C3070");
        assert_eq!(found.map(|e| e.model_name.as_str()), Some("Fujifilm Apeos C3070"));
    }
}