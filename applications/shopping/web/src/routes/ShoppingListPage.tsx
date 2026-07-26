import { useEffect, useMemo, useState } from "react";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { createRootRoute } from "@tanstack/react-router";
import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  useReactTable,
} from "@tanstack/react-table";
import { Trash2 } from "lucide-react";

import { type Item, ItemStatus } from "@/gen/shopping/v1/item_pb";
import { useItems, useDeleteItem, useUpdateItemStatus } from "@/lib/items";
import { useLabels } from "@/lib/labels";
import { contrastTextColor } from "@/lib/color";
import { relativeTime } from "@/lib/time";
import { Navbar } from "@/components/Navbar";
import { ItemInput } from "@/components/ItemInput";
import { ManageLabelsDialog } from "@/components/ManageLabelsDialog";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import { cn } from "@/lib/utils";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

const ALL_TAB = "all";

const columnHelper = createColumnHelper<Item>();

export const Route = createRootRoute({
  validateSearch: (search: Record<string, unknown>): { label?: string } => ({
    label: typeof search.label === "string" && search.label !== ALL_TAB ? search.label : undefined,
  }),
  component: ShoppingListPage,
});

function ShoppingListPage() {
  const { data: items, isLoading, isError, error } = useItems();
  const { data: labels } = useLabels();
  const deleteItem = useDeleteItem();
  const updateStatus = useUpdateItemStatus();
  const { label } = Route.useSearch();
  const selectedTab = label ?? ALL_TAB;
  const navigate = Route.useNavigate();
  const setSelectedTab = (next: string) =>
    navigate({ search: { label: next === ALL_TAB ? undefined : next }, replace: true });
  const [manageOpen, setManageOpen] = useState(false);

  const activeLabels = useMemo(() => labels?.filter((l) => !l.archived) ?? [], [labels]);
  const labelColorById = useMemo(
    () => new Map((labels ?? []).map((l) => [l.id, l.color])),
    [labels],
  );

  useEffect(() => {
    if (labels && selectedTab !== ALL_TAB && !activeLabels.some((l) => l.id === selectedTab)) {
      setSelectedTab(ALL_TAB);
    }
  }, [labels, activeLabels, selectedTab]);

  const visibleItems = useMemo(
    () => (selectedTab === ALL_TAB ? (items ?? []) : (items ?? []).filter((i) => i.labelId === selectedTab)),
    [items, selectedTab],
  );

  const columns = useMemo(
    () => [
      columnHelper.display({
        id: "done",
        cell: (info) => {
          const done = info.row.original.status === ItemStatus.DONE;
          return (
            <Checkbox
              checked={done}
              aria-label={done ? `Mark ${info.row.original.name} as todo` : `Mark ${info.row.original.name} as done`}
              disabled={updateStatus.isPending}
              onCheckedChange={(checked) =>
                updateStatus.mutate({
                  id: info.row.original.id,
                  status: checked ? ItemStatus.DONE : ItemStatus.TODO,
                })
              }
            />
          );
        },
      }),
      columnHelper.accessor("name", {
        header: "Name",
        cell: (info) => {
          const done = info.row.original.status === ItemStatus.DONE;
          const color = labelColorById.get(info.row.original.labelId);
          return (
            <div className={cn(done && "text-muted-foreground line-through")}>
              <span>
                {info.getValue()}
                {selectedTab === ALL_TAB && (
                  <Badge
                    className="ml-2"
                    style={color ? { backgroundColor: color, color: contrastTextColor(color) } : undefined}
                  >
                    {info.row.original.labelName}
                  </Badge>
                )}
              </span>
              <div className="text-xs text-muted-foreground">
                {relativeTime(timestampDate(info.row.original.createdAt!))}
              </div>
            </div>
          );
        },
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
    [selectedTab, deleteItem, updateStatus, labelColorById],
  );

  const table = useReactTable({
    data: visibleItems,
    columns,
    getCoreRowModel: getCoreRowModel(),
  });

  return (
    <div className="flex min-h-screen flex-col">
      <Navbar onManageLabels={() => setManageOpen(true)} />
      <div className="mx-auto flex w-full max-w-md flex-1 flex-col justify-center gap-4 p-4">
        <Card>
          <CardContent className="grid gap-4">
            <ItemInput
              labels={activeLabels}
              onManageLabels={() => setManageOpen(true)}
              defaultLabelId={selectedTab === ALL_TAB ? undefined : selectedTab}
            />

            <Tabs value={selectedTab} onValueChange={setSelectedTab}>
              <TabsList>
                <TabsTrigger value={ALL_TAB}>All</TabsTrigger>
                {activeLabels.map((label) => (
                  <TabsTrigger key={label.id} value={label.id}>
                    <span
                      className="size-2 rounded-full"
                      style={{ backgroundColor: label.color }}
                      aria-hidden="true"
                    />
                    {label.name}
                  </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>

            {isLoading && <p className="text-sm text-muted-foreground">Loading items…</p>}
            {isError && <p className="text-sm text-destructive">{error.message}</p>}

            {visibleItems.length > 0 && (
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
            {!isLoading && visibleItems.length === 0 && (
              <p className="text-sm text-muted-foreground">No items yet.</p>
            )}
            {deleteItem.isError && (
              <p className="text-sm text-destructive">{deleteItem.error.message}</p>
            )}
            {updateStatus.isError && (
              <p className="text-sm text-destructive">{updateStatus.error.message}</p>
            )}
          </CardContent>
        </Card>
      </div>

      <ManageLabelsDialog open={manageOpen} onOpenChange={setManageOpen} />
    </div>
  );
}
