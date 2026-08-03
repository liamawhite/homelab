import { Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import type { ItineraryItem } from "@/gen/trips/v1/itinerary_pb";
import { minutesToClock, zoneAbbreviation } from "@/lib/time";
import { MapsLink } from "@/components/MapsLink";
import { HOUR_HEIGHT_PX, yFromMinutes } from "./grid";

interface ScheduledBlockContentProps {
  item: ItineraryItem;
  onDelete: (id: string) => void;
  deleting: boolean;
  // Only set for a TIMED segment - the clock range visible on this
  // specific day (see TimedBlock's doc comment on why this can differ
  // from item.startMinute/item.endMinute for a multi-day item).
  timeLabel?: string;
}

function ScheduledBlockContent({
  item,
  onDelete,
  deleting,
  timeLabel,
}: ScheduledBlockContentProps) {
  return (
    <div className="flex h-full flex-col justify-between gap-0.5 overflow-hidden p-1 text-xs">
      <div className="flex items-start justify-between gap-1">
        <span className="font-medium">{item.title}</span>
        <Button
          variant="ghost"
          size="icon-xs"
          aria-label={`Delete ${item.title}`}
          disabled={deleting}
          onClick={(e) => {
            e.stopPropagation();
            onDelete(item.id);
          }}
        >
          <Trash2 />
        </Button>
      </div>
      {timeLabel && <span className="text-muted-foreground">{timeLabel}</span>}
      <MapsLink
        latitude={item.latitude}
        longitude={item.longitude}
        placeId={item.placeId}
      />
    </div>
  );
}

interface AllDayBlockProps {
  item: ItineraryItem;
  onClick: (item: ItineraryItem) => void;
  onDelete: (id: string) => void;
  deleting: boolean;
}

// An ALL_DAY item's chip in the all-day row - rendered once per day it
// spans (date through end_date inclusive), same repeat-per-column approach
// as the Accommodation row. Clicking it opens the schedule-edit modal.
export function AllDayBlock({
  item,
  onClick,
  onDelete,
  deleting,
}: AllDayBlockProps) {
  return (
    <button
      type="button"
      onClick={() => onClick(item)}
      className="w-full rounded-md border border-border bg-card text-left hover:bg-muted"
    >
      <ScheduledBlockContent
        item={item}
        onDelete={onDelete}
        deleting={deleting}
      />
    </button>
  );
}

interface TimedBlockProps {
  item: ItineraryItem;
  // The portion of item's full start_minute/end_minute span visible on
  // this particular day column - for a single-day item this just equals
  // item.startMinute/item.endMinute, but a multi-day item (end_date >
  // date) is split into one segment per day it touches (see
  // CalendarGrid's use of grid.ts's segmentForDay), each rendered as its
  // own TimedBlock instance in that day's column.
  segmentStartMinute: number;
  segmentEndMinute: number;
  onClick: (item: ItineraryItem) => void;
  onDelete: (id: string) => void;
  deleting: boolean;
}

// A TIMED item's block in the hourly grid, absolutely positioned via its
// segment's start/end minute against HOUR_HEIGHT_PX. Clicking it opens the
// schedule-edit modal (which also handles resizing/moving/unscheduling/
// spanning multiple days).
export function TimedBlock({
  item,
  segmentStartMinute,
  segmentEndMinute,
  onClick,
  onDelete,
  deleting,
}: TimedBlockProps) {
  const top = yFromMinutes(segmentStartMinute, HOUR_HEIGHT_PX);
  const height = Math.max(
    yFromMinutes(segmentEndMinute - segmentStartMinute, HOUR_HEIGHT_PX),
    16,
  );
  const timeLabel = `${minutesToClock(segmentStartMinute)}–${minutesToClock(segmentEndMinute)}${
    item.timezone ? ` ${zoneAbbreviation(item.date, item.timezone)}` : ""
  }`;

  return (
    <button
      type="button"
      onClick={() => onClick(item)}
      style={{ top, height }}
      className="absolute inset-x-0 rounded-md border border-border bg-card text-left hover:bg-muted"
    >
      <ScheduledBlockContent
        item={item}
        onDelete={onDelete}
        deleting={deleting}
        timeLabel={timeLabel}
      />
    </button>
  );
}
