import { ChevronDown, Dumbbell, Settings, User as UserIcon } from "lucide-react";
import { Link } from "@tanstack/react-router";

import type { User } from "@/gen/workouts/v1/user_pb";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";

interface UserMenuProps {
  users: User[];
  activeUserId: string | null;
  onSelect: (id: string) => void;
}

export function UserMenu({ users, activeUserId, onSelect }: UserMenuProps) {
  const activeUser = users.find((user) => user.id === activeUserId);

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm" className="gap-1.5">
          <UserIcon />
          {activeUser ? activeUser.name : users.length === 0 ? "No users" : "Select user"}
          <ChevronDown className="text-muted-foreground" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuLabel>Switch user</DropdownMenuLabel>
        <DropdownMenuSeparator />
        {users.length === 0 && <DropdownMenuItem disabled>No users yet</DropdownMenuItem>}
        {users.map((user) => (
          <DropdownMenuItem key={user.id} onSelect={() => onSelect(user.id)}>
            {user.name}
          </DropdownMenuItem>
        ))}
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild>
          <Link to="/exercises">
            <Dumbbell />
            Exercises
          </Link>
        </DropdownMenuItem>
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
