import { useState, type FormEvent } from "react";

import { ExerciseCategory, ExerciseEquipment } from "@/gen/workouts/v1/exercise_pb";
import { useCreateExercise } from "@/lib/exercises";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
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

interface AddExerciseDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function AddExerciseDialog({ open, onOpenChange }: AddExerciseDialogProps) {
  const [name, setName] = useState("");
  const [category, setCategory] = useState<ExerciseCategory>(ExerciseCategory.MAIN_LIFT);
  const [equipment, setEquipment] = useState<ExerciseEquipment>(ExerciseEquipment.BARBELL);
  const createExercise = useCreateExercise();

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    createExercise.mutate(
      { name: name.trim(), category, equipment },
      {
        onSuccess: () => {
          setName("");
          setCategory(ExerciseCategory.MAIN_LIFT);
          setEquipment(ExerciseEquipment.BARBELL);
          onOpenChange(false);
        },
      },
    );
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New exercise</DialogTitle>
          <DialogDescription>Added to the shared catalog everyone picks from.</DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="grid gap-4">
          <div className="grid gap-1.5">
            <Label htmlFor="new-exercise-name">Name</Label>
            <Input
              id="new-exercise-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Name"
              disabled={createExercise.isPending}
            />
          </div>

          <div className="grid gap-1.5">
            <Label>Category</Label>
            <div className="flex gap-1.5">
              {[ExerciseCategory.MAIN_LIFT, ExerciseCategory.ACCESSORY].map((option) => (
                <Button
                  key={option}
                  type="button"
                  variant="outline"
                  size="sm"
                  aria-expanded={category === option}
                  className={cn(category === option && "border-ring")}
                  onClick={() => setCategory(option)}
                >
                  {categoryLabel(option)}
                </Button>
              ))}
            </div>
          </div>

          <div className="grid gap-1.5">
            <Label>Equipment</Label>
            <div className="flex gap-1.5">
              {[ExerciseEquipment.BARBELL, ExerciseEquipment.DUMBBELL, ExerciseEquipment.BODYWEIGHT].map(
                (option) => (
                  <Button
                    key={option}
                    type="button"
                    variant="outline"
                    size="sm"
                    aria-expanded={equipment === option}
                    className={cn(equipment === option && "border-ring")}
                    onClick={() => setEquipment(option)}
                  >
                    {equipmentLabel(option)}
                  </Button>
                ),
              )}
            </div>
          </div>

          {createExercise.isError && (
            <p className="text-sm text-destructive">{createExercise.error.message}</p>
          )}

          <DialogFooter>
            <Button type="submit" disabled={createExercise.isPending || !name.trim()}>
              Add
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
