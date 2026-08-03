import { useState } from "react";
import { Check, Link2, Pencil, Plus, X } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

interface ConfirmationCellProps {
  confirmationLink: string;
  bookingNumber: string;
  isSaving: boolean;
  onSave: (confirmationLink: string, bookingNumber: string) => void;
}

// Inline-editable cell for a booking's confirmation link/number - shared by
// the flights and accommodations tables (both attach the same two optional
// fields to a booking, edited the same way).
export function ConfirmationCell({
  confirmationLink,
  bookingNumber,
  isSaving,
  onSave,
}: ConfirmationCellProps) {
  const [editing, setEditing] = useState(false);
  const [linkDraft, setLinkDraft] = useState(confirmationLink);
  const [numberDraft, setNumberDraft] = useState(bookingNumber);

  const startEditing = () => {
    setLinkDraft(confirmationLink);
    setNumberDraft(bookingNumber);
    setEditing(true);
  };

  const save = () => {
    onSave(linkDraft.trim(), numberDraft.trim());
    setEditing(false);
  };

  if (editing) {
    return (
      <div className="flex flex-col gap-1">
        <Input
          placeholder="Confirmation link"
          value={linkDraft}
          onChange={(e) => setLinkDraft(e.target.value)}
          className="h-7 text-xs"
        />
        <div className="flex gap-1">
          <Input
            placeholder="Booking #"
            value={numberDraft}
            onChange={(e) => setNumberDraft(e.target.value)}
            className="h-7 text-xs"
          />
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label="Save confirmation"
            disabled={isSaving}
            onClick={save}
          >
            <Check />
          </Button>
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label="Cancel"
            disabled={isSaving}
            onClick={() => setEditing(false)}
          >
            <X />
          </Button>
        </div>
      </div>
    );
  }

  if (!confirmationLink && !bookingNumber) {
    return (
      <Button
        variant="ghost"
        size="sm"
        onClick={startEditing}
        className="text-muted-foreground"
      >
        <Plus /> Add
      </Button>
    );
  }

  return (
    <div className="flex items-center gap-1 text-sm">
      <div className="flex flex-col">
        {confirmationLink && (
          <a
            href={confirmationLink}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-1 text-primary hover:underline"
          >
            <Link2 className="size-3.5" />
            Booking
          </a>
        )}
        {bookingNumber && (
          <span className="text-muted-foreground">{bookingNumber}</span>
        )}
      </div>
      <Button
        variant="ghost"
        size="icon-sm"
        aria-label="Edit confirmation"
        onClick={startEditing}
      >
        <Pencil />
      </Button>
    </div>
  );
}
