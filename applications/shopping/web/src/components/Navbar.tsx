import { Tag } from "lucide-react";

import { ThemeToggle } from "@/components/ThemeToggle";
import { Button } from "@/components/ui/button";

interface NavbarProps {
  onManageLabels: () => void;
}

export function Navbar({ onManageLabels }: NavbarProps) {
  return (
    <header className="flex items-center justify-between border-b px-4 py-3">
      <span className="text-sm font-semibold">Shopping List</span>
      <div className="flex items-center gap-2">
        <Button variant="outline" size="sm" onClick={onManageLabels}>
          <Tag data-icon="inline-start" />
          Manage Labels
        </Button>
        <ThemeToggle />
      </div>
    </header>
  );
}
