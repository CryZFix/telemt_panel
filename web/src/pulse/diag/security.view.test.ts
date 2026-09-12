import { describe, expect, it } from "vitest";
import { posture, tlsFingerprints } from "../__fixtures__/security";
import { filterTlsRows, securityLevel, tlsRowIdentity, tlsTotals } from "./security.view.helpers";

describe("Security custom detail view", () => {
  it("assesses the whole API access chain instead of alarming on one false", () => {
    expect(securityLevel({ ...posture, api_auth_header_enabled: false }, 0)).toBe("ok");
    expect(
      securityLevel(
        {
          ...posture,
          api_whitelist_enabled: false,
          api_auth_header_enabled: false,
          api_read_only: false,
        },
        0,
      ),
    ).toBe("error");
    expect(securityLevel({ ...posture, api_whitelist_enabled: false }, 0)).toBe("warn");
    expect(securityLevel(posture, 1)).toBe("warn");
  });

  it("uses the fingerprint list as the single ClientHello total", () => {
    const totals = tlsTotals(tlsFingerprints.by_fingerprint);
    expect(totals.observed).toBeGreaterThan(0);
    expect(totals.bad).toBe(0);
    expect(tlsTotals(undefined)).toEqual({ observed: null, bad: null });
  });

  it("keeps TLS scopes independent and searches every useful identity", () => {
    const ip = tlsFingerprints.by_ip[23]!;
    const rows = filterTlsRows(tlsFingerprints.by_ip, "by_ip", ip.scope!);
    expect(rows).toHaveLength(1);
    expect(tlsRowIdentity(rows[0]!, "by_ip")).toBe(ip.scope);
    expect(filterTlsRows(tlsFingerprints.by_ip, "by_ip", ip.ja4)).toContainEqual(ip);
  });

  it("filters signals and ranks their count rather than total traffic", () => {
    const base = tlsFingerprints.by_ip[0]!;
    const rows = [
      { ...base, scope: "192.0.2.1", total: 10000, bad_or_probe: 0 },
      { ...base, scope: "192.0.2.2", total: 1000, bad_or_probe: 2 },
      { ...base, scope: "192.0.2.3", total: 42, bad_or_probe: 42 },
    ];
    expect(filterTlsRows(rows, "by_ip", "", true).map((row) => row.scope)).toEqual(["192.0.2.3", "192.0.2.2"]);
    expect(filterTlsRows(rows, "by_ip", "192.0.2.1", true)).toEqual([]);
    expect(filterTlsRows(rows, "by_ip", "192.0.2.2", true)).toHaveLength(1);
    expect(filterTlsRows(rows, "by_ip", "")[0]?.scope).toBe("192.0.2.1");
    expect(rows[0]?.scope).toBe("192.0.2.1");
  });
});
