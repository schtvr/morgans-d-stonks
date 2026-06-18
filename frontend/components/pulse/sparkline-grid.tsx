"use client";

import { type PositionRow } from "@/components/positions-table";
import { apiFetch } from "@/lib/api";
import { toCandleChartRows, type CandleChartRow } from "@/lib/portfolio-history";
import { useEffect, useState } from "react";
import { Area, AreaChart, ResponsiveContainer } from "recharts";

type SparkState = {
  rows: CandleChartRow[];
  loading: boolean;
  error: boolean;
};

function fmtPrice(v: number, currency: string) {
  return new Intl.NumberFormat(undefined, {
    style: "currency",
    currency: currency || "USD",
    minimumFractionDigits: v >= 1000 ? 0 : v >= 1 ? 2 : 4,
    maximumFractionDigits: v >= 1000 ? 0 : v >= 1 ? 2 : 4,
  }).format(v);
}

function fmtDayPL(v: number, currency: string) {
  const prefix = v >= 0 ? "+" : "";
  return (
    prefix +
    new Intl.NumberFormat(undefined, {
      style: "currency",
      currency: currency || "USD",
      maximumFractionDigits: 0,
    }).format(v)
  );
}

function SparklineCard({
  position,
  spark,
}: {
  position: PositionRow;
  spark: SparkState | undefined;
}) {
  const p = position;

  // Derive day % from candle data — dayPL from ingest snapshots is always 0 for Coinbase positions.
  const rows = spark?.rows ?? [];
  const firstClose = rows.length >= 2 ? rows[0].close : null;
  const lastClose = rows.length >= 2 ? rows[rows.length - 1].close : null;
  const hasCandleMetrics = firstClose !== null && lastClose !== null && firstClose !== 0;
  const dayPct = hasCandleMetrics ? ((lastClose! - firstClose!) / firstClose!) * 100 : 0;
  const estimatedDayPL = hasCandleMetrics ? (dayPct / 100) * p.marketValue : 0;

  const isUp = dayPct >= 0;
  const extreme = Math.abs(dayPct) >= 5;
  const chartColor = isUp ? "#22c55e" : "#ef4444";
  const gradientId = `sp-grad-${p.symbol.replace(/[^a-zA-Z0-9]/g, "")}`;

  const ticker = p.symbol.replace(/-USD$/, "").replace(/-USDC$/, "");

  const glowClass = extreme
    ? isUp
      ? "border-green-500/50 shadow-[0_0_16px_rgba(34,197,94,0.2)]"
      : "border-red-500/50 shadow-[0_0_16px_rgba(239,68,68,0.2)]"
    : "border-border";

  return (
    <div
      className={[
        "relative rounded-xl border bg-card p-3 transition-all duration-700",
        glowClass,
      ].join(" ")}
    >
      {/* Header row */}
      <div className="flex items-start justify-between gap-1">
        <span className="font-mono text-xs font-bold tracking-wide">{ticker}</span>
        {spark?.loading ? (
          <span className="h-3.5 w-10 animate-pulse rounded bg-muted" />
        ) : (
          <span
            className={[
              "flex items-center gap-0.5 text-xs font-semibold tabular-nums",
              isUp ? "text-green-500" : "text-red-500",
            ].join(" ")}
          >
            {isUp ? "▲" : "▼"} {Math.abs(dayPct).toFixed(2)}%
          </span>
        )}
      </div>

      {/* Price */}
      <p className="mt-0.5 font-mono text-sm font-semibold tabular-nums">
        {fmtPrice(p.lastPrice, p.currency)}
      </p>

      {/* Sparkline */}
      <div className="mt-2 h-10">
        {spark?.loading ? (
          <div className="h-full animate-pulse rounded bg-muted" />
        ) : rows.length > 1 ? (
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={rows} margin={{ top: 1, right: 0, bottom: 1, left: 0 }}>
              <defs>
                <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor={chartColor} stopOpacity={0.35} />
                  <stop offset="100%" stopColor={chartColor} stopOpacity={0} />
                </linearGradient>
              </defs>
              <Area
                type="monotone"
                dataKey="close"
                stroke={chartColor}
                strokeWidth={1.5}
                fill={`url(#${gradientId})`}
                dot={false}
                isAnimationActive={false}
              />
            </AreaChart>
          </ResponsiveContainer>
        ) : (
          <div className="flex h-full items-center justify-center text-[10px] text-muted-foreground">
            no data
          </div>
        )}
      </div>

      {/* Footer row */}
      <div className="mt-2 flex items-center justify-between gap-1">
        <span className="text-[10px] text-muted-foreground">Day P&L</span>
        {spark?.loading ? (
          <span className="h-2.5 w-12 animate-pulse rounded bg-muted" />
        ) : (
          <span
            className={[
              "text-[10px] font-semibold tabular-nums",
              isUp ? "text-green-500" : "text-red-500",
            ].join(" ")}
          >
            {fmtDayPL(estimatedDayPL, p.currency)}
          </span>
        )}
      </div>

      {/* Market value */}
      <div className="flex items-center justify-between gap-1">
        <span className="text-[10px] text-muted-foreground">Value</span>
        <span className="text-[10px] tabular-nums text-muted-foreground">
          {fmtPrice(p.marketValue, p.currency)}
        </span>
      </div>
    </div>
  );
}

type Props = {
  positions: PositionRow[];
  /** Called whenever candle-derived day % values are updated. Key = symbol, value = day % change. */
  onDayPcts?: (pcts: Record<string, number>) => void;
};

export function SparklineGrid({ positions, onDayPcts }: Props) {
  const [sparks, setSparks] = useState<Record<string, SparkState>>({});

  // Emit candle-derived day % to parent whenever sparks update
  useEffect(() => {
    if (!onDayPcts) return;
    const pcts: Record<string, number> = {};
    for (const [sym, s] of Object.entries(sparks)) {
      if (!s.loading && s.rows.length >= 2) {
        const first = s.rows[0].close;
        const last = s.rows[s.rows.length - 1].close;
        pcts[sym] = first !== 0 ? ((last - first) / first) * 100 : 0;
      }
    }
    if (Object.keys(pcts).length > 0) onDayPcts(pcts);
  }, [sparks, onDayPcts]);

  useEffect(() => {
    if (positions.length === 0) return;

    // Mark all as loading
    setSparks(
      Object.fromEntries(
        positions.map((p) => [p.symbol, { rows: [], loading: true, error: false }]),
      ),
    );

    const ctrl = new AbortController();

    void Promise.all(
      positions.map(async (p) => {
        try {
          const res = await apiFetch(
            `/api/market/candles?symbol=${encodeURIComponent(p.symbol)}&range=1d`,
            { signal: ctrl.signal },
          );
          if (!res.ok) {
            setSparks((prev) => ({
              ...prev,
              [p.symbol]: { rows: [], loading: false, error: true },
            }));
            return;
          }
          const json = (await res.json()) as { points?: unknown[] };
          const rows = toCandleChartRows((json.points ?? []) as Parameters<typeof toCandleChartRows>[0]);
          setSparks((prev) => ({
            ...prev,
            [p.symbol]: { rows, loading: false, error: false },
          }));
        } catch {
          if (!ctrl.signal.aborted) {
            setSparks((prev) => ({
              ...prev,
              [p.symbol]: { rows: [], loading: false, error: true },
            }));
          }
        }
      }),
    );

    return () => ctrl.abort();
  }, [positions]);

  if (positions.length === 0) {
    return (
      <div className="flex h-32 items-center justify-center text-sm text-muted-foreground">
        No positions to display
      </div>
    );
  }

  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
      {positions.map((p) => (
        <SparklineCard key={p.symbol} position={p} spark={sparks[p.symbol]} />
      ))}
    </div>
  );
}
