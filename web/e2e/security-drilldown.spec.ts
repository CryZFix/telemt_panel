import { test, expect } from "./fixtures";

for (const locale of ["ru", "en"] as const) {
  test(`TLS assessment opens actionable aggregates (${locale})`, async ({ page: loginPage, login }, testInfo) => {
    await login();
    // The shared login fixture pins its own page to Russian on every load.
    // A new page shares its session but explicitly tests this locale on reload.
    const page = await loginPage.context().newPage();
    await page.addInitScript((value) => localStorage.setItem("telemt-panel:locale:v1", value), locale);
    let mode: "signals" | "empty-ip" | "zero" | "disabled" = "signals";
    await page.route("**/api/telemt/tls-fingerprints?*", async (route) => {
      const response = await route.fetch();
      const payload = await response.json();
      const base = { ...payload.data.by_fingerprint[0], first_seen_epoch_secs: 1789214400, last_seen_epoch_secs: 1789214520 };
      const entries = [
        { ...base, scope: "198.51.100.1", total: 10000, bad_or_probe: 0 },
        { ...base, scope: "198.51.100.2", total: 1000, bad_or_probe: mode === "zero" ? 0 : 2 },
        { ...base, scope: "198.51.100.42", total: 42, bad_or_probe: mode === "zero" ? 0 : 40 },
      ];
      payload.data.by_fingerprint = entries.map((row, i) => ({ ...row, scope: undefined, ja3: `fingerprint-${i}` }));
      payload.data.by_ip = mode === "empty-ip" ? [] : entries;
      payload.data.by_user = [];
      if (mode === "disabled") {
        payload.enabled = false;
        payload.reason = "feature_disabled";
        payload.data = null;
      }
      await route.fulfill({ response, json: payload });
    });

    const actionName = locale === "ru" ? "Показать подозрительные TLS-наблюдения" : "Show suspicious TLS observations";
    const filterName = locale === "ru" ? "Только подозрительные (bad_or_probe > 0)" : "Suspicious only (bad_or_probe > 0)";
    const action = page.getByRole("button", { name: actionName });
    await page.goto("/pulse/diag/security");
    await expect(page.getByTestId("security-hero")).toContainText("42");
    await action.focus();
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(/tab=tls/);
    await expect(page).toHaveURL(/tlsScope=by_ip/);
    await expect(page).toHaveURL(/tlsFilter=suspicious/);
    const panel = page.getByTestId("security-tls-panel");
    await expect(panel).toBeVisible();
    await expect(page.getByRole("checkbox", { name: filterName })).toBeChecked();
    await expect(panel.locator("[data-security-row]")).toHaveCount(2);
    await expect(panel.locator("[data-security-row]").first()).toContainText("198.51.100.42");
    await expect(panel.locator("[data-security-row]").first()).toBeInViewport({ ratio: 0.5 });
    await expect(panel.locator("[data-security-row=ok]")).toHaveCount(0);
    const firstBar = panel.locator("[data-security-row]").first().locator("i");
    await expect(firstBar).toHaveCSS("background-image", /linear-gradient/);
    expect(await firstBar.evaluate((node) => getComputedStyle(node).backgroundImage)).not.toContain("rgba(0, 0, 0, 0)");
    await expect(firstBar).toHaveAttribute("style", /width: 100%/);
    await expect(panel).toContainText(locale === "ru" ? "не число уникальных IP" : "not unique IPs");
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
    await page.screenshot({ path: testInfo.outputPath("tls-drilldown.png"), fullPage: true });

    await page.reload();
    await expect(page.getByRole("checkbox", { name: filterName })).toBeChecked();
    await page.goBack();
    await expect(page.getByTestId("security-posture-panel")).toBeVisible();
    await page.goForward();
    await expect(panel.locator("[data-security-row]")).toHaveCount(2);
    await page.getByRole("checkbox", { name: filterName }).uncheck();
    await expect(panel.locator("[data-security-row]")).toHaveCount(3);
    await expect(panel.locator("[data-security-row]").first()).toContainText("198.51.100.1");
    const search = panel.getByRole("textbox");
    await search.fill("198.51.100.2");
    await page.getByRole("checkbox", { name: filterName }).check();
    await expect(search).toHaveValue("198.51.100.2");
    await expect(panel.locator("[data-security-row]")).toHaveCount(1);
    await action.click();
    await expect(search).toHaveValue("");
    await expect(panel.locator("[data-security-row]")).toHaveCount(2);

    mode = "empty-ip";
    await page.goto("/pulse/diag/security");
    await action.click();
    await expect(panel.locator("[data-security-row]")).toHaveCount(0);
    await expect(panel).toContainText(locale === "ru" ? "не все наблюдения доступны по IP" : "not every observation is available by IP");
    await panel.getByRole("tab", { name: locale === "ru" ? /^Отпечатки/ : /^Fingerprints/ }).click();
    await expect(panel.locator("[data-security-row]")).toHaveCount(2);

    mode = "zero";
    await page.goto("/pulse/diag/security");
    await expect(page.getByTestId("security-posture-panel")).toBeVisible();
    await expect(action).toHaveCount(0);

    mode = "disabled";
    await page.goto("/pulse/diag/security?tab=tls&tlsScope=by_ip&tlsFilter=suspicious");
    await expect(page.getByTestId("security-detail")).toBeVisible();
    await expect(action).toHaveCount(0);
    await expect(panel).toHaveCount(0);
  });
}
