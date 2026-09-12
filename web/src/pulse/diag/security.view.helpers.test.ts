import { describe, expect, it } from "vitest";
import { en, ru } from "../../i18n/testing";
import { duration, tlsSeenAt } from "./security.view.helpers";

describe("security duration", () => {
  it.each([
    [0, "0 s", "0 с"],
    [59, "59 s", "59 с"],
    [60, "1 min", "1 мин"],
    [61, "61 s", "61 с"],
    [120, "2 min", "2 мин"],
    [3600, "60 min", "60 мин"],
    [1.5, "1.5 s", "1,5 с"],
    [-60, "-60 s", "-60 с"],
  ])("preserves exact units and locale for %s seconds", (seconds, english, russian) => {
    expect(duration(en, seconds)).toBe(english);
    expect(duration(ru, seconds)).toBe(russian);
  });
});

describe("TLS observation timestamps", () => {
  it.each([0, -1, NaN, Infinity, 1e20])("does not display an invalid time (%s)", (value) => {
    expect(tlsSeenAt(en, value)).toBe("—");
  });
  it("uses the current locale and includes the date and time", () => {
    const epoch = 1789214400;
    expect(tlsSeenAt(en, epoch)).toBe(new Date(epoch * 1000).toLocaleString(en.locale, { dateStyle: "short", timeStyle: "medium" }));
    expect(tlsSeenAt(ru, epoch)).not.toBe(tlsSeenAt(en, epoch));
  });
});
