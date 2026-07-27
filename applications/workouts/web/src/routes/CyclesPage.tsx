import { useMemo, useState } from "react";
import { Link, useNavigate } from "@tanstack/react-router";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  useReactTable,
} from "@tanstack/react-table";
import { Plus, Trash2 } from "lucide-react";

import type { Cycle } from "@/gen/workouts/v1/cycle_pb";
import { useActiveUser } from "@/lib/activeUser";
import { useCycles, useDeleteCycle } from "@/lib/cycles";
import { relativeTime } from "@/lib/time";
import { AddCycleDialog } from "@/components/AddCycleDialog";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

const columnHelper = createColumnHelper<Cycle>();

export function CyclesPage() {
  const [activeUserId] = useActiveUser();
  const { data: cycles, isLoading, isError, error } = useCycles(activeUserId);
  const [addOpen, setAddOpen] = useState(false);
  const deleteCycle = useDeleteCycle();
  const navigate = useNavigate();

  const columns = useMemo(
    () => [
      columnHelper.accessor("name", {
        header: "Name",
        cell: (info) => (
          <Link
            to="/cycles/$cycleId"
            params={{ cycleId: info.row.original.id }}
            className="underline underline-offset-4"
          >
            {info.getValue()}
          </Link>
        ),
      }),
      columnHelper.display({
        id: "createdAt",
        header: "Created",
        cell: (info) => (
          <span className="text-sm text-muted-foreground">
            {info.row.original.createdAt ? relativeTime(timestampDate(info.row.original.createdAt)) : "—"}
          </span>
        ),
      }),
      columnHelper.display({
        id: "actions",
        cell: (info) => (
          <div className="flex justify-end">
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={`Delete ${info.row.original.name}`}
              disabled={deleteCycle.isPending}
              onClick={() => deleteCycle.mutate(info.row.original.id)}
            >
              <Trash2 />
            </Button>
          </div>
        ),
      }),
    ],
    [deleteCycle],
  );

  const table = useReactTable({
    data: cycles ?? [],
    columns,
    getCoreRowModel: getCoreRowModel(),
  });

  if (!activeUserId) {
    return (
      <div className="flex flex-1 items-center justify-center p-4">
        <p className="text-sm text-muted-foreground">
          Select or create a{" "}
          <Link to="/users" className="underline underline-offset-4">
            user
          </Link>{" "}
          to build training cycles.
        </p>
      </div>
    );
  }

  return (
    <div className="mx-auto flex w-full max-w-4xl flex-1 flex-col gap-4 p-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">Cycles</h1>
        <Button size="sm" className="gap-1" onClick={() => setAddOpen(true)}>
          <Plus />
          New cycle
        </Button>
      </div>

      {isLoading && <p className="text-sm text-muted-foreground">Loading cycles…</p>}
      {isError && <p className="text-sm text-destructive">{error.message}</p>}
      {deleteCycle.isError && (
        <p className="text-sm text-destructive">{deleteCycle.error.message}</p>
      )}

      {cycles && cycles.length > 0 && (
        <div className="rounded-lg border">
          <Table>
            <TableHeader>
              {table.getHeaderGroups().map((headerGroup) => (
                <TableRow key={headerGroup.id}>
                  {headerGroup.headers.map((header) => (
                    <TableHead key={header.id}>
                      {header.isPlaceholder
                        ? null
                        : flexRender(header.column.columnDef.header, header.getContext())}
                    </TableHead>
                  ))}
                </TableRow>
              ))}
            </TableHeader>
            <TableBody>
              {table.getRowModel().rows.map((row) => (
                <TableRow key={row.id}>
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id}>
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  ))}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
      {cycles && cycles.length === 0 && (
        <p className="text-sm text-muted-foreground">No cycles yet.</p>
      )}

      <AddCycleDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        userId={activeUserId}
        onCreated={(cycleId) => navigate({ to: "/cycles/$cycleId", params: { cycleId } })}
      />
    </div>
  );
}
