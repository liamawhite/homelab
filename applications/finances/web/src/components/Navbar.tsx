import { Link } from "@tanstack/react-router";

import type { User } from "@/gen/finances/v1/user_pb";
import { ThemeToggle } from "@/components/ThemeToggle";
import { UserFilter } from "@/components/UserFilter";

interface NavbarProps {
  users: User[];
  selectedUserIds: string[];
  onToggleUser: (id: string) => void;
}

export function Navbar({ users, selectedUserIds, onToggleUser }: NavbarProps) {
  return (
    <header className="flex items-center justify-between border-b px-4 py-3">
      <Link to="/" className="text-sm font-semibold">
        Finances
      </Link>
      <div className="flex items-center gap-2">
        <UserFilter users={users} selectedUserIds={selectedUserIds} onToggle={onToggleUser} />
        <ThemeToggle />
      </div>
    </header>
  );
}
