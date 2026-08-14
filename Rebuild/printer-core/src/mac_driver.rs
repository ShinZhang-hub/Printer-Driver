//! macOS Fujifilm driver support: embedded filter + PDE plugins referenced by
//! the FF PPD, unpacked to /Library/Printers/FUJIFILM at install time so a
//! fresh machine can print without a separately-installed driver package.
//!
//! Mirrors the Windows approach (driver.rs) — resources compiled in as static
//! bytes via build.rs (MAC_MANIFEST), extracted on demand.

use std::path::{Path, PathBuf};

/// Extract the embedded FF driver to an arbitrary destination, preserving the
/// relative layout (Filter/..., PDEs/.../*.plugin/Contents/...).
pub fn unpack_to(dest: &Path) -> Result<PathBuf, String> {
    for (rel, data) in MAC_MANIFEST {
        let p = dest.join(rel);
        if let Some(parent) = p.parent() {
            std::fs::create_dir_all(parent).map_err(|e| e.to_string())?;
        }
        std::fs::write(&p, data).map_err(|e| e.to_string())?;
    }
    Ok(dest.to_path_buf())
}

// Injected at compile time by build.rs: `MAC_MANIFEST: &[(&str, &[u8])]`.
include!(concat!(env!("OUT_DIR"), "/drv_embedded.rs"));

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn mac_manifest_has_filter() {
        assert!(
            MAC_MANIFEST
                .iter()
                .any(|(n, _)| n.contains("FFACMMCFilter")),
            "mac manifest must include FFACMMCFilter"
        );
    }

    #[test]
    fn unpack_preserves_layout() {
        let dest = std::path::PathBuf::from("/tmp/printer-core-mac-drv-test");
        let _ = std::fs::remove_dir_all(&dest);
        unpack_to(&dest).unwrap();

        let filter = dest.join("Filter/FFACMMCFilter");
        assert!(filter.exists(), "filter must exist");
        assert!(
            std::fs::metadata(&filter).map(|m| m.len() > 100_000).unwrap_or(false),
            "filter should be a real binary"
        );
        assert!(
            dest.join("PDEs/FFACMMFeatures.plugin/Contents/Info.plist").exists(),
            "PDE plugin must be unpacked"
        );
        let _ = std::fs::remove_dir_all(&dest);
    }
}
