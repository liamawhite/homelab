import { useSchedules } from "@/lib/schedules";
import { formatBrightness, formatColorTempK } from "@/lib/format";
import { CircadianAnchor } from "@/gen/lumenetes/v1/circadian_schedule_pb";
import { Badge } from "@/components/ui/badge";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";

function anchorLabel(anchor: CircadianAnchor): string {
  switch (anchor) {
    case CircadianAnchor.SUNRISE:
      return "sunrise";
    case CircadianAnchor.SOLAR_NOON:
      return "solar noon";
    case CircadianAnchor.SUNSET:
      return "sunset";
    case CircadianAnchor.SOLAR_MIDNIGHT:
      return "solar midnight";
    default:
      return "unknown";
  }
}

function offsetLabel(minutes: number): string {
  if (minutes === 0) return "";
  const sign = minutes > 0 ? "+" : "-";
  return ` ${sign}${Math.abs(minutes)}m`;
}

export function SchedulesPage() {
  const { data: schedules, isLoading, isError, error } = useSchedules();

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-4 p-4">
      <h1 className="text-lg font-semibold">Schedules</h1>

      {isLoading && <p className="text-sm text-muted-foreground">Loading schedules…</p>}
      {isError && <p className="text-sm text-destructive">{error.message}</p>}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        {schedules?.map((schedule) => (
          <Card key={schedule.id}>
            <CardHeader>
              <CardTitle className="flex items-center justify-between">
                <span>{schedule.id}</span>
                {schedule.validationError ? (
                  <Badge variant="destructive">{schedule.validationError}</Badge>
                ) : (
                  <Badge variant="default">
                    {schedule.currentBrightness !== undefined && formatBrightness(schedule.currentBrightness)}
                    {schedule.currentColorTempK !== undefined && ` · ${formatColorTempK(schedule.currentColorTempK)}`}
                  </Badge>
                )}
              </CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-2 text-sm">
              <p className="text-muted-foreground">Group: {schedule.group}</p>
              <ul className="list-inside list-disc text-muted-foreground">
                {schedule.keyframes.map((kf, i) => (
                  <li key={i}>
                    {anchorLabel(kf.anchor)}
                    {offsetLabel(kf.offsetMinutes)}: {formatBrightness(kf.brightness)}, {formatColorTempK(kf.colorTempK)}
                  </li>
                ))}
              </ul>
            </CardContent>
          </Card>
        ))}
      </div>
      {schedules && schedules.length === 0 && (
        <p className="text-sm text-muted-foreground">No schedules found.</p>
      )}
    </div>
  );
}
