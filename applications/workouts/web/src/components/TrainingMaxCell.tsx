import { useState, type FormEvent } from "react";
import { Check } from "lucide-react";

import { WeightUnit, type TrainingMax } from "@/gen/workouts/v1/training_max_pb";
import { useRecordTrainingMax } from "@/lib/trainingMaxes";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";

interface TrainingMaxCellProps {
  userId: string;
  exerciseId: string;
  current?: TrainingMax;
}

// An inline-editable weight+unit pair for one exercise's row - typing a
// value and hitting Save (or Enter) records it directly via
// useRecordTrainingMax, no dialog involved. Recording always inserts a new
// history row (see storage.TrainingMaxes.Record's doc comment) rather than
// overwriting, so re-saving the same value is harmless, just redundant.
export function TrainingMaxCell({ userId, exerciseId, current }: TrainingMaxCellProps) {
  const [weight, setWeight] = useState(current ? String(current.weight) : "");
  const [unit, setUnit] = useState<WeightUnit>(current?.unit ?? WeightUnit.KG);
  const recordTrainingMax = useRecordTrainingMax();

  const parsed = Number(weight);
  const valid = weight.trim() !== "" && !Number.isNaN(parsed) && parsed >= 0;

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!valid) return;
    recordTrainingMax.mutate({ userId, exerciseId, weight: parsed, unit });
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-1">
      <div className="flex items-center gap-1.5">
        <Input
          type="number"
          step="0.5"
          min="0"
          value={weight}
          onChange={(e) => setWeight(e.target.value)}
          placeholder="—"
          aria-label="Weight"
          className="h-7 w-20"
          disabled={recordTrainingMax.isPending}
        />
        {[WeightUnit.KG, WeightUnit.LB].map((option) => (
          <Button
            key={option}
            type="button"
            variant={unit === option ? "default" : "outline"}
            size="sm"
            aria-label={option === WeightUnit.KG ? "kg" : "lb"}
            aria-pressed={unit === option}
            className="px-2 text-xs"
            onClick={() => setUnit(option)}
          >
            {option === WeightUnit.KG ? "kg" : "lb"}
          </Button>
        ))}
        <Button type="submit" variant="ghost" size="icon-sm" aria-label="Save" disabled={!valid || recordTrainingMax.isPending}>
          <Check />
        </Button>
      </div>
      {recordTrainingMax.isError && (
        <span className="text-xs text-destructive">{recordTrainingMax.error.message}</span>
      )}
    </form>
  );
}
