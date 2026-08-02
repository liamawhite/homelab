import { ChevronDown, Settings, Users as UsersIcon } from "lucide-react";
import { Link } from "@tanstack/react-router";

import type { User } from "@/gen/finances/v1/user_pb";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";

interface UserFilterProps {
  users: User[];
  selectedUserIds: string[];
  onToggle: (id: string) => void;
}

// Multi-select, unlike workouts' single-active-user switcher: this filters
// which household member's data is shown (Liam, Tia, or both), not "who
// you're acting as". Nothing selected reads the same as everything
// selected - both mean "show everyone".
export function UserFilter({ users, selectedUserIds, onToggle }: UserFilterProps) {
  const selectedUsers = users.filter((user) => selectedUserIds.includes(user.id));
  const label =
    users.length === 0
      ? "No users"
      : selectedUsers.length === 0 || selectedUsers.length === users.length
        ? "Everyone"
        : selectedUsers.map((user) => user.name).join(", ");

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm" className="gap-1.5">
          <UsersIcon />
          {label}
          <ChevronDown className="text-muted-foreground" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuLabel>Show</DropdownMenuLabel>
        <DropdownMenuSeparator />
        {users.length === 0 && <DropdownMenuItem disabled>No users yet</DropdownMenuItem>}
        {users.map((user) => (
          <DropdownMenuCheckboxItem
            key={user.id}
            checked={selectedUserIds.includes(user.id)}
            onSelect={(e) => e.preventDefault()}
            onCheckedChange={() => onToggle(user.id)}
          >
            {user.name}
          </DropdownMenuCheckboxItem>
        ))}
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild>
          <Link to="/users">
            <Settings />
            Manage
          </Link>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
