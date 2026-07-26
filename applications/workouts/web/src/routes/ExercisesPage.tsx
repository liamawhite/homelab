import { useMemo, useState } from "react";
import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  useReactTable,
} from "@tanstack/react-table";
import { Archive as ArchiveIcon, ArchiveRestore, Plus } from "lucide-react";

import { ExerciseCategory, ExerciseEquipment, type Exercise } from "@/gen/workouts/v1/exercise_pb";
import { useExercises, useArchiveExercise, useRestoreExercise } from "@/lib/exercises";
import { AddExerciseDialog } from "@/components/AddExerciseDialog";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { cn } from "@/lib/utils";

function categoryLabel(category: ExerciseCategory): string {
  return category === ExerciseCategory.MAIN_LIFT ? "Main lift" : "Accessory";
}

function equipmentLabel(equipment: ExerciseEquipment): string {
  switch (equipment) {
    case ExerciseEquipment.BARBELL:
      return "Barbell";
    case ExerciseEquipment.DUMBBELL:
      return "Dumbbell";
    case ExerciseEquipment.BODYWEIGHT:
      return "Bodyweight";
    default:
      return "Unknown";
  }
}

const columnHelper = createColumnHelper<Exercise>();

export function ExercisesPage() {
  const { data: exercises, isLoading, isError, error } = useExercises();
  const [addOpen, setAddOpen] = useState(false);
  const archiveExercise = useArchiveExercise();
  const restoreExercise = useRestoreExercise();

  const columns = useMemo(
    () => [
      columnHelper.accessor("name", {
        header: "Name",
        cell: (info) => (
          <span className={cn(info.row.original.archived && "text-muted-foreground line-through")}>
            {info.getValue()}
          </span>
        ),
      }),
      columnHelper.accessor("category", {
        header: "Category",
        cell: (info) => (
          <Badge variant={info.getValue() === ExerciseCategory.MAIN_LIFT ? "default" : "secondary"}>
            {categoryLabel(info.getValue())}
          </Badge>
        ),
      }),
      columnHelper.accessor("equipment", {
        header: "Equipment",
        cell: (info) => <Badge variant="outline">{equipmentLabel(info.getValue())}</Badge>,
      }),
      columnHelper.accessor("archived", {
        header: "Status",
        cell: (info) => (
          <Badge variant={info.getValue() ? "outline" : "secondary"}>
            {info.getValue() ? "Archived" : "Active"}
          </Badge>
        ),
      }),
      columnHelper.display({
        id: "actions",
        cell: (info) => (
          <div className="flex justify-end">
            {info.row.original.archived ? (
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={`Restore ${info.row.original.name}`}
                disabled={restoreExercise.isPending}
                onClick={() => restoreExercise.mutate(info.row.original.id)}
              >
                <ArchiveRestore />
              </Button>
            ) : (
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={`Archive ${info.row.original.name}`}
                disabled={archiveExercise.isPending}
                onClick={() => archiveExercise.mutate(info.row.original.id)}
              >
                <ArchiveIcon />
              </Button>
            )}
          </div>
        ),
      }),
    ],
    [archiveExercise, restoreExercise],
  );

  const table = useReactTable({
    data: exercises ?? [],
    columns,
    getCoreRowModel: getCoreRowModel(),
  });

  return (
    <div className="mx-auto flex w-full max-w-4xl flex-1 flex-col gap-4 p-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">Exercises</h1>
        <Button size="sm" className="gap-1" onClick={() => setAddOpen(true)}>
          <Plus />
          New exercise
        </Button>
      </div>

      {isLoading && <p className="text-sm text-muted-foreground">Loading exercises…</p>}
      {isError && <p className="text-sm text-destructive">{error.message}</p>}
      {archiveExercise.isError && (
        <p className="text-sm text-destructive">{archiveExercise.error.message}</p>
      )}
      {restoreExercise.isError && (
        <p className="text-sm text-destructive">{restoreExercise.error.message}</p>
      )}

      {exercises && exercises.length > 0 && (
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
      {exercises && exercises.length === 0 && (
        <p className="text-sm text-muted-foreground">No exercises yet.</p>
      )}

      <AddExerciseDialog open={addOpen} onOpenChange={setAddOpen} />
    </div>
  );
}
