import { useMemo, useState, type FormEvent } from "react";
import { Link } from "@tanstack/react-router";
import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  useReactTable,
} from "@tanstack/react-table";
import { Trash2 } from "lucide-react";

import type { User } from "@/gen/finances/v1/user_pb";
import { useUsers, useCreateUser, useDeleteUser } from "@/lib/users";
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

const columnHelper = createColumnHelper<User>();

export function UsersPage() {
  const { data: users, isLoading, isError, error } = useUsers();
  const [name, setName] = useState("");
  const createUser = useCreateUser();
  const deleteUser = useDeleteUser();

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
              disabled={deleteUser.isPending}
              onClick={() => deleteUser.mutate(info.row.original.id)}
            >
              <Trash2 />
            </Button>
          </div>
        ),
      }),
    ],
    [deleteUser],
  );

  const table = useReactTable({
    data: users ?? [],
    columns,
    getCoreRowModel: getCoreRowModel(),
  });

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    createUser.mutate(name.trim(), { onSuccess: () => setName("") });
  }

  return (
    <div className="mx-auto flex w-full max-w-md flex-1 flex-col justify-center gap-4 p-4">
      <Card>
        <CardHeader>
          <CardTitle>Manage users</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-4">
          {isLoading && <p className="text-sm text-muted-foreground">Loading users…</p>}
          {isError && <p className="text-sm text-destructive">{error.message}</p>}

          {users && users.length > 0 && (
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
          {users && users.length === 0 && (
            <p className="text-sm text-muted-foreground">No users yet.</p>
          )}
          {deleteUser.isError && (
            <p className="text-sm text-destructive">{deleteUser.error.message}</p>
          )}

          <form onSubmit={handleSubmit} className="grid gap-1.5">
            <Label htmlFor="new-user-name">New user</Label>
            <div className="flex gap-2">
              <Input
                id="new-user-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Name"
                disabled={createUser.isPending}
              />
              <Button type="submit" disabled={createUser.isPending || !name.trim()}>
                Add
              </Button>
            </div>
            {createUser.isError && (
              <p className="text-sm text-destructive">{createUser.error.message}</p>
            )}
          </form>

          <Link to="/" className="text-sm text-muted-foreground underline underline-offset-4">
            Back
          </Link>
        </CardContent>
      </Card>
    </div>
  );
}
