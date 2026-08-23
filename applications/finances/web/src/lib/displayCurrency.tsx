import { createContext, useContext, useEffect, useState, type ReactNode } from "react";

import { Currency } from "@/gen/finances/v1/asset_pb";

type DisplayCurrency = Currency.USD | Currency.GBP;

const STORAGE_KEY = "finances.displayCurrency";

const DisplayCurrencyContext = createContext<{
  displayCurrency: DisplayCurrency;
  toggleDisplayCurrency: () => void;
} | null>(null);

function getInitialDisplayCurrency(): DisplayCurrency {
  const stored = localStorage.getItem(STORAGE_KEY);
  return stored === "gbp" ? Currency.GBP : Currency.USD;
}

// Which currency money is *shown* in - separate from an asset's stored
// `currency` field, which never changes. Everything converts to this on
// display via useExchangeRates (see @/lib/exchangeRates).
export function DisplayCurrencyProvider({ children }: { children: ReactNode }) {
  const [displayCurrency, setDisplayCurrency] = useState<DisplayCurrency>(getInitialDisplayCurrency);

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, displayCurrency === Currency.GBP ? "gbp" : "usd");
  }, [displayCurrency]);

  function toggleDisplayCurrency() {
    setDisplayCurrency((current) => (current === Currency.USD ? Currency.GBP : Currency.USD));
  }

  return (
    <DisplayCurrencyContext.Provider value={{ displayCurrency, toggleDisplayCurrency }}>
      {children}
    </DisplayCurrencyContext.Provider>
  );
}

export function useDisplayCurrency() {
  const ctx = useContext(DisplayCurrencyContext);
  if (!ctx) {
    throw new Error("useDisplayCurrency must be used within a DisplayCurrencyProvider");
  }
  return ctx;
}
