"use client";

import { type Summary } from "@/components/account-summary";
import { CollapsibleSection } from "@/components/collapsible-section";
import { type PositionRow } from "@/components/positions-table";
import { PortfolioHeatmap } from "@/components/pulse/portfolio-heatmap";
import { SparklineGrid } from "@/components/pulse/sparkline-grid";
import { TopMovers } from "@/components/pulse/top-movers";
import { SiteHeader } from "@/components/site-header";
import { apiFetch } from "@/lib/api";
import { useCallback, useEffect, useState } from "react";

function fmtMoney(v: number, currency = "USD") {
  return new Intl.NumberFormat(undefined, {
    style: "currency",
    currency,
    maximumFractionDigits: 0,
  }).format(v);
}

export default function PulsePage() {
  const [positions, setPositions] = useState<PositionRow[]>([]);
  const [summary, setSummary] = useState<Summary | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
  const [secondsAgo, setSecondsAgo] = useState(0);
  // Candle-derived day % collected from SparklineGrid (dayPL from positions is always 0 for Coinbase).
  const [dayPctMap, setDayPctMap] = useState<Record<string, number>>({});

  const load = useCallback(async (silent = false) => {
    setError(null);
    if (!silent) setLoading(true);
    try {
      const [pRes, sRes] = await Promise.all([
        apiFetch("/api/portfolio/positions"),
        apiFetch("/api/portfolio/summary"),
      ]);
      if (!pRes.ok) throw new Error(await pRes.text());
      if (!sRes.ok) throw new Error(await sRes.text());
      const pJson = (await pRes.json()) as { positions: PositionRow[] };
      const sJson = (await sRes.json()) as Summary;
      setPositions(pJson.positions ?? []);
      setSummary(sJson.accountId ? sJson : null);
      setLastUpdated(new Date());
      setSecondsAgo(0);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    }
    setLoading(false);
  }, []);

  useEffect(() => {
    void load(false);
    const refreshId = window.setInterval(() => void load(true), 30_000);
    const onVis = () => {
      if (document.visibilityState === "visible") void load(true);
    };
    document.addEventListener("visibilitychange", onVis);
    return () => {
      window.clearInterval(refreshId);
      document.removeEventListener("visibilitychange", onVis);
    };
  }, [load]);

  // Live "Xs ago" ticker
  useEffect(() => {
    if (!lastUpdated) return;
    const id = window.setInterval(() => setSecondsAgo((s) => s + 1), 1000);
    return () => window.clearInterval(id);
  }, [lastUpdated]);

  const totalDayPL = positions.reduce((acc, p) => acc + p.dayPL, 0);
  const totalValue =
    summary?.netLiquidation ?? positions.reduce((acc, p) => acc + p.marketValue, 0);
  const currency = summary?.currency ?? "USD";

  const dayPLPositive = totalDayPL >= 0;

  return (
    <div className="min-h-screen bg-background text-foreground">
      <SiteHeader />
      <main className="mx-auto max-w-6xl space-y-6 px-4 py-6">
        {/* Page header */}
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-2xl font-semibold tracking-tight">Pulse</h1>
              <span
                className={[
                  "h-2 w-2 rounded-full transition-colors",
                  loading ? "animate-pulse bg-yellow-500" : "bg-green-500",
                ].join(" ")}
              />
            </div>
            <p className="text-sm text-muted-foreground">
              Live market heatmap · refreshes every 30s while tab is visible.
            </p>
          </div>

          {/* Summary pill */}
          <div className="flex flex-col items-end gap-0.5 rounded-xl border bg-card px-4 py-2.5">
            <div className="flex items-baseline gap-2">
              <span className="text-lg font-bold tabular-nums">
                {fmtMoney(totalValue, currency)}
              </span>
              <span
                className={[
                  "font-mono text-sm font-bold tabular-nums",
                  dayPLPositive ? "text-green-500" : "text-red-500",
                ].join(" ")}
              >
                {dayPLPositive ? "+" : ""}
                {fmtMoney(totalDayPL, currency)} today
              </span>
            </div>
            {lastUpdated && (
              <span className="text-xs text-muted-foreground">
                updated {secondsAgo}s ago
              </span>
            )}
          </div>
        </div>

        {error && (
          <div className="rounded-lg border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-500">
            {error}
          </div>
        )}

        {/* Heatmap */}
        <CollapsibleSection
          storageKey="pulse-collapse"
          sectionId="heatmap"
          title="Portfolio Heatmap"
          description="Cell size = market value · Color = today's % move (green = up, red = down)"
        >
          <PortfolioHeatmap positions={positions} dayPctMap={dayPctMap} />
        </CollapsibleSection>

        {/* Top Movers */}
        <CollapsibleSection
          storageKey="pulse-collapse"
          sectionId="movers"
          title="Today's Movers"
          description="Best and worst performing positions by day % change."
        >
          <TopMovers positions={positions} />
        </CollapsibleSection>

        {/* Sparkline Grid */}
        <CollapsibleSection
          storageKey="pulse-collapse"
          sectionId="sparklines"
          title="Live Sparklines"
          description="1-day candle chart per position pulled from Coinbase. Glowing border = move ≥ 5%."
        >
          <SparklineGrid positions={positions} onDayPcts={setDayPctMap} />
        </CollapsibleSection>
      </main>
    </div>
  );
}
