"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { apiFetch } from "@/lib/api";

type FollowedSymbol = {
  symbol: string;
  source?: string;
  createdAt?: string;
  updatedAt?: string;
};

type FollowedSymbolsResponse = {
  symbols: FollowedSymbol[];
};

function normalizeSymbol(value: string) {
  return value.trim().toUpperCase();
}

function fmtSource(source?: string) {
  if (!source) return "manual";
  return source;
}

export function CryptoWatchlistCard() {
  const [symbols, setSymbols] = useState<FollowedSymbol[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [value, setValue] = useState("");

  const load = useCallback(async () => {
    setError(null);
    try {
      const res = await apiFetch("/api/trading/followed-symbols");
      if (!res.ok) throw new Error(await res.text());
      const json = (await res.json()) as FollowedSymbolsResponse;
      setSymbols(json.symbols ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load watchlist");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const sortedSymbols = useMemo(() => {
    return [...symbols].sort((a, b) => a.symbol.localeCompare(b.symbol));
  }, [symbols]);

  async function addSymbol() {
    const symbol = normalizeSymbol(value);
    if (!symbol) return;
    setBusy(true);
    setError(null);
    try {
      const res = await apiFetch("/api/trading/followed-symbols", {
        method: "POST",
        body: JSON.stringify({ symbol }),
      });
      if (!res.ok) throw new Error(await res.text());
      setValue("");
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to add symbol");
    } finally {
      setBusy(false);
    }
  }

  async function removeSymbol(symbol: string) {
    setBusy(true);
    setError(null);
    try {
      const res = await apiFetch(`/api/trading/followed-symbols/${encodeURIComponent(symbol)}`, {
        method: "DELETE",
      });
      if (!res.ok && res.status !== 204) throw new Error(await res.text());
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to remove symbol");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card>
      <CardHeader className="space-y-2">
        <CardTitle>Crypto watchlist</CardTitle>
        <CardDescription>
          Follow Coinbase symbols here. Alerts fire only when the price move clears your threshold.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex flex-col gap-3 sm:flex-row">
          <Input
            value={value}
            onChange={(e) => setValue(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                void addSymbol();
              }
            }}
            placeholder="BTC-USD, ETH-USDC"
            aria-label="Add followed symbol"
          />
          <Button type="button" onClick={() => void addSymbol()} disabled={busy || normalizeSymbol(value) === ""}>
            Follow
          </Button>
        </div>
        {error ? <p className="text-sm text-destructive">{error}</p> : null}
        <div className="rounded-md border bg-muted/20">
          {loading ? (
            <div className="space-y-2 p-4">
              <div className="h-4 w-32 animate-pulse rounded bg-muted" />
              <div className="h-4 w-44 animate-pulse rounded bg-muted" />
              <div className="h-4 w-24 animate-pulse rounded bg-muted" />
            </div>
          ) : sortedSymbols.length === 0 ? (
            <div className="p-4">
              <p className="text-sm text-muted-foreground">No followed crypto symbols yet.</p>
              <p className="mt-1 text-xs text-muted-foreground">The first seed comes from your Coinbase positions.</p>
            </div>
          ) : (
            <ul className="divide-y">
              {sortedSymbols.map((item) => (
                <li key={item.symbol} className="flex items-center justify-between gap-3 p-4">
                  <div>
                    <p className="font-medium">{item.symbol}</p>
                    <p className="text-xs text-muted-foreground">Source: {fmtSource(item.source)}</p>
                  </div>
                  <Button type="button" variant="outline" size="sm" onClick={() => void removeSymbol(item.symbol)} disabled={busy}>
                    Remove
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
