import { useState } from "react";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useNavigate } from "@tanstack/react-router";
import { Trash2 } from "lucide-react";

import { useTrips, useCreateTrip, useDeleteTrip } from "@/lib/trips";
import { relativeTime } from "@/lib/time";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

export function TripsPage() {
  const { data: trips, isLoading, isError, error } = useTrips();
  const createTrip = useCreateTrip();
  const deleteTrip = useDeleteTrip();
  const [name, setName] = useState("");
  const navigate = useNavigate();

  const handleCreate = () => {
    const trimmed = name.trim();
    if (!trimmed) return;
    createTrip.mutate(trimmed, { onSuccess: () => setName("") });
  };

  return (
    <div className="mx-auto flex w-full max-w-2xl flex-1 flex-col gap-4 p-4">
      <div className="flex gap-2">
        <Input
          placeholder="Trip name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") handleCreate();
          }}
        />
        <Button onClick={handleCreate} disabled={createTrip.isPending || !name.trim()}>
          Add trip
        </Button>
      </div>

      {isLoading && <p className="text-sm text-muted-foreground">Loading trips…</p>}
      {isError && <p className="text-sm text-destructive">{error.message}</p>}

      {trips && trips.length > 0 && (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Created</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {trips.map((trip) => (
              <TableRow
                key={trip.id}
                className="cursor-pointer"
                onClick={() => navigate({ to: "/trips/$tripId", params: { tripId: trip.id } })}
              >
                <TableCell className="font-medium">{trip.name}</TableCell>
                <TableCell className="text-muted-foreground">
                  {trip.createdAt ? relativeTime(timestampDate(trip.createdAt)) : null}
                </TableCell>
                <TableCell className="text-right">
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label={`Delete ${trip.name}`}
                    disabled={deleteTrip.isPending}
                    onClick={(e) => {
                      e.stopPropagation();
                      deleteTrip.mutate(trip.id);
                    }}
                  >
                    <Trash2 />
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
      {!isLoading && trips && trips.length === 0 && (
        <p className="text-sm text-muted-foreground">No trips yet.</p>
      )}
      {createTrip.isError && (
        <p className="text-sm text-destructive">{createTrip.error.message}</p>
      )}
      {deleteTrip.isError && (
        <p className="text-sm text-destructive">{deleteTrip.error.message}</p>
      )}
    </div>
  );
}
