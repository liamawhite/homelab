import { useEffect, useRef, useState } from "react";
import { Code, ConnectError } from "@connectrpc/connect";
import { X } from "lucide-react";

import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { searchLocations } from "@/lib/itinerary";
import type { LocationCandidate } from "@/gen/trips/v1/itinerary_pb";

const DEBOUNCE_MS = 300;

interface LocationSearchInputProps {
  selected: LocationCandidate | null;
  onSelect: (candidate: LocationCandidate | null) => void;
}

// Type-ahead location search backed by the ItineraryService.SearchLocations
// RPC (which proxies to Google Places server-side - see
// internal/itineraryservice). Debounces queries, shows a dropdown of
// candidates, and degrades to a disabled input if the backend reports
// location search isn't configured (no API key), rather than erroring on
// every keystroke.
export function LocationSearchInput({
  selected,
  onSelect,
}: LocationSearchInputProps) {
  const [query, setQuery] = useState("");
  const [candidates, setCandidates] = useState<LocationCandidate[]>([]);
  const [unavailable, setUnavailable] = useState(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);

    if (!query.trim() || unavailable) {
      setCandidates([]);
      return;
    }

    debounceRef.current = setTimeout(() => {
      searchLocations(query.trim())
        .then(setCandidates)
        .catch((err: unknown) => {
          if (
            err instanceof ConnectError &&
            err.code === Code.FailedPrecondition
          ) {
            setUnavailable(true);
            return;
          }
          setCandidates([]);
        });
    }, DEBOUNCE_MS);

    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [query, unavailable]);

  if (unavailable) {
    return (
      <Input placeholder="Location search unavailable" disabled readOnly />
    );
  }

  if (selected) {
    return (
      <div className="flex items-center gap-1">
        <Input value={selected.description} disabled readOnly />
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label="Change location"
          onClick={() => {
            onSelect(null);
            setQuery("");
          }}
        >
          <X />
        </Button>
      </div>
    );
  }

  return (
    <div className="relative">
      <Input
        placeholder="Location (optional)"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
      />
      {candidates.length > 0 && (
        <ul className="absolute z-10 mt-1 w-full rounded-lg border border-border bg-popover shadow-md">
          {candidates.map((candidate) => (
            <li key={candidate.placeId}>
              <button
                type="button"
                className="w-full px-2.5 py-1.5 text-left text-sm hover:bg-muted"
                onClick={() => {
                  onSelect(candidate);
                  setCandidates([]);
                }}
              >
                {candidate.description}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
