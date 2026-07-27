import { formatColor } from "@/lib/format";

export function ColorSwatch({ color }: { color: string }) {
  if (!color) return <span className="text-muted-foreground">{formatColor(color)}</span>;
  return (
    <span className="inline-flex items-center gap-2">
      <span className="size-3 rounded-full border" style={{ backgroundColor: color }} />
      {color}
    </span>
  );
}
