import { useNavigate, useSearch } from "@tanstack/react-router";

// Who the navbar's user filter is currently showing - "Liam", "Tia", or
// both. Lives in the `users` URL search param (root route, see
// router.tsx) rather than localStorage: a bookmarked/shared link should
// reproduce the same filtered view, and it's fine for it to reset per-tab
// like any other query param. An empty/missing selection means "everyone".
export function useSelectedUserIds(): [string[], (ids: string[]) => void] {
  const { users } = useSearch({ from: "__root__" });
  const selectedUserIds = users ? users.split(",").filter(Boolean) : [];

  const navigate = useNavigate();
  const setSelectedUserIds = (ids: string[]) => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, users: ids.length > 0 ? ids.join(",") : undefined }),
      replace: true,
    });
  };

  return [selectedUserIds, setSelectedUserIds];
}
