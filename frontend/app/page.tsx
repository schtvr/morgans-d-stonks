"use client";

import { AccountSummaryBar, type Summary } from "@/components/account-summary";
import { CollapsibleSection } from "@/components/collapsible-section";
import { CryptoAlertControlsCard } from "@/components/crypto-alert-controls-card";
import { CryptoRecentAlertsCard } from "@/components/crypto-recent-alerts-card";
import { CryptoWatchlistCard } from "@/components/crypto-watchlist-card";
import { PortfolioHistoryChart } from "@/components/portfolio-history-chart";
import { PositionsTable, type PositionRow } from "@/components/positions-table";
import { SiteHeader } from "@/components/site-header";
import { apiFetch } from "@/lib/api";
import { useCallback, useEffect, useState } from "react";

export default function HomePage() {
  const [summary, setSummary] = useState<Summary | null>(null);
  const [positions, setPositions] = useState<PositionRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [marketChartSymbol, setMarketChartSymbol] = useState<string | null>(null);

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
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    }
    setLoading(false);
  }, []);

  useEffect(() => {
    void load(false);
    const id = window.setInterval(() => void load(true), 45_000);
    const onVis = () => {
      if (document.visibilityState === "visible") void load(true);
    };
    document.addEventListener("visibilitychange", onVis);
    return () => {
      window.clearInterval(id);
      document.removeEventListener("visibilitychange", onVis);
    };
  }, [load]);

  return (
    <div className="min-h-screen bg-background text-foreground">
      <SiteHeader />
      <main className="mx-auto max-w-6xl space-y-6 px-4 py-6">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Portfolio</h1>
          <p className="text-sm text-muted-foreground">Positions refresh every 45s while this tab is visible.</p>
        </div>
        <CollapsibleSection
          storageKey="portfolio-dashboard-collapse"
          sectionId="summary"
          title="Account summary"
          description="Net liquidation, cash, and buying power."
        >
          <AccountSummaryBar summary={summary} loading={loading} />
        </CollapsibleSection>
        <CollapsibleSection
          storageKey="portfolio-dashboard-collapse"
          sectionId="value-chart"
          title="Value over time"
          description="Portfolio from stored snapshots, or click a position to chart Coinbase candles for that product."
        >
          <PortfolioHistoryChart marketSymbol={marketChartSymbol} onClearMarket={() => setMarketChartSymbol(null)} />
        </CollapsibleSection>
        <CollapsibleSection storageKey="portfolio-dashboard-collapse" sectionId="positions" title="Positions">
          <PositionsTable
            positions={positions}
            loading={loading}
            error={error}
            onRetry={() => void load()}
            onSymbolClick={(sym) => setMarketChartSymbol(sym)}
          />
        </CollapsibleSection>
        <CollapsibleSection
          storageKey="portfolio-dashboard-collapse"
          sectionId="crypto-controls"
          title="Crypto controls"
          description="Watchlist and signal filter controls."
        >
          <div className="grid gap-6 xl:grid-cols-[1.35fr_0.9fr]">
            <CryptoWatchlistCard />
            <CryptoAlertControlsCard />
          </div>
        </CollapsibleSection>
        <CollapsibleSection
          storageKey="portfolio-dashboard-collapse"
          sectionId="recent-alerts"
          title="Recent alerts"
          description="Latest Coinbase moves that cleared your threshold."
        >
          <CryptoRecentAlertsCard />
        </CollapsibleSection>
      </main>
    </div>
  );
}
