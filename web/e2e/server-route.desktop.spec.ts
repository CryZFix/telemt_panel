import { test, expect } from "./fixtures";

for (const width of [360, 1280]) {
  for (const locale of ["ru", "en"] as const) {
    test(`four runtime stages distinguish pool, mode and config (${locale}, ${width}px)`, async ({ page: loginPage, login }, testInfo) => {
      await login();
      const page = await loginPage.context().newPage();
      await page.setViewportSize({ width, height: 900 });
      await page.addInitScript((value) => localStorage.setItem("telemt-panel:locale:v1", value), locale);
      let state: "me" | "fallback" | "direct" | "mixed" | "empty" | "disabled" | "stale" = "me";
      await page.route("**/api/telemt/config", async (route) => {
        const response = await route.fetch();
        const body = await response.json();
        // Deliberately contradict the runtime. The path must not use this.
        body.sections.general = { ...body.sections.general, use_middle_proxy: true, me2dc_fallback: true };
        body.sections.upstreams = [{ type: "direct" }];
        await route.fulfill({ response, json: body });
      });
      await page.route("**/api/events?*", async (route) => {
        let body;
        for (let i = 0; i < 30; i++) {
          body = await (await page.request.get("/api/snapshot?topics=runtime,security,stats")).json();
          if (body.runtime?.gates && body.runtime?.upstream_quality) break;
          await new Promise((resolve) => setTimeout(resolve, 100));
        }
        body.runtime.gates = { ...body.runtime.gates, route_mode: state === "fallback" || state === "direct" ? "direct" : "middle", reroute_active: state === "fallback", use_middle_proxy: state !== "direct" };
        const base = { upstream_id: 1, route_kind: "direct", address: "198.51.100.1:443", weight: 1, scopes: "", healthy: true, fails: 0, last_check_age_secs: 0, effective_latency_ms: 12.5, dc: [] };
        const entries = state === "empty" ? [] : state === "mixed" ? [
          { ...base, route_kind: "direct", healthy: true, scopes: "" },
          { ...base, route_kind: "socks5", healthy: true, scopes: "" },
          { ...base, route_kind: "socks5", healthy: false, scopes: "" },
          { ...base, route_kind: "shadowsocks", healthy: false, scopes: "fetch" },
        ] : [{ ...base, route_kind: "socks5", healthy: true, scopes: "", address: "PRIVATE_CREDENTIALS" }];
        body.runtime.upstream_quality = { ...body.runtime.upstream_quality, enabled: state !== "disabled", summary: { ...body.runtime.upstream_quality.summary, configured_total: entries.length }, upstreams: entries };
        const ts = Math.floor(Date.now() / 1000);
        const frames = Object.entries(body).map(([topic, v]) => `event: ${topic}\ndata: ${JSON.stringify({ v, ts })}\n\n`).join("");
        const error = state === "stale" ? 'event: source_error\ndata: {"topic":"runtime","code":"telemt_unavailable"}\n\n' : "";
        await route.fulfill({ contentType: "text/event-stream", body: "retry: 60000\n\n" + frames + error });
      });
      const summary = page.getByTestId("server-config-route");
      const mode = page.getByTestId("server-runtime-mode");
      const pool = page.getByTestId("server-runtime-pool");
      const poolNode = pool.locator("..");
      await page.goto("/server");
      await expect(mode).toHaveText("ME");
      await expect(pool).toHaveText("SOCKS5");
      await expect(poolNode).toHaveAttribute("title", locale === "ru" ? /Доступно 1 из 1/ : /Available: 1 of 1/);
      await expect(summary).not.toContainText("PRIVATE");
      await expect(summary.locator("ol > li")).toHaveCount(4);
      const tops = await summary.locator(".server-route-orb").evaluateAll((items) => items.map((node) => node.getBoundingClientRect().top));
      expect(new Set(tops).size).toBe(1);
      await expect(summary).not.toContainText(locale === "ru" ? "Назначение" : "Destination");
      const track = summary.locator(".server-route-track");
      const before = await track.evaluate((node) => getComputedStyle(node, "::after").transform);
      await page.waitForTimeout(300);
      expect(await track.evaluate((node) => getComputedStyle(node, "::after").transform)).not.toBe(before);
      await page.emulateMedia({ reducedMotion: "reduce" });
      expect(await track.evaluate((node) => getComputedStyle(node, "::after").animationName)).toBe("none");
      await page.emulateMedia({ reducedMotion: "no-preference" });
      await summary.screenshot({ path: testInfo.outputPath("runtime-route.png") });

      state = "fallback";
      await page.reload();
      await expect(mode).toHaveText("Fallback");
      state = "direct";
      await page.reload();
      await expect(mode).toHaveText("Direct");
      await expect(pool).toHaveText("SOCKS5");
      state = "mixed";
      await page.reload();
      await expect(pool).toContainText("SOCKS5");
      await expect(pool).toContainText("Shadowsocks");
      await expect(poolNode).toHaveAttribute("data-attention", "true");
      await expect(poolNode).toHaveAttribute("title", locale === "ru" ? /Доступно 2 из 4/ : /Available: 2 of 4/);
      await expect(poolNode).toHaveAttribute("title", locale === "ru" ? /В том числе со scopes: 1/ : /Including scoped entries: 1/);
      expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
      await summary.screenshot({ path: testInfo.outputPath("runtime-pool.png") });
      state = "empty";
      await page.reload();
      await expect(pool).toHaveText(locale === "ru" ? "Пул пуст" : "Empty pool");
      state = "disabled";
      await page.reload();
      await expect(pool).toHaveText("—");
      await expect(mode).toHaveText("ME");
      state = "stale";
      await page.reload();
      await expect(mode).toHaveText("—");
      await expect(pool).toHaveText("—");
      await expect(poolNode).toHaveAttribute("title", locale === "ru" ? /Нет свежих runtime-данных/ : /No fresh runtime data/);
      expect(await track.evaluate((node) => getComputedStyle(node, "::after").animationName)).toBe("none");
    });
  }
}
