import { Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import type { ItineraryItem } from "@/gen/trips/v1/itinerary_pb";
import { MapsLink } from "@/components/MapsLink";

interface BacklogItemProps {
  item: ItineraryItem;
  onClick: (item: ItineraryItem) => void;
  onDelete: (id: string) => void;
  deleting: boolean;
}

function BacklogItem({ item, onClick, onDelete, deleting }: BacklogItemProps) {
  return (
    <button
      type="button"
      onClick={() => onClick(item)}
      className="flex w-56 shrink-0 items-center justify-between gap-2 rounded-lg border border-border bg-card p-2 text-left text-sm hover:bg-muted"
    >
      <div className="flex flex-col overflow-hidden">
        <span className="truncate">{item.title}</span>
        <MapsLink
          latitude={item.latitude}
          longitude={item.longitude}
          placeId={item.placeId}
        />
      </div>
      <Button
        variant="ghost"
        size="icon-sm"
        aria-label={`Delete ${item.title}`}
        disabled={deleting}
        onClick={(e) => {
          e.stopPropagation();
          onDelete(item.id);
        }}
      >
        <Trash2 />
      </Button>
    </button>
  );
}

interface BacklogProps {
  items: ItineraryItem[];
  onItemClick: (item: ItineraryItem) => void;
  onDelete: (id: string) => void;
  deleting: boolean;
}

// The unscheduled-items list, laid out as a horizontally-wrapping row (it
// lives in a full-width card above the calendar). Clicking an item opens
// the schedule-edit modal - see ItinerarySection.
export function Backlog({
  items,
  onItemClick,
  onDelete,
  deleting,
}: BacklogProps) {
  return (
    <div className="flex min-h-16 flex-wrap gap-2 rounded-lg border border-dashed border-border p-2">
      {items.length === 0 && (
        <p className="p-2 text-sm text-muted-foreground">
          No unscheduled items yet.
        </p>
      )}
      {items.map((item) => (
        <BacklogItem
          key={item.id}
          item={item}
          onClick={onItemClick}
          onDelete={onDelete}
          deleting={deleting}
        />
      ))}
    </div>
  );
}
