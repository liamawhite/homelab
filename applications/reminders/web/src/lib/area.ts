import { Area } from "@/gen/reminders/v1/one_off_item_pb";

// The selectable areas, in display order. AREA_UNSPECIFIED is deliberately
// excluded - it's a wire-format sentinel, never a real value.
export const AREA_OPTIONS: Area[] = [Area.PERSONAL, Area.FAMILY, Area.WORK, Area.PROFESSIONAL];

export const AREA_LABELS: Record<Area, string> = {
  [Area.UNSPECIFIED]: "personal",
  [Area.PERSONAL]: "personal",
  [Area.FAMILY]: "family",
  [Area.WORK]: "work",
  [Area.PROFESSIONAL]: "professional",
};

export const AREA_BY_LABEL: Record<string, Area> = {
  personal: Area.PERSONAL,
  family: Area.FAMILY,
  work: Area.WORK,
  professional: Area.PROFESSIONAL,
};

// Fixed categorical hues (see index.css's --area-* tokens) - always paired
// with the "@label" text, never color-alone.
export const AREA_TEXT_CLASSES: Record<Area, string> = {
  [Area.UNSPECIFIED]: "text-area-personal",
  [Area.PERSONAL]: "text-area-personal",
  [Area.FAMILY]: "text-area-family",
  [Area.WORK]: "text-area-work",
  [Area.PROFESSIONAL]: "text-area-professional",
};

export const AREA_BORDER_CLASSES: Record<Area, string> = {
  [Area.UNSPECIFIED]: "border-area-personal/40",
  [Area.PERSONAL]: "border-area-personal/40",
  [Area.FAMILY]: "border-area-family/40",
  [Area.WORK]: "border-area-work/40",
  [Area.PROFESSIONAL]: "border-area-professional/40",
};
