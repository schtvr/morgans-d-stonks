"use client";

import type { AgentAction, AgentDecision, TriggerKind } from "@/lib/agent-api";
import { useRouter } from "next/navigation";

function relativeTime(iso: string): string {
  const diffMs = Date.now() - new Date(iso).getTime();
  const diffSec = Math.floor(diffMs / 1000);
  if (diffSec < 60) return `${diffSec}s ago`;
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffH = Math.floor(diffMin / 60);
  if (diffH < 24) return `${diffH}h ago`;
  return `${Math.floor(diffH / 24)}d ago`;
}

function ActionChip({ action }: { action: AgentAction }) {
  const cls =
    action === "buy"
      ? "bg-green-100 text-green-800"
      : action === "sell"
        ? "bg-red-100 text-red-800"
        : "bg-gray-100 text-gray-700";
  return (
    <span className={`inline-flex items-center rounded px-2 py-0.5 text-xs font-medium ${cls}`}>
      {action}
    </span>
  );
}

function TriggerChip({ kind }: { kind: TriggerKind }) {
  const cls =
    kind === "signal"
      ? "bg-blue-100 text-blue-800"
      : "bg-purple-100 text-purple-700";
  return (
    <span className={`inline-flex items-center rounded px-2 py-0.5 text-xs font-medium ${cls}`}>
      {kind}
    </span>
  );
}

function ConfidenceBar({ value }: { value: number }) {
  const pct = Math.round(value * 100);
  const color = pct >= 70 ? "bg-green-500" : pct >= 40 ? "bg-yellow-500" : "bg-red-400";
  return (
    <div className="flex items-center gap-1.5">
      <div className="h-1.5 w-16 overflow-hidden rounded-full bg-muted">
        <div className={`h-full rounded-full ${color}`} style={{ width: `${pct}%` }} />
      </div>
      <span className="text-xs tabular-nums text-muted-foreground">{pct}%</span>
    </div>
  );
}

export interface DecisionListProps {
  decisions: AgentDecision[];
}

export function DecisionList({ decisions }: DecisionListProps) {
  const router = useRouter();

  if (decisions.length === 0) {
    return (
      <div className="flex h-40 items-center justify-center rounded-lg border border-dashed text-sm text-muted-foreground">
        No decisions recorded yet.
      </div>
    );
  }

  return (
    <div className="overflow-x-auto rounded-lg border border-border">
      <table className="w-full text-sm">
        <thead className="bg-muted/40">
          <tr>
            <th className="px-3 py-2.5 text-left font-medium text-muted-foreground">Time</th>
            <th className="px-3 py-2.5 text-left font-medium text-muted-foreground">Trigger</th>
            <th className="px-3 py-2.5 text-left font-medium text-muted-foreground">Symbol</th>
            <th className="px-3 py-2.5 text-left font-medium text-muted-foreground">Action</th>
            <th className="px-3 py-2.5 text-left font-medium text-muted-foreground">Confidence</th>
            <th className="px-3 py-2.5 text-left font-medium text-muted-foreground">Rationale</th>
            <th className="px-3 py-2.5 text-left font-medium text-muted-foreground">Model</th>
            <th className="px-3 py-2.5 text-left font-medium text-muted-foreground">Cost</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-border">
          {decisions.map((d) => (
            <tr
              key={d.id}
              className="cursor-pointer transition-colors hover:bg-muted/30"
              onClick={() => router.push(`/agent/decisions/${d.id}`)}
            >
              <td className="whitespace-nowrap px-3 py-2.5 text-muted-foreground">
                {relativeTime(d.triggerAt)}
              </td>
              <td className="whitespace-nowrap px-3 py-2.5">
                <TriggerChip kind={d.triggerKind} />
              </td>
              <td className="whitespace-nowrap px-3 py-2.5 font-mono text-xs">{d.symbol}</td>
              <td className="whitespace-nowrap px-3 py-2.5">
                <ActionChip action={d.action} />
              </td>
              <td className="whitespace-nowrap px-3 py-2.5">
                <ConfidenceBar value={d.confidence} />
              </td>
              <td className="max-w-xs px-3 py-2.5 text-muted-foreground">
                <span title={d.rationale}>
                  {d.rationale.length > 120 ? `${d.rationale.slice(0, 120)}…` : d.rationale}
                </span>
              </td>
              <td className="whitespace-nowrap px-3 py-2.5 font-mono text-xs text-muted-foreground">
                {d.model.split("-").slice(0, 3).join("-")}
              </td>
              <td className="whitespace-nowrap px-3 py-2.5 text-muted-foreground">
                {d.costCents}¢
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
