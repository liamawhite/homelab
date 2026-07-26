import { useMemo, useState, type FormEvent } from "react";
import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  useReactTable,
} from "@tanstack/react-table";
import { Trash2 } from "lucide-react";

import type { Item } from "@/gen/shopping/v1/item_pb";
import { useItems, useCreateItem, useDeleteItem } from "@/lib/items";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

const columnHelper = createColumnHelper<Item>();

export function ShoppingListPage() {
  const { data: items, isLoading, isError, error } = useItems();
  const [name, setName] = useState("");
  const createItem = useCreateItem();
  const deleteItem = useDeleteItem();

  const columns = useMemo(
    () => [
      columnHelper.accessor("name", {
        header: "Name",
      }),
      columnHelper.display({
        id: "actions",
        cell: (info) => (
          <div className="flex justify-end">
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={`Delete ${info.row.original.name}`}
              disabled={deleteItem.isPending}
              onClick={() => deleteItem.mutate(info.row.original.id)}
            >
              <Trash2 />
            </Button>
          </div>
        ),
      }),
    ],
    [deleteItem],
  );

  const table = useReactTable({
    data: items ?? [],
    columns,
    getCoreRowModel: getCoreRowModel(),
  });

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    createItem.mutate(name.trim(), { onSuccess: () => setName("") });
  }

  return (
    <div className="mx-auto flex min-h-screen w-full max-w-md flex-1 flex-col justify-center gap-4 p-4">
      <Card>
        <CardHeader>
          <CardTitle>Shopping list</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-4">
          {isLoading && <p className="text-sm text-muted-foreground">Loading items…</p>}
          {isError && <p className="text-sm text-destructive">{error.message}</p>}

          {items && items.length > 0 && (
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
          )}
          {items && items.length === 0 && (
            <p className="text-sm text-muted-foreground">No items yet.</p>
          )}
          {deleteItem.isError && (
            <p className="text-sm text-destructive">{deleteItem.error.message}</p>
          )}

          <form onSubmit={handleSubmit} className="grid gap-1.5">
            <Label htmlFor="new-item-name">New item</Label>
            <div className="flex gap-2">
              <Input
                id="new-item-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Name"
                disabled={createItem.isPending}
              />
              <Button type="submit" disabled={createItem.isPending || !name.trim()}>
                Add
              </Button>
            </div>
            {createItem.isError && (
              <p className="text-sm text-destructive">{createItem.error.message}</p>
            )}
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
