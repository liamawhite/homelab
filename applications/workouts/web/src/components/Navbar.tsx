import type { User } from "@/gen/workouts/v1/user_pb";
import { ThemeToggle } from "@/components/ThemeToggle";
import { UserMenu } from "@/components/UserMenu";

interface NavbarProps {
  users: User[];
  activeUserId: string | null;
  onSelectUser: (id: string) => void;
}

export function Navbar({ users, activeUserId, onSelectUser }: NavbarProps) {
  return (
    <header className="flex items-center justify-between border-b px-4 py-3">
      <span className="text-sm font-semibold">Workouts</span>
      <div className="flex items-center gap-2">
        <UserMenu users={users} activeUserId={activeUserId} onSelect={onSelectUser} />
        <ThemeToggle />
      </div>
    </header>
  );
}
