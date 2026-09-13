import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { getBrandingSettingsOptions, getBrandingSettingsQueryKey, getPublicBrandingQueryKey, putBrandingSettingsMutation } from "../../lib/api/generated/@tanstack/react-query.gen";
import type { BrandingConfig } from "../../lib/api/generated/types.gen";
import { PanelLogo } from "../../branding/branding";
import { useBranding } from "../../branding/useBranding";
import { useStrings } from "../../i18n";
import { apiErrorMessage } from "../../people/apiError";
import { Button } from "../../ui/Button";
import { Input } from "../../ui/Input";
import { Select } from "../../ui/Select";
import { Sheet } from "../../ui/Sheet";
import { pushToast } from "../../ui/Toast";

export function BrandingSettings() {
  const s = useStrings();
  const t = s.server.settings.branding;
  const branding = useBranding();
  const queryClient = useQueryClient();
  const settings = useQuery(getBrandingSettingsOptions());
  const [draft, setDraft] = useState<BrandingConfig | null>(null);
  const save = useMutation({
    ...putBrandingSettingsMutation(),
    onSuccess: (data) => {
      queryClient.setQueryData(getBrandingSettingsQueryKey(), data);
      queryClient.setQueryData(getPublicBrandingQueryKey(), data.public);
      setDraft(null);
      pushToast(t.saved, "ok");
    },
  });
  return <>
    <section data-testid="settings-branding" className="overflow-hidden rounded-xl bg-surface">
      <div className="border-b border-border px-4 py-3.5">
        <span className="text-xs font-semibold text-text-muted">{t.scope}</span>
        <h2 className="mt-1 text-base font-bold text-text">{t.title}</h2>
        <p className="mt-2 text-sm leading-relaxed text-text-muted">{t.note}</p>
      </div>
      <div className="space-y-4 p-4">
        <div aria-label={t.preview} className="flex min-h-16 items-center gap-3 rounded-lg bg-surface-2 px-4 py-3">
          <PanelLogo branding={branding} className="h-10 w-10 shrink-0 object-contain" />
          <span className="min-w-0 break-words text-base font-semibold text-text">{branding.title}</span>
        </div>
        {settings.data?.logo_status === "unavailable" && <p role="status" className="text-sm text-warn">{t.unavailable}</p>}
        {settings.error && <p role="alert" className="text-sm text-error">{apiErrorMessage(settings.error, s)}</p>}
        <Button variant="secondary" disabled={!settings.data} onClick={() => {
          if (!settings.data) return;
          save.reset();
          const { title, logo_mode, logo_path } = settings.data;
          setDraft({ title, logo_mode, logo_path });
        }}>{t.edit}</Button>
      </div>
    </section>
    <Sheet open={draft !== null} onClose={() => { if (!save.isPending) setDraft(null); }} title={t.title} subtitle={t.scope}>
      {draft && <form className="space-y-5 p-4" onSubmit={(event) => { event.preventDefault(); save.mutate({ body: draft }); }}>
        <label className="block space-y-2 text-sm font-semibold text-text">
          <span>{t.name}</span>
          <Input value={draft.title} maxLength={80} required disabled={save.isPending} onChange={(event) => setDraft({ ...draft, title: event.target.value })} />
        </label>
        <label className="block space-y-2 text-sm font-semibold text-text">
          <span>{t.logo}</span>
          <Select value={draft.logo_mode} disabled={save.isPending} onChange={(event) => setDraft({ ...draft, logo_mode: event.target.value as BrandingConfig["logo_mode"] })}>
            {(["default", "custom", "hidden"] as const).map((mode) => <option key={mode} value={mode}>{t[mode]}</option>)}
          </Select>
        </label>
        {draft.logo_mode === "custom" && <div className="space-y-2">
          <label className="block space-y-2 text-sm font-semibold text-text">
            <span>{t.path}</span>
            <Input value={draft.logo_path} required disabled={save.isPending} autoCapitalize="off" autoComplete="off" spellCheck={false} placeholder="/var/lib/telemt-panel/logo.webp" onChange={(event) => setDraft({ ...draft, logo_path: event.target.value })} />
          </label>
          <p className="text-sm leading-relaxed text-text-muted">{t.pathNote}</p>
          <p className="text-sm text-text-muted">{t.limits}</p>
          <p className="text-sm leading-relaxed text-text-muted">{t.reload}</p>
        </div>}
        {save.error && <p role="alert" className="rounded-lg bg-error/10 p-3 text-sm text-error">{apiErrorMessage(save.error, s)}</p>}
        <p className="text-sm leading-relaxed text-text-muted">{t.privacy}</p>
        <div className="flex flex-wrap gap-2 border-t border-border pt-4">
          <Button type="submit" disabled={save.isPending || !draft.title.trim()}>{t.save}</Button>
          <Button type="button" variant="secondary" disabled={save.isPending} onClick={() => setDraft({ title: "Telemt Panel", logo_mode: "default", logo_path: "" })}>{t.reset}</Button>
        </div>
      </form>}
    </Sheet>
  </>;
}
