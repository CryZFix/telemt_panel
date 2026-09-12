export interface DetailSearch {
  entity?: string;
  tab?: string;
  tlsScope?: "by_fingerprint" | "by_ip" | "by_cidr" | "by_user";
  tlsFilter?: "suspicious";
}

const MAX_SEARCH_VALUE_LENGTH = 256;

function searchString(value: unknown): string | undefined {
  if (typeof value !== "string") return undefined;
  const normalized = value.trim().slice(0, MAX_SEARCH_VALUE_LENGTH);
  return normalized === "" ? undefined : normalized;
}

export function validateDetailSearch(search: Record<string, unknown>): DetailSearch {
  const entity = searchString(search["entity"]);
  const tab = searchString(search["tab"]);
  const scope = search["tlsScope"];
  const tlsScope = scope === "by_fingerprint" || scope === "by_ip" || scope === "by_cidr" || scope === "by_user" ? scope : undefined;
  return {
    ...(entity !== undefined ? { entity } : {}),
    ...(tab !== undefined ? { tab } : {}),
    ...(tlsScope !== undefined ? { tlsScope } : {}),
    ...(search["tlsFilter"] === "suspicious" ? { tlsFilter: "suspicious" as const } : {}),
  };
}
