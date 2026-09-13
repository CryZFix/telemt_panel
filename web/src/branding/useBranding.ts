import { useQuery } from "@tanstack/react-query";
import { getPublicBrandingOptions } from "../lib/api/generated/@tanstack/react-query.gen";
import type { PublicBranding } from "../lib/api/generated/types.gen";

declare global {
  interface Window { __PANEL_BRANDING__?: PublicBranding }
}

export function initialBranding(): PublicBranding {
  return window.__PANEL_BRANDING__ ?? { title: "Telemt Panel", logo_mode: "default", logo_url: "", icon_url: "" };
}

export function useBranding() {
  return useQuery({ ...getPublicBrandingOptions(), initialData: initialBranding, staleTime: 30_000 }).data;
}
