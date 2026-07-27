// Kelvin -> RGB approximation (Tanner Helland's black-body radiation fit),
// used to give a color-temperature-only light (no xy color set) the same
// kind of visual swatch a color-managed light gets from its #rrggbb value.
function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

function kelvinToRgb(kelvin: number): [number, number, number] {
  const temp = kelvin / 100;

  const red = temp <= 66 ? 255 : clamp(329.698727446 * Math.pow(temp - 60, -0.1332047592), 0, 255);

  const green = clamp(
    temp <= 66 ? 99.4708025861 * Math.log(temp) - 161.1195681661 : 288.1221695283 * Math.pow(temp - 60, -0.0755148492),
    0,
    255,
  );

  const blue = temp >= 66 ? 255 : temp <= 19 ? 0 : clamp(138.5177312231 * Math.log(temp - 10) - 305.0447927307, 0, 255);

  return [Math.round(red), Math.round(green), Math.round(blue)];
}

function toHex(component: number): string {
  return component.toString(16).padStart(2, "0");
}

export function kelvinToHex(kelvin: number): string {
  const [r, g, b] = kelvinToRgb(kelvin);
  return `#${toHex(r)}${toHex(g)}${toHex(b)}`;
}

// hexToRgba renders a "#rrggbb" swatch color as a translucent background
// tint - full opacity would make light/badge text unreadable regardless of
// theme, so callers use this rather than the raw color for backgrounds.
export function hexToRgba(hex: string, alpha: number): string {
  const value = hex.replace("#", "");
  const r = parseInt(value.slice(0, 2), 16);
  const g = parseInt(value.slice(2, 4), 16);
  const b = parseInt(value.slice(4, 6), 16);
  return `rgba(${r}, ${g}, ${b}, ${alpha})`;
}

// lightSwatchColor is the color a Light's box should visually reflect: its
// explicit color if color-managed, else an approximation of its color
// temperature, else none (no color/temp support, or the light is off - an
// off light shows no tint regardless of its last-known color/temp).
export function lightSwatchColor(light: { observedOn: boolean; observedColor: string; observedColorTempK: number }): string | undefined {
  if (!light.observedOn) return undefined;
  if (light.observedColor) return light.observedColor;
  if (light.observedColorTempK > 0) return kelvinToHex(light.observedColorTempK);
  return undefined;
}
