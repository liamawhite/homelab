// Shared formatting helpers for the read-only Light/Switch/Group/Scene/
// CircadianSchedule tables - kept in one place since every page needs the
// same "-1/0 means unsupported" sentinel handling the CRDs use (see
// applications/lumenetes/api/v1alpha1's doc comments).

import { ActiveSceneKind } from "@/gen/lumenetes/v1/group_pb";

export function formatBrightness(brightness: number): string {
  return brightness < 0 ? "—" : `${brightness}%`;
}

export function formatColorTempK(colorTempK: number): string {
  return colorTempK <= 0 ? "—" : `${colorTempK}K`;
}

export function formatColor(color: string): string {
  return color === "" ? "—" : color;
}

export function activeSceneKindLabel(kind: ActiveSceneKind): string {
  switch (kind) {
    case ActiveSceneKind.SCENE:
      return "Scene";
    case ActiveSceneKind.CIRCADIAN_SCHEDULE:
      return "Circadian schedule";
    case ActiveSceneKind.OFF:
      return "Off";
    case ActiveSceneKind.REACTIVE:
      return "Reactive";
    default:
      return "Unmanaged";
  }
}
