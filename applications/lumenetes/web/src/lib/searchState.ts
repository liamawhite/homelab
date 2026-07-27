// Shared "what's expanded" persistence for routes with collapsible
// cards/rows (Dashboard's group cards, Switches' per-device bindings) - kept
// in the URL's ?open= query string (comma-separated keys) via TanStack
// Router's validateSearch, so refreshing or sharing the URL preserves
// whatever was toggled open. Mirrors the convention already established by
// applications/shopping's label-tab persistence (see ShoppingListPage.tsx).
export interface OpenSearch {
  open?: string;
}

export function validateOpenSearch(search: Record<string, unknown>): OpenSearch {
  return { open: typeof search.open === "string" ? search.open : undefined };
}

export function parseOpenKeys(open: string | undefined): Set<string> {
  return new Set(open ? open.split(",") : []);
}

// toggleOpenValue returns the next ?open= value after key's state changes to
// isOpen - undefined (not "") once nothing is open, keeping the URL clean.
export function toggleOpenValue(openKeys: Set<string>, key: string, isOpen: boolean): string | undefined {
  const next = new Set(openKeys);
  if (isOpen) {
    next.add(key);
  } else {
    next.delete(key);
  }
  return next.size > 0 ? Array.from(next).join(",") : undefined;
}
