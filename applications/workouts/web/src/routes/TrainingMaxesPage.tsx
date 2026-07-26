import { useMemo } from "react";
import { Link } from "@tanstack/react-router";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  useReactTable,
} from "@tanstack/react-table";

import type { Exercise } from "@/gen/workouts/v1/exercise_pb";
import type { TrainingMax } from "@/gen/workouts/v1/training_max_pb";
import { useActiveUser } from "@/lib/activeUser";
import { useExercises } from "@/lib/exercises";
import { useCurrentTrainingMaxes } from "@/lib/trainingMaxes";
import { relativeTime } from "@/lib/time";
import { TrainingMaxCell } from "@/components/TrainingMaxCell";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

interface Row {
  exercise: Exercise;
  current?: TrainingMax;
}

const columnHelper = createColumnHelper<Row>();

export function TrainingMaxesPage() {
  const [activeUserId] = useActiveUser();
  const { data: exercises, isLoading: exercisesLoading } = useExercises();
  const {
    data: trainingMaxes,
    isLoading: maxesLoading,
    isError,
    error,
  } = useCurrentTrainingMaxes(activeUserId);
  // Both queries must have settled before rendering any row: exercises and
  // trainingMaxes resolve independently, and TrainingMaxCell only reads its
  // `current` prop once (via useState's initializer) to seed its inline
  // input - if it mounted before trainingMaxes resolved, it would latch
  // onto `current: undefined` and never pick up the real value once the
  // query finished, showing a blank input for an exercise that does have a
  // recorded max.
  const isLoading = exercisesLoading || maxesLoading;

  const activeExercises = useMemo(() => exercises?.filter((e) => !e.archived) ?? [], [exercises]);
  const currentByExerciseId = useMemo(
    () => new Map((trainingMaxes ?? []).map((max) => [max.exerciseId, max])),
    [trainingMaxes],
  );
  const rows = useMemo<Row[]>(
    () => activeExercises.map((exercise) => ({ exercise, current: currentByExerciseId.get(exercise.id) })),
    [activeExercises, currentByExerciseId],
  );

  const columns = useMemo(
    () => [
      columnHelper.accessor((row) => row.exercise.name, {
        id: "name",
        header: "Exercise",
      }),
      columnHelper.display({
        id: "current",
        header: "Current max",
        cell: (info) =>
          activeUserId ? (
            <TrainingMaxCell
              userId={activeUserId}
              exerciseId={info.row.original.exercise.id}
              current={info.row.original.current}
            />
          ) : null,
      }),
      columnHelper.display({
        id: "lastRecorded",
        header: "Last recorded",
        cell: (info) => {
          const effectiveAt = info.row.original.current?.effectiveAt;
          return (
            <span className="text-sm text-muted-foreground">
              {effectiveAt ? relativeTime(timestampDate(effectiveAt)) : "—"}
            </span>
          );
        },
      }),
    ],
    [activeUserId],
  );

  const table = useReactTable({
    data: rows,
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
          to track training maxes.
        </p>
      </div>
    );
  }

  return (
    <div className="mx-auto flex w-full max-w-4xl flex-1 flex-col gap-4 p-4">
      <h1 className="text-lg font-semibold">Training maxes</h1>

      {isLoading && <p className="text-sm text-muted-foreground">Loading…</p>}
      {isError && <p className="text-sm text-destructive">{error.message}</p>}

      {!isLoading && rows.length > 0 && (
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
      {rows.length === 0 && !isLoading && (
        <p className="text-sm text-muted-foreground">
          No exercises yet. Add one on the{" "}
          <Link to="/exercises" className="underline underline-offset-4">
            exercises
          </Link>{" "}
          page first.
        </p>
      )}
    </div>
  );
}
