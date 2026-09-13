import { useEffect } from "react";
import type { PublicBranding } from "../lib/api/generated/types.gen";
import { withBasePath } from "../lib/base-path";
import loginLogo from "../assets/logo-login.webp";
import menuLogo from "../assets/logo-menu.webp";
import { useBranding } from "./useBranding";

export function BrandingDocument() {
  const branding = useBranding();
  useEffect(() => {
    window.__PANEL_BRANDING__ = branding;
    document.title = branding.title;
    document.querySelector('meta[name="apple-mobile-web-app-title"]')?.setAttribute("content", branding.title);
    const favicon = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
    if (favicon) {
      favicon.href = branding.icon_url || withBasePath("/icon.svg");
      favicon.type = branding.icon_url ? "image/png" : "image/svg+xml";
    }
    document.querySelector('link[rel="apple-touch-icon"]')?.setAttribute("href", branding.icon_url || withBasePath("/apple-touch-icon.png"));
  }, [branding]);
  return null;
}

export function PanelLogo({ branding, login = false, className }: { branding: PublicBranding; login?: boolean; className?: string }) {
  const src = branding.logo_mode === "default" ? (login ? loginLogo : menuLogo) : branding.logo_url;
  if (branding.logo_mode === "hidden" || !src) return null;
  return <img key={src} src={src} alt="" aria-hidden="true" className={className} onError={(event) => { event.currentTarget.style.visibility = "hidden"; }} />;
}
