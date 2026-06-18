"use client";

import type { AgentBenchmarkResponse } from "@/lib/agent-api";
import {
  CartesianGrid,
  ComposedChart,
  Legend,
  Line,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

function fmtDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, { month: "short", day: "numeric" });
}

function fmtPct(n: number): string {
  return `${n >= 0 ? "+" : ""}${n.toFixed(2)}%`;
}

export function BenchmarkChart({ data }: { data: AgentBenchmarkResponse }) {
  const positive = data.headlineExcessPct >= 0;

  if (data.points.length === 0) {
    return (
      <div className="flex h-[220px] items-center justify-center rounded-lg border border-dashed text-sm text-muted-foreground">
        No scored decisions yet — check back after the first 14-day window closes.
      </div>
    );
  }

  const chartData = data.points.map((p) => ({
    label: fmtDate(p.asOf),
    asOf: p.asOf,
    agent: p.realizedReturnPct,
    btc: p.btcReturnPct,
    excess: p.excessReturnPct,
  }));

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <span
          className={`text-xl font-semibold tabular-nums ${positive ? "text-green-600" : "text-red-600"}`}
        >
          {fmtPct(data.headlineExcessPct)}
        </span>
        <span className="text-sm text-muted-foreground">vs BTC ({data.window} rolling)</span>
      </div>

      <div className="h-[260px] w-full min-w-0">
        <ResponsiveContainer width="100%" height="100%">
          <ComposedChart data={chartData} margin={{ left: 4, right: 12, top: 8, bottom: 4 }}>
            <CartesianGrid strokeDasharray="3 3" className="stroke-border" vertical={false} />
            <XAxis
              dataKey="label"
              tickLine={false}
              axisLine={false}
              fontSize={11}
              interval="preserveStartEnd"
              minTickGap={24}
            />
            <YAxis
              tickLine={false}
              axisLine={false}
              fontSize={11}
              tickFormatter={(v) => `${Number(v).toFixed(1)}%`}
              width={52}
            />
            <Tooltip
              contentStyle={{
                background: "var(--card)",
                border: "1px solid var(--border)",
                borderRadius: "0.5rem",
              }}
              formatter={(value, name) => {
                const n = typeof value === "number" ? value : Number(value);
                return [fmtPct(n), String(name)];
              }}
              labelFormatter={(_, payload) => {
                const row = payload?.[0]?.payload as { asOf?: string } | undefined;
                return row?.asOf ? new Date(row.asOf).toLocaleDateString() : "";
              }}
            />
            <Legend />
            <ReferenceLine y={0} stroke="var(--border)" strokeDasharray="4 4" />
            <Line
              type="monotone"
              dataKey="agent"
              name="Agent"
              stroke="#3b82f6"
              strokeWidth={2.5}
              dot={false}
              activeDot={{ r: 4 }}
            />
            <Line
              type="monotone"
              dataKey="btc"
              name="BTC baseline"
              stroke="#f97316"
              strokeWidth={2}
              strokeDasharray="5 3"
              dot={false}
              activeDot={{ r: 4 }}
            />
          </ComposedChart>
        </ResponsiveContainer>
      </div>

      <div className="flex flex-wrap gap-2">
        {data.noteShadowFeesNotPaid && (
          <span className="rounded-full bg-yellow-100 px-2.5 py-0.5 text-xs text-yellow-800">
            Shadow mode — fees not modeled
          </span>
        )}
        {data.noteWindowIncomplete && (
          <span className="rounded-full bg-blue-100 px-2.5 py-0.5 text-xs text-blue-800">
            &lt; 14 days of data
          </span>
        )}
      </div>
    </div>
  );
}
