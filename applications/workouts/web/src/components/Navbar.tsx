import { Link } from "@tanstack/react-router";
import { CalendarRange, Dumbbell, TrendingUp } from "lucide-react";

import type { User } from "@/gen/workouts/v1/user_pb";
import { ThemeToggle } from "@/components/ThemeToggle";
import { UserMenu } from "@/components/UserMenu";
import { Button } from "@/components/ui/button";

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
        <Button variant="ghost" size="icon" aria-label="Cycles" asChild>
          <Link to="/cycles">
            <CalendarRange />
          </Link>
        </Button>
        <Button variant="ghost" size="icon" aria-label="Training maxes" asChild>
          <Link to="/training-maxes">
            <TrendingUp />
          </Link>
        </Button>
        <Button variant="ghost" size="icon" aria-label="Exercises" asChild>
          <Link to="/exercises">
            <Dumbbell />
          </Link>
        </Button>
        <UserMenu users={users} activeUserId={activeUserId} onSelect={onSelectUser} />
        <ThemeToggle />
      </div>
    </header>
  );
}
