"use client";

import type { AgentCostResponse } from "@/lib/agent-api";

function fmtDollars(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`;
}

function fmtDay(day: string): string {
  const d = new Date(`${day}T00:00:00`);
  return d.toLocaleDateString(undefined, { month: "short", day: "numeric" });
}

export function CostMeter({ data }: { data: AgentCostResponse }) {
  const { today, capCents, points } = data;
  const pct = capCents > 0 ? Math.min(100, Math.round((today.costCents / capCents) * 100)) : 0;
  const barColor =
    pct >= 90 ? "bg-red-500" : pct >= 70 ? "bg-yellow-500" : "bg-green-500";

  return (
    <div className="space-y-4">
      <div>
        <div className="mb-1 flex items-baseline justify-between text-sm">
          <span className="font-medium">Today&apos;s spend</span>
          {capCents > 0 ? (
            <span className="text-muted-foreground">
              {fmtDollars(today.costCents)} / {fmtDollars(capCents)}
            </span>
          ) : (
            <span className="text-muted-foreground">{fmtDollars(today.costCents)}</span>
          )}
        </div>
        {capCents > 0 ? (
          <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
            <div
              className={`h-full rounded-full transition-all ${barColor}`}
              style={{ width: `${pct}%` }}
            />
          </div>
        ) : (
          <p className="text-xs text-muted-foreground">No cap configured</p>
        )}
        <p className="mt-1 text-xs text-muted-foreground">
          {today.decisions} decision{today.decisions !== 1 ? "s" : ""} today
        </p>
      </div>

      {points.length > 0 && (
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-border text-muted-foreground">
                <th className="pb-1.5 text-left font-medium">Day</th>
                <th className="pb-1.5 text-right font-medium">Decisions</th>
                <th className="pb-1.5 text-right font-medium">Cost</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {[...points].reverse().slice(0, 7).map((p) => (
                <tr key={p.day}>
                  <td className="py-1.5 text-muted-foreground">{fmtDay(p.day)}</td>
                  <td className="py-1.5 text-right tabular-nums">{p.decisions}</td>
                  <td className="py-1.5 text-right tabular-nums text-muted-foreground">
                    {fmtDollars(p.costCents)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
