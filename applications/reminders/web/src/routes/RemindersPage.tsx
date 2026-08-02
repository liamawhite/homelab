import { useState } from "react";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { Trash2 } from "lucide-react";

import { useOneOffItems, useCreateOneOffItem, useDeleteOneOffItem } from "@/lib/oneOffItems";
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

export function RemindersPage() {
  const { data: items, isLoading, isError, error } = useOneOffItems();
  const createItem = useCreateOneOffItem();
  const deleteItem = useDeleteOneOffItem();
  const [title, setTitle] = useState("");

  const handleCreate = () => {
    const trimmed = title.trim();
    if (!trimmed) return;
    createItem.mutate(trimmed, { onSuccess: () => setTitle("") });
  };

  return (
    <div className="mx-auto flex w-full max-w-2xl flex-1 flex-col gap-4 p-4">
      <div className="flex gap-2">
        <Input
          placeholder="Add a reminder"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") handleCreate();
          }}
        />
        <Button onClick={handleCreate} disabled={createItem.isPending || !title.trim()}>
          Add
        </Button>
      </div>

      {isLoading && <p className="text-sm text-muted-foreground">Loading reminders…</p>}
      {isError && <p className="text-sm text-destructive">{error.message}</p>}

      {items && items.length > 0 && (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Title</TableHead>
              <TableHead>Created</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {items.map((item) => (
              <TableRow key={item.id}>
                <TableCell className="font-medium">{item.title}</TableCell>
                <TableCell className="text-muted-foreground">
                  {item.createdAt ? relativeTime(timestampDate(item.createdAt)) : null}
                </TableCell>
                <TableCell className="text-right">
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label={`Delete ${item.title}`}
                    disabled={deleteItem.isPending}
                    onClick={() => deleteItem.mutate(item.id)}
                  >
                    <Trash2 />
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
      {!isLoading && items && items.length === 0 && (
        <p className="text-sm text-muted-foreground">No reminders yet.</p>
      )}
      {createItem.isError && (
        <p className="text-sm text-destructive">{createItem.error.message}</p>
      )}
      {deleteItem.isError && (
        <p className="text-sm text-destructive">{deleteItem.error.message}</p>
      )}
    </div>
  );
}
