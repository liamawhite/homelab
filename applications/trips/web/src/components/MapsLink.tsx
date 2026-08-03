import { MapPin } from "lucide-react";

interface MapsLinkProps {
  // Precise mode - coordinates + place_id, used by itinerary items (whose
  // location is resolved via Google Places). Renders null if placeId is
  // empty (no location set).
  latitude?: number;
  longitude?: number;
  placeId?: string;
  // Fallback text-search mode, used by flights (which only have an IATA
  // airport code - no resolved coordinates). Ignored if placeId is set.
  query?: string;
  label?: string;
}

// Renders a link to Google Maps. In precise mode it uses the coordinates +
// place_id deep link (not a text search) so it points at the exact place
// the user picked; in query mode it falls back to a plain text search.
export function MapsLink({
  latitude,
  longitude,
  placeId,
  query,
  label = "Map",
}: MapsLinkProps) {
  let url: string;
  if (placeId) {
    url = `https://www.google.com/maps/search/?api=1&query=${latitude},${longitude}&query_place_id=${encodeURIComponent(placeId)}`;
  } else if (query) {
    url = `https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(query)}`;
  } else {
    return null;
  }

  return (
    <a
      href={url}
      target="_blank"
      rel="noopener noreferrer"
      className="flex items-center gap-1 text-primary hover:underline"
    >
      <MapPin className="size-3.5" />
      {label}
    </a>
  );
}
