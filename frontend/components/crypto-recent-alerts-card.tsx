"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { apiFetch } from "@/lib/api";

type RecentAlert = {
  id: number;
  type?: string;
  symbol: string;
  source?: string;
  currentPrice: number;
  previousPrice?: number | null;
  deltaAmount?: number | null;
  deltaPct: number;
  thresholdPct: number;
  quantity?: number | null;
  avgCost?: number | null;
  costBasis?: number | null;
  unrealizedPl?: number | null;
  unrealizedPlPct?: number | null;
  firedAt: string;
  createdAt: string;
};

type RecentAlertsResponse = {
  alerts: RecentAlert[];
};

function fmtPct(value?: number | null) {
  if (value == null || Number.isNaN(value)) return "n/a";
  return `${value >= 0 ? "+" : ""}${value.toFixed(2)}%`;
}

function fmtMoney(value?: number | null) {
  if (value == null || Number.isNaN(value)) return "n/a";
  return value.toLocaleString(undefined, { style: "currency", currency: "USD" });
}

function fmtShortTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

export function CryptoRecentAlertsCard() {
  const [alerts, setAlerts] = useState<RecentAlert[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setError(null);
    try {
      const res = await apiFetch("/api/trading/recent-alerts?limit=8");
      if (!res.ok) throw new Error(await res.text());
      const json = (await res.json()) as RecentAlertsResponse;
      setAlerts(json.alerts ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load recent alerts");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
    const id = window.setInterval(() => void load(), 30_000);
    return () => window.clearInterval(id);
  }, [load]);

  const visibleAlerts = useMemo(() => [...alerts].sort((a, b) => b.firedAt.localeCompare(a.firedAt)), [alerts]);

  return (
    <Card className="relative overflow-hidden border-border/70">
      <div className="absolute inset-x-0 top-0 h-1 bg-gradient-to-r from-amber-400 via-orange-400 to-rose-500" />
      <CardHeader className="space-y-2">
        <CardTitle>Recent alerts</CardTitle>
        <CardDescription>
          The latest Coinbase moves that cleared your threshold. This stays intentionally compact for OpenClaw.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-center justify-between gap-3">
          <p className="text-xs text-muted-foreground">Updates every 30 seconds while the tab is open.</p>
          <Button type="button" variant="outline" size="sm" onClick={() => void load()} disabled={loading}>
            Refresh
          </Button>
        </div>

        {error ? <p className="text-sm text-destructive">{error}</p> : null}

        <div className="rounded-md border bg-muted/20">
          {loading ? (
            <div className="space-y-2 p-4">
              <div className="h-4 w-40 animate-pulse rounded bg-muted" />
              <div className="h-4 w-52 animate-pulse rounded bg-muted" />
              <div className="h-4 w-36 animate-pulse rounded bg-muted" />
            </div>
          ) : visibleAlerts.length === 0 ? (
            <div className="p-4">
              <p className="text-sm text-muted-foreground">No recent alerts yet.</p>
              <p className="mt-1 text-xs text-muted-foreground">Once a followed symbol moves enough, it will show up here.</p>
            </div>
          ) : (
            <ul className="divide-y">
              {visibleAlerts.map((alert) => (
                <li key={alert.id} className="space-y-2 p-4">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <p className="text-sm font-semibold">{alert.symbol}</p>
                      <p className="text-xs text-muted-foreground">{fmtShortTime(alert.firedAt)}</p>
                    </div>
                    <div className="text-right">
                      <p className="text-sm font-medium">{fmtPct(alert.deltaPct)}</p>
                      <p className="text-xs text-muted-foreground">Threshold {fmtPct(alert.thresholdPct)}</p>
                    </div>
                  </div>
                  <div className="grid gap-2 text-sm sm:grid-cols-2">
                    <p>Price: {fmtMoney(alert.currentPrice)}</p>
                    <p>Move: {fmtMoney(alert.deltaAmount)}</p>
                    <p>Prev: {fmtMoney(alert.previousPrice)}</p>
                    <p>Basis: {fmtMoney(alert.costBasis)}</p>
                    <p>P/L: {fmtMoney(alert.unrealizedPl)}</p>
                    <p>P/L %: {fmtPct(alert.unrealizedPlPct)}</p>
                  </div>
                  <p className="text-xs uppercase tracking-[0.16em] text-muted-foreground">
                    {alert.source || "manual"} {alert.quantity != null ? `• Qty ${alert.quantity}` : ""}
                  </p>
                </li>
              ))}
            </ul>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
