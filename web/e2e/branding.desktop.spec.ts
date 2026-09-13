import path from "node:path";
import { test, expect } from "./fixtures";

test.describe.configure({ mode: "serial" });

for (const width of [320, 390, 1440]) {
  test(`local branding applies to settings, login and browser (${width}px)`, async ({ page, login }, testInfo) => {
    await login();
    await page.setViewportSize({ width, height: 900 });
    const endpoint = "/api/settings/branding";
    const defaults = { title: "Telemt Panel", logo_mode: "default", logo_path: "" };
    const saved = await page.request.get(endpoint);
    expect(saved.status()).toBe(200);
    try {
      await page.goto("/server/settings");
      const card = page.getByTestId("settings-branding");
      await card.getByRole("button", { name: "Настроить оформление" }).click();
      const dialog = page.getByRole("dialog");
      await dialog.getByLabel("Название панели", { exact: true }).fill("Домашняя сеть");
      await dialog.getByRole("combobox", { name: "Логотип" }).selectOption("custom");
      await dialog.getByLabel("Путь к файлу на сервере", { exact: true }).fill("/missing/private-logo.webp");
      await dialog.getByRole("button", { name: "Сохранить оформление" }).click();
      await expect(dialog.getByRole("alert")).toContainText("Не удалось прочитать файл");
      await expect(page).toHaveTitle("Telemt Panel");
      const logoPath = path.resolve("src/assets/logo-menu.webp");
      await dialog.getByLabel("Путь к файлу на сервере", { exact: true }).fill(logoPath);
      await dialog.screenshot({ path: testInfo.outputPath("branding-form.png") });
      await dialog.getByRole("button", { name: "Сохранить оформление" }).click();
      await expect(dialog).not.toBeVisible();
      await expect(page).toHaveTitle("Домашняя сеть");
      await expect(card.locator("img")).toHaveAttribute("src", /\/api\/branding\/logo\?v=/);
      await expect.poll(() => card.locator("img").evaluate((img: HTMLImageElement) => img.complete && img.naturalWidth > 0)).toBe(true);
      if (width === 1440) await expect(page.getByTestId("full-sidebar")).toContainText("Домашняя сеть");
      expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
      await card.screenshot({ path: testInfo.outputPath("branding-settings.png") });

      const anonymous = await page.context().browser()!.newContext({ baseURL: testInfo.project.use.baseURL, viewport: { width, height: 900 } });
      try {
        const visitor = await anonymous.newPage();
        await visitor.goto("/login");
        await expect(visitor).toHaveTitle("Домашняя сеть");
        await expect(visitor.getByRole("heading", { name: "Домашняя сеть" })).toBeVisible();
        await expect(visitor.locator('img[src*="/api/branding/logo"]')).toHaveCount(1);
        const html = await (await visitor.request.get("/login")).text();
        expect(html).toContain("<title>Домашняя сеть</title>");
        expect(html).not.toContain(logoPath);
        expect((await visitor.request.get(endpoint)).status()).toBe(401);
        const manifest = await (await visitor.request.get("/manifest.webmanifest")).json();
        expect(manifest.name).toBe("Домашняя сеть");
        expect(manifest.icons[0].src).toContain("/api/branding/icon");
        await visitor.screenshot({ path: testInfo.outputPath("branding-login.png") });
        await card.getByRole("button", { name: "Настроить оформление" }).click();
        await dialog.getByRole("combobox", { name: "Логотип" }).selectOption("hidden");
        await dialog.getByRole("button", { name: "Сохранить оформление" }).click();
        await expect(dialog).not.toBeVisible();
        await expect(card.locator("img")).toHaveCount(0);
        await visitor.reload();
        await expect(visitor.getByRole("heading", { name: "Домашняя сеть" })).toBeVisible();
        await expect(visitor.locator("img")).toHaveCount(0);
        await visitor.screenshot({ path: testInfo.outputPath("branding-hidden.png") });
      } finally { await anonymous.close(); }

      await card.getByRole("button", { name: "Настроить оформление" }).click();
      await dialog.getByRole("button", { name: "Вернуть стандартное" }).click();
      await dialog.getByRole("button", { name: "Сохранить оформление" }).click();
      await expect(dialog).not.toBeVisible();
      await expect(page).toHaveTitle("Telemt Panel");
    } finally {
      const restored = await page.request.put(endpoint, { data: defaults, headers: { "Sec-Fetch-Site": "same-origin" } });
      expect(restored.status()).toBe(200);
    }
  });
}
