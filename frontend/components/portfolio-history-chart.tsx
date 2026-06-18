"use client";

import { Button } from "@/components/ui/button";
import { apiFetch } from "@/lib/api";
import {
  PORTFOLIO_HISTORY_RANGES,
  quoteCurrencyFromSymbol,
  toCandleChartRows,
  toPortfolioChartRows,
  type CandleChartRow,
  type MarketCandlesResponse,
  type PortfolioChartRow,
  type PortfolioHistoryPoint,
  type PortfolioHistoryRange,
  type PortfolioHistoryResponse,
} from "@/lib/portfolio-history";
import { useEffect, useId, useMemo, useState } from "react";
import { Area, CartesianGrid, ComposedChart, Legend, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

function fmtMoney(n: number, currency: string) {
  return new Intl.NumberFormat(undefined, { style: "currency", currency: currency || "USD", maximumFractionDigits: 0 }).format(n);
}

type Props = {
  /** When set, chart shows Coinbase candles for this product symbol (e.g. BTC-USD), not portfolio snapshots. */
  marketSymbol: string | null;
  onClearMarket: () => void;
};

export function PortfolioHistoryChart({ marketSymbol, onClearMarket }: Props) {
  const [range, setRange] = useState<PortfolioHistoryRange>("1w");
  const [portfolioPoints, setPortfolioPoints] = useState<PortfolioHistoryPoint[]>([]);
  const [candleRows, setCandleRows] = useState<CandleChartRow[]>([]);
  const [candleMeta, setCandleMeta] = useState<{ productId: string; granularity: string } | null>(null);
  const [currency, setCurrency] = useState("USD");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setError(null);
    setLoading(true);
    void (async () => {
      try {
        if (marketSymbol) {
          const res = await apiFetch(
            `/api/market/candles?symbol=${encodeURIComponent(marketSymbol)}&range=${encodeURIComponent(range)}`,
          );
          if (!res.ok) throw new Error(await res.text());
          const json = (await res.json()) as MarketCandlesResponse;
          if (cancelled) return;
          setCandleRows(toCandleChartRows(json.points ?? []));
          setCandleMeta({ productId: json.productId, granularity: json.granularity });
          setCurrency(quoteCurrencyFromSymbol(json.productId || marketSymbol));
          setPortfolioPoints([]);
        } else {
          const res = await apiFetch(`/api/portfolio/history?range=${encodeURIComponent(range)}`);
          if (!res.ok) throw new Error(await res.text());
          const json = (await res.json()) as PortfolioHistoryResponse;
          if (cancelled) return;
          setPortfolioPoints(json.points ?? []);
          setCandleRows([]);
          setCandleMeta(null);
          const cur = json.points?.find((p) => p.currency)?.currency;
          if (cur) setCurrency(cur);
        }
      } catch (e) {
        if (!cancelled) {
          setError(e instanceof Error ? e.message : "Failed to load chart");
          setPortfolioPoints([]);
          setCandleRows([]);
          setCandleMeta(null);
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [range, marketSymbol]);

  const portfolioChartData = useMemo(() => toPortfolioChartRows(portfolioPoints), [portfolioPoints]);
  const marketMode = marketSymbol !== null;
  const chartData: (PortfolioChartRow | CandleChartRow)[] = marketMode ? candleRows : portfolioChartData;
  const fillId = useId().replace(/:/g, "");
  const areaName = marketMode ? `Close (${candleMeta?.productId ?? marketSymbol})` : "Portfolio";
  const areaKey = marketMode ? "close" : "portfolio";

  return (
    <div className="space-y-4">
      {marketMode ? (
        <div className="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-border bg-muted/30 px-3 py-2 text-sm">
          <span className="text-muted-foreground">
            Coinbase candles — <span className="font-medium text-foreground">{marketSymbol}</span>
            {candleMeta?.granularity ? (
              <span className="ml-2 text-xs">({candleMeta.granularity.replaceAll("_", " ").toLowerCase()})</span>
            ) : null}
          </span>
          <Button type="button" variant="secondary" size="sm" onClick={onClearMarket}>
            Back to portfolio
          </Button>
        </div>
      ) : null}
      <div className="flex flex-wrap gap-2">
        {PORTFOLIO_HISTORY_RANGES.map(({ value, label }) => (
          <Button
            key={value}
            type="button"
            size="sm"
            variant={range === value ? "default" : "outline"}
            className="h-8 min-w-[2.5rem] px-2"
            onClick={() => setRange(value)}
          >
            {label}
          </Button>
        ))}
      </div>
      {loading ? (
        <div className="flex h-[280px] items-center justify-center rounded-lg border border-dashed text-sm text-muted-foreground">
          Loading…
        </div>
      ) : error ? (
        <div className="rounded-md border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive">{error}</div>
      ) : chartData.length === 0 ? (
        <div className="flex h-[280px] items-center justify-center rounded-lg border border-dashed text-sm text-muted-foreground">
          {marketMode ? "No candle data for this range." : "No snapshots in this range yet. Ingest will fill this chart over time."}
        </div>
      ) : (
        <div className="h-[300px] w-full min-w-0">
          <ResponsiveContainer width="100%" height="100%">
            <ComposedChart data={chartData} margin={{ left: 4, right: 12, top: 8, bottom: 4 }}>
              <defs>
                <linearGradient id={fillId} x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="var(--chart-1)" stopOpacity={0.45} />
                  <stop offset="100%" stopColor="var(--chart-1)" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" className="stroke-border" vertical={false} />
              <XAxis dataKey="label" tickLine={false} axisLine={false} fontSize={11} interval="preserveStartEnd" minTickGap={24} />
              <YAxis
                tickLine={false}
                axisLine={false}
                fontSize={11}
                tickFormatter={(v) => fmtMoney(Number(v), currency)}
                width={72}
              />
              <Tooltip
                contentStyle={{
                  background: "var(--card)",
                  border: "1px solid var(--border)",
                  borderRadius: "0.5rem",
                }}
                formatter={(value, name) => {
                  if (value == null) return ["—", String(name)];
                  const n = typeof value === "number" ? value : Number(value);
                  if (Number.isNaN(n)) return [String(value), String(name)];
                  return [fmtMoney(n, currency), String(name)];
                }}
                labelFormatter={(_, payload) => {
                  const row = payload?.[0]?.payload as { asOf?: string } | undefined;
                  return row?.asOf ? new Date(row.asOf).toLocaleString() : "";
                }}
              />
              <Legend />
              <Area
                type="monotone"
                dataKey={areaKey}
                name={areaName}
                stroke="var(--chart-1)"
                strokeWidth={2.5}
                fill={`url(#${fillId})`}
                dot={false}
                activeDot={{ r: 4 }}
              />
            </ComposedChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  );
}
