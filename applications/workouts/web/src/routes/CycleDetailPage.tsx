import { useMemo, useState } from "react";
import { Link, useNavigate, useParams, useSearch } from "@tanstack/react-router";
import { Plus } from "lucide-react";

import { ExerciseCategory } from "@/gen/workouts/v1/exercise_pb";
import type { ExerciseSet } from "@/gen/workouts/v1/cycle_pb";
import { useActiveUser } from "@/lib/activeUser";
import { useCycle } from "@/lib/cycles";
import { useCurrentTrainingMaxes } from "@/lib/trainingMaxes";
import { AddBlockDialog } from "@/components/AddBlockDialog";
import { AddCycleExerciseDialog } from "@/components/AddCycleExerciseDialog";
import { CycleExerciseRow } from "@/components/CycleExerciseRow";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";

export function CycleDetailPage() {
  const { cycleId } = useParams({ from: "/cycles/$cycleId" });
  const { blockId } = useSearch({ from: "/cycles/$cycleId" });
  const navigate = useNavigate({ from: "/cycles/$cycleId" });

  const [activeUserId] = useActiveUser();
  const { data: cycle, isLoading, isError, error } = useCycle(cycleId);
  const { data: trainingMaxes } = useCurrentTrainingMaxes(activeUserId);
  const [addBlockOpen, setAddBlockOpen] = useState(false);
  const [addExerciseOpen, setAddExerciseOpen] = useState(false);

  const blocks = useMemo(
    () => (cycle?.blocks ?? []).slice().sort((a, b) => Number(a.position) - Number(b.position)),
    [cycle],
  );
  const selectedBlockId = blockId ?? blocks[0]?.id;

  const mainLifts = useMemo(
    () =>
      (cycle?.cycleExercises ?? [])
        .filter((ce) => ce.exerciseCategory === ExerciseCategory.MAIN_LIFT)
        .sort((a, b) => Number(a.position) - Number(b.position)),
    [cycle],
  );
  const accessories = useMemo(
    () =>
      (cycle?.cycleExercises ?? [])
        .filter((ce) => ce.exerciseCategory === ExerciseCategory.ACCESSORY)
        .sort((a, b) => Number(a.position) - Number(b.position)),
    [cycle],
  );
  const existingExerciseIds = useMemo(
    () => new Set((cycle?.cycleExercises ?? []).map((ce) => ce.exerciseId)),
    [cycle],
  );

  const setsByCell = useMemo(() => {
    const map = new Map<string, ExerciseSet[]>();
    for (const set of cycle?.exerciseSets ?? []) {
      const key = `${set.cycleExerciseId}:${set.blockId}`;
      const existing = map.get(key);
      if (existing) {
        existing.push(set);
      } else {
        map.set(key, [set]);
      }
    }
    for (const sets of map.values()) {
      sets.sort((a, b) => Number(a.position) - Number(b.position));
    }
    return map;
  }, [cycle]);

  const trainingMaxByExerciseId = useMemo(
    () => new Map((trainingMaxes ?? []).map((max) => [max.exerciseId, max])),
    [trainingMaxes],
  );

  function setSelectedBlock(next: string) {
    void navigate({ search: { blockId: next }, replace: true });
  }

  if (isError) {
    return (
      <div className="flex flex-1 items-center justify-center p-4">
        <p className="text-sm text-destructive">
          {error.message}{" "}
          <Link to="/cycles" className="underline underline-offset-4">
            Back to cycles
          </Link>
        </p>
      </div>
    );
  }

  if (isLoading || !cycle) {
    return (
      <div className="flex flex-1 items-center justify-center p-4">
        <p className="text-sm text-muted-foreground">Loading…</p>
      </div>
    );
  }

  return (
    <div className="mx-auto flex w-full max-w-4xl flex-1 flex-col gap-4 p-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">{cycle.name}</h1>
        <Button size="sm" className="gap-1" onClick={() => setAddExerciseOpen(true)}>
          <Plus />
          Add exercise
        </Button>
      </div>

      <div className="flex items-center gap-2">
        <Tabs value={selectedBlockId} onValueChange={setSelectedBlock} className="flex-1">
          <TabsList>
            {blocks.map((block) => (
              <TabsTrigger key={block.id} value={block.id}>
                {block.name}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
        <Button variant="outline" size="sm" className="gap-1" onClick={() => setAddBlockOpen(true)}>
          <Plus />
          New block
        </Button>
      </div>

      {blocks.length === 0 && (
        <p className="text-sm text-muted-foreground">Add a block to start prescribing sets.</p>
      )}

      {mainLifts.length > 0 && (
        <div className="rounded-lg border p-3">
          <Badge className="mb-2">Main lifts</Badge>
          {mainLifts.map((ce, index) => (
            <CycleExerciseRow
              key={ce.id}
              cycleExercise={ce}
              blockId={selectedBlockId}
              sets={selectedBlockId ? (setsByCell.get(`${ce.id}:${selectedBlockId}`) ?? []) : []}
              isFirstInGroup={index === 0}
              isLastInGroup={index === mainLifts.length - 1}
              currentTrainingMax={trainingMaxByExerciseId.get(ce.exerciseId)}
            />
          ))}
        </div>
      )}

      {accessories.length > 0 && (
        <div className="rounded-lg border p-3">
          <Badge variant="secondary" className="mb-2">
            Accessory
          </Badge>
          {accessories.map((ce, index) => (
            <CycleExerciseRow
              key={ce.id}
              cycleExercise={ce}
              blockId={selectedBlockId}
              sets={selectedBlockId ? (setsByCell.get(`${ce.id}:${selectedBlockId}`) ?? []) : []}
              isFirstInGroup={index === 0}
              isLastInGroup={index === accessories.length - 1}
              currentTrainingMax={trainingMaxByExerciseId.get(ce.exerciseId)}
            />
          ))}
        </div>
      )}

      {mainLifts.length === 0 && accessories.length === 0 && (
        <p className="text-sm text-muted-foreground">No exercises yet. Add one to get started.</p>
      )}

      <AddBlockDialog
        open={addBlockOpen}
        onOpenChange={setAddBlockOpen}
        cycleId={cycleId}
        onCreated={(newBlockId) => setSelectedBlock(newBlockId)}
      />
      <AddCycleExerciseDialog
        open={addExerciseOpen}
        onOpenChange={setAddExerciseOpen}
        cycleId={cycleId}
        existingExerciseIds={existingExerciseIds}
      />
    </div>
  );
}
