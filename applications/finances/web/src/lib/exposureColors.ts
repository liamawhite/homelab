// Validated categorical order (node scripts/validate_palette.js) - shared
// between NetWorthChart's hover tooltip and ExposureChart's bars, so the
// same band always carries the same color everywhere on the page. The
// three "<region> Real Estate" bands share Real Estate's hue - they're one
// identity (real estate risk) split by geography, not distinct series. The
// "<region> ETF"/"<region> Individual Stock" bands likewise share their
// region's hue - geography is the color axis; vehicle type (fund vs
// single name) is conveyed by the label text, not a second color per
// region (that would blow well past the 8-hue budget for identity that's
// still readable as text).
export const EXPOSURE_COLORS: Record<string, { light: string; dark: string }> = {
  US: { light: "#2a78d6", dark: "#3987e5" }, // slot 1 blue
  "US ETF": { light: "#2a78d6", dark: "#3987e5" }, // slot 1 blue
  "US Individual Stock": { light: "#2a78d6", dark: "#3987e5" }, // slot 1 blue
  International: { light: "#eb6834", dark: "#d95926" }, // slot 2 orange
  "International ETF": { light: "#eb6834", dark: "#d95926" }, // slot 2 orange
  "International Individual Stock": { light: "#eb6834", dark: "#d95926" }, // slot 2 orange
  UK: { light: "#1baf7a", dark: "#199e70" }, // slot 3 aqua
  "UK ETF": { light: "#1baf7a", dark: "#199e70" }, // slot 3 aqua
  "UK Individual Stock": { light: "#1baf7a", dark: "#199e70" }, // slot 3 aqua
  Bonds: { light: "#eda100", dark: "#c98500" }, // slot 4 yellow
  "US Real Estate": { light: "#e87ba4", dark: "#d55181" }, // slot 5 magenta
  "International Real Estate": { light: "#e87ba4", dark: "#d55181" }, // slot 5 magenta
  "UK Real Estate": { light: "#e87ba4", dark: "#d55181" }, // slot 5 magenta
  Loan: { light: "#008300", dark: "#008300" }, // slot 6 green
  Mortgage: { light: "#4a3aa7", dark: "#9085e9" }, // slot 7 violet
  Cash: { light: "#e34948", dark: "#e66767" }, // slot 8 red
  // Unallocated/Commodity/Crypto intentionally have no assigned hue - they
  // fall back to muted gray, which reads correctly as "no [geographic]
  // allocation" rather than a series.
};
