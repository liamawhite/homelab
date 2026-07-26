// Must stay in sync with storage.LabelPalette (Go) and the CHECK constraint
// in 0006_add_labels_color.sql - the server rejects any other value.
export const LABEL_PALETTE = [
  "#e03131", "#f08c00", "#f5c518", "#2f9e44", "#0ca678",
  "#1971c2", "#4263eb", "#7048e8", "#ae3ec9", "#e64980",
];

// Picks black or white text for a given hex background, using perceived
// brightness (YIQ) rather than a straight average - good enough for the
// small fixed label palette (see storage.LabelPalette on the Go side).
export function contrastTextColor(hex: string): string {
  const clean = hex.replace("#", "");
  const r = parseInt(clean.slice(0, 2), 16);
  const g = parseInt(clean.slice(2, 4), 16);
  const b = parseInt(clean.slice(4, 6), 16);
  const yiq = (r * 299 + g * 587 + b * 114) / 1000;
  return yiq >= 150 ? "#000000" : "#ffffff";
}
