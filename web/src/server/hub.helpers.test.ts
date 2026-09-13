import { describe, expect, it } from "vitest";
import type { HostInfo, TelemtConfig, UpdatesStatus } from "../lib/api/generated/types.gen";
import type { RuntimeUpstreamQualityUpstream } from "../realtime/topics";
import { gates } from "../pulse/__fixtures__/runtime";
import { upstreamQuality } from "../pulse/__fixtures__/stats";
import {
  activeUpdateRun,
  hostCapabilityCount,
  newestAvailableRelease,
  summarizeServerConfig,
  summarizeEgress,
  runtimeRouteMode,
} from "./hub.helpers";

function config(sections: TelemtConfig["sections"]): TelemtConfig {
  return { revision: "rev", sections };
}

describe("summarizeServerConfig", () => {
  it("keeps config facts separate from the live connection path", () => {
    expect(
      summarizeServerConfig(
        config({
          general: {
            use_middle_proxy: true,
            me2dc_fallback: true,
            modes: { tls: true },
          },
          censorship: { mask: true },
          dc_overrides: { "203": ["127.0.0.1:443"] },
        }),
      ),
    ).toEqual({
      transport: "tls",
      masking: true,
      dcOverrides: 1,
    });
  });

  it("keeps missing and future section shapes honest", () => {
    expect(summarizeServerConfig(config({ general: null }))).toEqual({
      transport: "unknown",
      masking: null,
      dcOverrides: null,
    });
  });
});

describe("live runtime route", () => {
  it("does not call permission to fall back an active fallback", () => {
    expect(runtimeRouteMode({ ...gates, route_mode: "middle", reroute_active: false, me2dc_fallback_enabled: true })).toBe("me");
    expect(runtimeRouteMode({ ...gates, route_mode: "direct", reroute_active: true })).toBe("fallback");
    expect(runtimeRouteMode({ ...gates, use_middle_proxy: false, route_mode: "direct", reroute_active: false })).toBe("direct");
  });
  it("keeps missing, future and inconsistent route states unknown", () => {
    expect(runtimeRouteMode(null)).toBe("unknown");
    expect(runtimeRouteMode({ ...gates, route_mode: "future" })).toBe("unknown");
    expect(runtimeRouteMode({ ...gates, route_mode: "middle", reroute_active: true })).toBe("unknown");
  });
});

function quality(rows: Array<Partial<RuntimeUpstreamQualityUpstream>>) {
  return { ...upstreamQuality, enabled: true, summary: { ...upstreamQuality.summary!, configured_total: rows.length }, upstreams: rows.map((row, upstream_id) => ({ ...upstreamQuality.upstreams![0]!, scopes: "", upstream_id, ...row })) };
}

describe("runtime pool availability", () => {
  it("groups all loaded upstreams, including scoped and unhealthy entries", () => {
    expect(summarizeEgress(quality([
      { route_kind: "direct", healthy: true },
      { route_kind: "socks5", healthy: true },
      { route_kind: "socks5", healthy: false },
      { route_kind: "shadowsocks", healthy: false, scopes: "fetch" },
    ]))).toEqual({ routes: [{ type: "direct", count: 1, healthy: 1 }, { type: "socks5", count: 2, healthy: 1 }, { type: "shadowsocks", count: 1, healthy: 0 }], total: 4, healthy: 2, scoped: 1 });
  });
  it("does not invent a default Direct upstream or a healthy state", () => {
    expect(summarizeEgress(null)).toBeNull();
    expect(summarizeEgress({ ...quality([]), enabled: false })).toBeNull();
    expect(summarizeEgress({ ...quality([{ healthy: true }]), upstreams: undefined })).toBeNull();
    expect(summarizeEgress({ ...quality([]), upstreams: undefined })).toEqual({ routes: [], scoped: 0, total: 0, healthy: 0 });
    expect(summarizeEgress(quality([{ route_kind: "direct", healthy: false }]))?.healthy).toBe(0);
  });
  it("does not reveal endpoints or treat unknown protocol names as labels", () => {
    const summary = summarizeEgress(quality([{ route_kind: "PRIVATE_FUTURE_TYPE", address: "PRIVATE_CREDENTIALS", healthy: true }]));
    expect(summary?.routes).toEqual([{ type: "unknown", count: 1, healthy: 1 }]);
    expect(JSON.stringify(summary)).not.toContain("PRIVATE");
  });
});

describe("server hub update and host summaries", () => {
  const targets: UpdatesStatus["targets"] = [
    {
      target: "telemt",
      current_version: "3.4.25",
      releases: [
        { version: "3.5.4", published_at: "2026-08-26T00:00:00Z", newer: true },
        { version: "3.5.5", published_at: "2026-08-27T00:00:00Z", newer: true },
      ],
      active_run: {
        run_id: "run-1",
        target: "telemt",
        phase: "installing",
        version_to: "3.5.5",
        started_at: "2026-08-27T00:00:00Z",
      },
    },
  ];

  it("selects the newest newer release and the non-terminal run", () => {
    expect(newestAvailableRelease(targets[0])?.version).toBe("3.5.5");
    expect(activeUpdateRun(targets)?.run_id).toBe("run-1");
  });

  it("counts capabilities without assuming a fixed total", () => {
    const caps: HostInfo["caps"] = {
      restart_telemt: true,
      restart_panel: true,
      log_tail: true,
      log_stream: true,
      self_update: false,
    };
    expect(hostCapabilityCount(caps)).toEqual({ available: 4, total: 5 });
    expect(hostCapabilityCount(undefined)).toEqual({ available: 0, total: 0 });
  });
});
