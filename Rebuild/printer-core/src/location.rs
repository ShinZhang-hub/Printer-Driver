use crate::config::{Config, LocationConfig};
use std::net::Ipv4Addr;

/// Collect all local IPv4 addresses (excluding loopback).
fn local_v4_all() -> Vec<Ipv4Addr> {
    let mut v = local_v4_via_ifconfig();
    v.retain(|ip| !ip.is_loopback());
    v.dedup();
    v
}

/// Portable local IPv4 enumeration.
#[cfg(any(target_os = "macos", target_os = "linux"))]
fn local_v4_via_ifconfig() -> Vec<Ipv4Addr> {
    let mut v4 = Vec::new();
    if let Ok(out) = std::process::Command::new("ifconfig").output() {
        let text = String::from_utf8_lossy(&out.stdout);
        for line in text.lines() {
            let line = line.trim_start();
            if let Some(rest) = line.strip_prefix("inet ") {
                let mut parts = rest.split_whitespace();
                if let Some(addr) = parts.next() {
                    if let Ok(ip) = addr.parse::<Ipv4Addr>() {
                        v4.push(ip);
                    }
                }
            }
        }
    }
    v4
}

#[cfg(target_os = "windows")]
fn local_v4_via_ifconfig() -> Vec<Ipv4Addr> {
    let mut v4 = Vec::new();
    if let Ok(out) = std::process::Command::new("ipconfig").output() {
        let text = String::from_utf8_lossy(&out.stdout);
        for line in text.lines() {
            let line = line.trim();
            if let Some(rest) = line.strip_prefix("IPv4 Address") {
                if let Some(idx) = rest.find(':') {
                    let addr = rest[idx + 1..].trim();
                    if let Ok(ip) = addr.parse::<Ipv4Addr>() {
                        v4.push(ip);
                    }
                }
            }
        }
    }
    v4
}

/// Pick the local IPv4 that falls inside one of the configured subnets,
/// preferring it over whatever interface comes first (VPN/VM adapters sort
/// before the real LAN NIC on fresh machines).
pub fn detected_local_ip(cfg: &Config) -> Option<Ipv4Addr> {
    let addrs = local_v4_all();
    if let Some(first) = addrs.first() {
        if cfg.match_location(*first).is_some() {
            return Some(*first);
        }
    }
    for a in &addrs {
        if is_link_local(*a) {
            continue;
        }
        if cfg.match_location(*a).is_some() {
            return Some(*a);
        }
    }
    addrs.first().copied()
}

pub fn is_link_local(ip: Ipv4Addr) -> bool {
    let o = ip.octets();
    o[0] == 169 && o[1] == 254
}

impl Config {
    /// Match a local IP against configured location subnets.
    pub fn match_location(&self, ip: Ipv4Addr) -> Option<&LocationConfig> {
        for loc in &self.locations {
            for subnet in &loc.subnets {
                if cidr_contains(subnet, ip) {
                    return Some(loc);
                }
            }
        }
        None
    }

    pub fn location_by_name(&self, name: &str) -> Option<&LocationConfig> {
        self.locations.iter().find(|l| l.name == name)
    }
}

/// Minimal CIDR containment check (no external ipnet dependency).
pub fn cidr_contains(cidr: &str, ip: Ipv4Addr) -> bool {
    let (net_part, prefix_part) = match cidr.split_once('/') {
        Some(p) => p,
        None => return false,
    };
    let prefix: u8 = match prefix_part.parse() {
        Ok(p) if p <= 32 => p,
        _ => return false,
    };
    let net: Ipv4Addr = match net_part.parse() {
        Ok(n) => n,
        Err(_) => return false,
    };
    let net_u = u32::from(net);
    let ip_u = u32::from(ip);
    let mask = if prefix == 0 {
        0
    } else {
        u32::MAX << (32 - prefix)
    };
    (net_u & mask) == (ip_u & mask)
}
