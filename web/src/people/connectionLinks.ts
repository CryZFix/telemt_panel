import type {UserLinksWire} from "../realtime/topics";
import type {WebAccessView} from "../lib/api/generated/types.gen";
import {parseLink} from "./parseLink";

export type ConnectionKind = "tls" | "secure" | "classic" | "web";
export type LinkFormat = "tg" | "tme";
export interface WebLinkProfile { host: string; mode: "plain" | "dd" }
export interface ConnectionLink {
  kind: ConnectionKind; url: string; endpoint: string; domain: string | null;
  profileMode?: "plain" | "dd"; primary: boolean;
}

export function webProfileIndex(view: WebAccessView | undefined): Map<string, WebLinkProfile[]> {
  const index = new Map<string, WebLinkProfile[]>();
  if (!view?.enabled) return index;
  for (const vhost of view.vhosts) for (const profile of vhost.profiles) {
    if (profile.secret_mode !== "plain" && profile.secret_mode !== "dd") continue;
    const entries = index.get(profile.user) ?? [];
    entries.push({host: vhost.host, mode: profile.secret_mode});
    index.set(profile.user, entries);
  }
  return index;
}

function proxyURL(raw: string): URL | null {
  try {
    const u = new URL(raw);
    if (!(u.protocol === "tg:" && u.hostname === "proxy" && !u.pathname || u.protocol === "https:" && u.hostname === "t.me" && u.pathname === "/proxy")) return null;
    if (u.username || u.password || u.hash || (u.protocol === "https:" && u.port)) return null;
    if (["server", "port", "secret"].some(key => u.searchParams.getAll(key).length !== 1)) return null;
    const host = u.searchParams.get("server"), port = u.searchParams.get("port");
    if (!host || /[\s/?#@]/.test(host) || !port || !/^\d+$/.test(port) || Number(port)<1 || Number(port)>65535) return null;
    return u;
  } catch { return null; }
}

export function collectConnectionLinks(links: UserLinksWire, profiles: readonly WebLinkProfile[] = []): ConnectionLink[] {
  const result: ConnectionLink[] = [];
  const seen = new Set<string>();
  const masks = new Map(links.tls_domains.map(d => [d.link, d.domain]));
  let secret: string | null = null;
  let conflictingSecrets = false;
  for (const kind of ["tls", "secure", "classic"] as const) {
    const originals = kind === "tls" ? [...links.tls, ...links.tls_domains.map(d=>d.link)] : links[kind];
    for (const [i, raw] of originals.entries()) {
      const url = proxyURL(raw), parsed = parseLink(raw);
      if (!url || parsed.type !== kind || !parsed.secret || (kind === "tls" && !parsed.domain)) continue;
      const canonical = `tg://proxy?${url.searchParams.toString()}`;
      if (seen.has(canonical)) continue;
      seen.add(canonical);
      if (secret !== null && secret !== parsed.secret) conflictingSecrets = true;
      secret ??= parsed.secret;
      result.push({kind, url: canonical, endpoint: `${parsed.server}:${parsed.port}`, domain: masks.get(raw) ?? parsed.domain, primary: i === 0 && (kind !== "tls" || links.tls.length > 0)});
    }
  }
  if (secret && !conflictingSecrets) for (const [i, p] of profiles.entries()) {
    // WEB uses the vhost hostname, never public_addr or a synthetic port.
    if (!p.host || /[\s/:?#@]/.test(p.host)) continue;
    const url = `tg://webproxy?${new URLSearchParams({server:p.host,secret:(p.mode === "dd" ? "dd" : "")+secret})}`;
    if (seen.has(url)) continue;
    seen.add(url);result.push({kind:"web", url, endpoint:p.host, domain:null, profileMode:p.mode, primary:i===0});
  }
  return result;
}

export function formatConnectionLink(link: ConnectionLink, format: LinkFormat): string {
  return format === "tme" && link.kind !== "web" ? link.url.replace(/^tg:\/\/proxy\?/, "https://t.me/proxy?") : link.url;
}
