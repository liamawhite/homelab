import { useState } from "react";
import { Trash2 } from "lucide-react";

import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  ItineraryItemState,
  type ItineraryItem,
} from "@/gen/trips/v1/itinerary_pb";
import { MapsLink } from "@/components/MapsLink";
import {
  DEFAULT_DURATION_MINUTES,
  minutesToTimeInput,
  timeInputToMinutes,
} from "./grid";
import { zoneAbbreviation } from "@/lib/time";
import { IANA_TIMEZONES } from "./timezones";

function todayDateInput(): string {
  const now = new Date();
  const month = String(now.getMonth() + 1).padStart(2, "0");
  const day = String(now.getDate()).padStart(2, "0");
  return `${now.getFullYear()}-${month}-${day}`;
}

interface ScheduleInput {
  state: ItineraryItemState;
  date: string;
  endDate: string;
  startMinute: number;
  endMinute: number;
  timezone: string;
}

interface ItemEditModalContentProps {
  item: ItineraryItem;
  onSchedule: (input: ScheduleInput) => void;
  onDelete: (id: string) => void;
  onClose: () => void;
  saving: boolean;
}

const STATE_OPTIONS = [
  { value: ItineraryItemState.UNSCHEDULED, label: "Unscheduled" },
  { value: ItineraryItemState.ALL_DAY, label: "All day" },
  { value: ItineraryItemState.TIMED, label: "Timed" },
] as const;

function ItemEditModalContent({
  item,
  onSchedule,
  onDelete,
  onClose,
  saving,
}: ItemEditModalContentProps) {
  const [state, setState] = useState(item.state);
  const [date, setDate] = useState(item.date || todayDateInput());
  const [endDate, setEndDate] = useState(
    item.endDate || item.date || todayDateInput(),
  );
  const [startMinute, setStartMinute] = useState(
    item.state === ItineraryItemState.TIMED ? item.startMinute : 9 * 60,
  );
  const [endMinute, setEndMinute] = useState(
    item.state === ItineraryItemState.TIMED
      ? item.endMinute
      : 9 * 60 + DEFAULT_DURATION_MINUTES,
  );
  const [timezone, setTimezone] = useState(item.timezone);

  const spansMultipleDays = endDate !== date;
  const endDateInvalid = endDate < date;
  const timedInvalid =
    state === ItineraryItemState.TIMED &&
    !spansMultipleDays &&
    endMinute <= startMinute;

  const handleSave = () => {
    if (state === ItineraryItemState.UNSCHEDULED) {
      onSchedule({
        state,
        date: "",
        endDate: "",
        startMinute: 0,
        endMinute: 0,
        timezone,
      });
    } else if (state === ItineraryItemState.ALL_DAY) {
      if (endDateInvalid) return;
      onSchedule({
        state,
        date,
        endDate,
        startMinute: 0,
        endMinute: 0,
        timezone,
      });
    } else {
      if (endDateInvalid || timedInvalid) return;
      onSchedule({ state, date, endDate, startMinute, endMinute, timezone });
    }
    onClose();
  };

  return (
    <DialogContent>
      <DialogHeader>
        <DialogTitle>{item.title}</DialogTitle>
      </DialogHeader>

      <MapsLink
        latitude={item.latitude}
        longitude={item.longitude}
        placeId={item.placeId}
      />

      <div className="flex flex-col gap-3">
        <div className="flex flex-col gap-1.5">
          <Label>Schedule</Label>
          <div className="flex gap-1">
            {STATE_OPTIONS.map((option) => (
              <Button
                key={option.value}
                type="button"
                variant={state === option.value ? "default" : "outline"}
                size="sm"
                onClick={() => setState(option.value)}
              >
                {option.label}
              </Button>
            ))}
          </div>
        </div>

        {state !== ItineraryItemState.UNSCHEDULED && (
          <div className="flex gap-2">
            <div className="flex flex-1 flex-col gap-1.5">
              <Label htmlFor="item-date">Starts</Label>
              <Input
                id="item-date"
                type="date"
                value={date}
                onChange={(e) => setDate(e.target.value)}
              />
            </div>
            <div className="flex flex-1 flex-col gap-1.5">
              <Label htmlFor="item-end-date">Ends</Label>
              <Input
                id="item-end-date"
                type="date"
                value={endDate}
                onChange={(e) => setEndDate(e.target.value)}
              />
            </div>
          </div>
        )}
        {endDateInvalid && (
          <p className="text-sm text-destructive">
            End date must not be before start date.
          </p>
        )}

        <div className="flex flex-col gap-1.5">
          <Label htmlFor="item-timezone">Timezone</Label>
          <select
            id="item-timezone"
            value={timezone}
            onChange={(e) => setTimezone(e.target.value)}
            className="rounded-lg border border-input bg-transparent px-2.5 py-1.5 text-sm"
          >
            {IANA_TIMEZONES.map((tz) => (
              <option key={tz} value={tz}>
                {tz}
              </option>
            ))}
          </select>
          {state !== ItineraryItemState.UNSCHEDULED && (
            <p className="text-xs text-muted-foreground">
              Relabels the times below - it doesn't move this item on the
              calendar. Currently {zoneAbbreviation(date, timezone)}.
            </p>
          )}
        </div>

        {state === ItineraryItemState.TIMED && (
          <div className="flex gap-2">
            <div className="flex flex-1 flex-col gap-1.5">
              <Label htmlFor="item-start">Start time</Label>
              <Input
                id="item-start"
                type="time"
                value={minutesToTimeInput(startMinute)}
                onChange={(e) =>
                  setStartMinute(timeInputToMinutes(e.target.value))
                }
              />
            </div>
            <div className="flex flex-1 flex-col gap-1.5">
              <Label htmlFor="item-end">
                End time{spansMultipleDays && " (on end date)"}
              </Label>
              <Input
                id="item-end"
                type="time"
                value={minutesToTimeInput(endMinute)}
                onChange={(e) =>
                  setEndMinute(timeInputToMinutes(e.target.value))
                }
              />
            </div>
          </div>
        )}
        {timedInvalid && (
          <p className="text-sm text-destructive">
            End time must be after start time.
          </p>
        )}
      </div>

      <DialogFooter className="sm:justify-between">
        <Button
          type="button"
          variant="destructive"
          onClick={() => {
            onDelete(item.id);
            onClose();
          }}
        >
          <Trash2 /> Delete
        </Button>
        <Button
          type="button"
          onClick={handleSave}
          disabled={saving || timedInvalid || endDateInvalid}
        >
          Save
        </Button>
      </DialogFooter>
    </DialogContent>
  );
}

interface ItemEditModalProps {
  item: ItineraryItem | null;
  onClose: () => void;
  onSchedule: (input: ScheduleInput) => void;
  onDelete: (id: string) => void;
  saving: boolean;
}

// Clicking any itinerary item (backlog or scheduled) opens this modal to
// set its placement - replaces drag/resize/drag-to-unschedule with plain
// form controls.
export function ItemEditModal({
  item,
  onClose,
  onSchedule,
  onDelete,
  saving,
}: ItemEditModalProps) {
  return (
    <Dialog open={!!item} onOpenChange={(open) => !open && onClose()}>
      {item && (
        <ItemEditModalContent
          key={item.id}
          item={item}
          onSchedule={onSchedule}
          onDelete={onDelete}
          onClose={onClose}
          saving={saving}
        />
      )}
    </Dialog>
  );
}
