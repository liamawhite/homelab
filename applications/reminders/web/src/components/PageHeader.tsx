import { CheckCircle } from "lucide-react";

export function PageHeader({ title, count }: { title: string; count: number }) {
  return (
    <div className="flex flex-col gap-1.5 px-1">
      <h1 className="text-3xl font-bold">{title}</h1>
      <div className="flex items-center gap-1.5 text-sm text-muted-foreground">
        <CheckCircle className="size-4" />
        <span>
          {count} {count === 1 ? "task" : "tasks"}
        </span>
      </div>
    </div>
  );
}
