import { useEffect } from "react";
import { Outlet } from "@tanstack/react-router";

import { useUsers } from "@/lib/users";
import { useActiveUser } from "@/lib/activeUser";
import { Navbar } from "@/components/Navbar";

export function RootLayout() {
  const { data: users } = useUsers();
  const [activeUserId, setActiveUserId] = useActiveUser();

  // Fall back to the first user whenever there's no valid active
  // selection - either nothing was ever picked, or the previously active
  // user no longer exists (e.g. deleted). Lives here (not a page
  // component) so it keeps working no matter which route is active.
  useEffect(() => {
    if (!users || users.length === 0) return;
    if (users.some((user) => user.id === activeUserId)) return;
    setActiveUserId(users[0].id);
  }, [users, activeUserId, setActiveUserId]);

  return (
    <div className="flex min-h-screen flex-col">
      <Navbar users={users ?? []} activeUserId={activeUserId} onSelectUser={setActiveUserId} />
      <Outlet />
    </div>
  );
}
