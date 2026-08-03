import { useMemo, useState } from "react";

import {
  useItineraryItems,
  useAddItineraryItem,
  useScheduleItineraryItem,
  useDeleteItineraryItem,
} from "@/lib/itinerary";
import {
  ItineraryItemState,
  type ItineraryItem,
  type LocationCandidate,
} from "@/gen/trips/v1/itinerary_pb";
import type { Flight } from "@/gen/trips/v1/flight_pb";
import type { Accommodation } from "@/gen/trips/v1/accommodation_pb";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Backlog } from "./Backlog";
import { CalendarGrid } from "./CalendarGrid";
import { LocationSearchInput } from "./LocationSearchInput";
import { ItemEditModal } from "./ItemEditModal";
import { computeDateRange } from "./dateRange";
import {
  IANA_TIMEZONES,
  loadDefaultTimezone,
  saveDefaultTimezone,
} from "./timezones";

interface ItinerarySectionProps {
  tripId: string;
  flights: Flight[];
  accommodations: Accommodation[];
}

// Top-level itinerary section: the "add unscheduled item" form, the
// unscheduled backlog card, the calendar grid, and the derived day range
// they're both rendered against. Clicking any item (backlog or scheduled)
// opens ItemEditModal, which is the only way to change an item's placement
// - see that component's doc comment.
export function ItinerarySection({
  tripId,
  flights,
  accommodations,
}: ItinerarySectionProps) {
  const { data: items, isLoading, isError, error } = useItineraryItems(tripId);
  const addItem = useAddItineraryItem(tripId);
  const scheduleItem = useScheduleItineraryItem(tripId);
  const deleteItem = useDeleteItineraryItem(tripId);

  const [title, setTitle] = useState("");
  const [location, setLocation] = useState<LocationCandidate | null>(null);
  const [editingItem, setEditingItem] = useState<ItineraryItem | null>(null);
  // Persisted to localStorage (not the backend - it's a client-side
  // default applied to new items, see handleAdd). Existing items keep
  // whatever timezone they were created with regardless of this selection.
  const [defaultTimezone, setDefaultTimezoneState] =
    useState(loadDefaultTimezone);
  const setDefaultTimezone = (tz: string) => {
    setDefaultTimezoneState(tz);
    saveDefaultTimezone(tz);
  };

  const days = useMemo(
    () => computeDateRange(flights, accommodations, items ?? []),
    [flights, accommodations, items],
  );

  const handleAdd = () => {
    const trimmed = title.trim();
    if (!trimmed) return;
    addItem.mutate(
      { title: trimmed, placeId: location?.placeId, timezone: defaultTimezone },
      {
        onSuccess: () => {
          setTitle("");
          setLocation(null);
        },
      },
    );
  };

  const unscheduled =
    items?.filter((item) => item.state === ItineraryItemState.UNSCHEDULED) ??
    [];

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap gap-2">
        <Input
          placeholder="Title (e.g. Visit the museum)"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") handleAdd();
          }}
          className="min-w-48 flex-1"
        />
        <div className="min-w-48 flex-1">
          <LocationSearchInput selected={location} onSelect={setLocation} />
        </div>
        <Button
          onClick={handleAdd}
          disabled={addItem.isPending || !title.trim()}
        >
          Add item
        </Button>
      </div>

      {isLoading && (
        <p className="text-sm text-muted-foreground">Loading itinerary…</p>
      )}
      {isError && <p className="text-sm text-destructive">{error.message}</p>}
      {addItem.isError && (
        <p className="text-sm text-destructive">{addItem.error.message}</p>
      )}
      {scheduleItem.isError && (
        <p className="text-sm text-destructive">{scheduleItem.error.message}</p>
      )}
      {deleteItem.isError && (
        <p className="text-sm text-destructive">{deleteItem.error.message}</p>
      )}

      {items && (
        <div className="flex flex-col gap-3">
          <Card size="sm">
            <CardHeader>
              <CardTitle>Unscheduled</CardTitle>
            </CardHeader>
            <CardContent>
              <Backlog
                items={unscheduled}
                onItemClick={setEditingItem}
                onDelete={(id) => deleteItem.mutate(id)}
                deleting={deleteItem.isPending}
              />
            </CardContent>
          </Card>
          <div className="flex items-center justify-end gap-2 text-sm">
            <label
              htmlFor="itinerary-timezone"
              className="text-muted-foreground"
            >
              New items in
            </label>
            <select
              id="itinerary-timezone"
              value={defaultTimezone}
              onChange={(e) => setDefaultTimezone(e.target.value)}
              className="rounded-lg border border-input bg-transparent px-2 py-1 text-sm"
            >
              {IANA_TIMEZONES.map((tz) => (
                <option key={tz} value={tz}>
                  {tz}
                </option>
              ))}
            </select>
          </div>
          <CalendarGrid
            days={days}
            items={items}
            flights={flights}
            accommodations={accommodations}
            onItemClick={setEditingItem}
            onDelete={(id) => deleteItem.mutate(id)}
            deleting={deleteItem.isPending}
          />
        </div>
      )}

      <ItemEditModal
        item={editingItem}
        onClose={() => setEditingItem(null)}
        saving={scheduleItem.isPending}
        onSchedule={(input) => {
          if (!editingItem) return;
          scheduleItem.mutate({ id: editingItem.id, ...input });
        }}
        onDelete={(id) => deleteItem.mutate(id)}
      />
    </div>
  );
}
