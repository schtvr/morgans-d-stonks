"use client";

import { Button } from "@/components/ui/button";
import { apiFetch } from "@/lib/api";
import {
  PORTFOLIO_HISTORY_RANGES,
  toChartRows,
  type PortfolioHistoryPoint,
  type PortfolioHistoryRange,
  type PortfolioHistoryResponse,
} from "@/lib/portfolio-history";
import { useCallback, useEffect, useMemo, useState } from "react";
import { CartesianGrid, Legend, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

function fmtMoney(n: number, currency: string) {
  return new Intl.NumberFormat(undefined, { style: "currency", currency: currency || "USD", maximumFractionDigits: 0 }).format(n);
}

type Props = {
  highlightSymbol: string | null;
};

export function PortfolioHistoryChart({ highlightSymbol }: Props) {
  const [range, setRange] = useState<PortfolioHistoryRange>("1w");
  const [points, setPoints] = useState<PortfolioHistoryPoint[]>([]);
  const [currency, setCurrency] = useState("USD");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async (r: PortfolioHistoryRange) => {
    setError(null);
    setLoading(true);
    try {
      const res = await apiFetch(`/api/portfolio/history?range=${encodeURIComponent(r)}`);
      if (!res.ok) throw new Error(await res.text());
      const json = (await res.json()) as PortfolioHistoryResponse;
      setPoints(json.points ?? []);
      const cur = json.points?.find((p) => p.currency)?.currency;
      if (cur) setCurrency(cur);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load history");
      setPoints([]);
    }
    setLoading(false);
  }, []);

  useEffect(() => {
    void load(range);
  }, [load, range]);

  const chartData = useMemo(() => toChartRows(points, highlightSymbol), [points, highlightSymbol]);

  const hasSymbolLine = highlightSymbol !== null && chartData.some((d) => d.symbolValue !== null);
  const dimPortfolio = Boolean(highlightSymbol && hasSymbolLine);

  return (
    <div className="space-y-4">
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
          Loading history…
        </div>
      ) : error ? (
        <div className="rounded-md border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive">{error}</div>
      ) : chartData.length === 0 ? (
        <div className="flex h-[280px] items-center justify-center rounded-lg border border-dashed text-sm text-muted-foreground">
          No snapshots in this range yet. Ingest will fill this chart over time.
        </div>
      ) : (
        <div className="h-[300px] w-full min-w-0">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={chartData} margin={{ left: 4, right: 12, top: 8, bottom: 4 }}>
              <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
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
                  background: "hsl(var(--card))",
                  border: "1px solid hsl(var(--border))",
                  borderRadius: "0.5rem",
                }}
                formatter={(value: number | string, name: string) => {
                  const n = typeof value === "number" ? value : Number(value);
                  if (Number.isNaN(n)) return [value, name];
                  return [fmtMoney(n, currency), name];
                }}
                labelFormatter={(_, payload) => {
                  const row = payload?.[0]?.payload as { asOf?: string } | undefined;
                  return row?.asOf ? new Date(row.asOf).toLocaleString() : "";
                }}
              />
              <Legend />
              <Line
                type="monotone"
                dataKey="portfolio"
                name="Portfolio"
                stroke="hsl(var(--chart-1))"
                strokeWidth={2}
                dot={false}
                strokeOpacity={dimPortfolio ? 0.25 : 1}
              />
              {hasSymbolLine ? (
                <Line
                  type="monotone"
                  dataKey="symbolValue"
                  name={highlightSymbol ?? "Symbol"}
                  stroke="hsl(var(--chart-2))"
                  strokeWidth={2}
                  dot={false}
                  connectNulls
                />
              ) : null}
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  );
}
