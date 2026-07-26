import { createContext, useCallback, useContext, useState, type ReactNode } from "react";

const STORAGE_KEY = "workouts.activeUserId";

// No auth/session concept exists yet, so "active user" is purely a
// client-side convenience for picking who you're acting as - not a security
// boundary. Persisted in localStorage so it survives a page reload.
//
// Backed by a single context (not a per-component localStorage-initialized
// useState) so every consumer - the navbar's dropdown, the users management
// page, RootLayout's auto-select fallback - shares one live value. Two
// independent useState instances would each snapshot localStorage once at
// mount and never see each other's updates.
const ActiveUserContext = createContext<{
  activeUserId: string | null;
  setActiveUserId: (id: string) => void;
} | null>(null);

export function ActiveUserProvider({ children }: { children: ReactNode }) {
  const [activeUserId, setActiveUserIdState] = useState<string | null>(() =>
    localStorage.getItem(STORAGE_KEY),
  );

  const setActiveUserId = useCallback((id: string) => {
    localStorage.setItem(STORAGE_KEY, id);
    setActiveUserIdState(id);
  }, []);

  return (
    <ActiveUserContext.Provider value={{ activeUserId, setActiveUserId }}>
      {children}
    </ActiveUserContext.Provider>
  );
}

export function useActiveUser(): [string | null, (id: string) => void] {
  const ctx = useContext(ActiveUserContext);
  if (!ctx) {
    throw new Error("useActiveUser must be used within an ActiveUserProvider");
  }
  return [ctx.activeUserId, ctx.setActiveUserId];
}
