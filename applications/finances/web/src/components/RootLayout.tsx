import { Outlet } from "@tanstack/react-router";

import { useUsers } from "@/lib/users";
import { useSelectedUserIds } from "@/lib/selectedUsers";
import { Navbar } from "@/components/Navbar";

export function RootLayout() {
  const { data: users } = useUsers();
  const [selectedUserIds, setSelectedUserIds] = useSelectedUserIds();

  function toggleUser(id: string) {
    setSelectedUserIds(
      selectedUserIds.includes(id)
        ? selectedUserIds.filter((existing) => existing !== id)
        : [...selectedUserIds, id],
    );
  }

  return (
    <div className="flex min-h-screen flex-col">
      <Navbar users={users ?? []} selectedUserIds={selectedUserIds} onToggleUser={toggleUser} />
      <Outlet />
    </div>
  );
}
