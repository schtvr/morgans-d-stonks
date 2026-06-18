"use client";

import type { AgentAction, AgentDecision, TriggerKind } from "@/lib/agent-api";
import { useRouter } from "next/navigation";
import { useCallback, useRef, useState } from "react";
import { createPortal } from "react-dom";

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

const RATIONALE_CLAMP_CHARS = 100;

function RationaleCell({ rationale }: { rationale: string }) {
  const anchorRef = useRef<HTMLSpanElement>(null);
  const [open, setOpen] = useState(false);
  const [position, setPosition] = useState<{ top: number; left: number } | null>(null);
  const truncated = rationale.length > RATIONALE_CLAMP_CHARS;
  const preview =
    truncated ? `${rationale.slice(0, RATIONALE_CLAMP_CHARS).trimEnd()}…` : rationale;

  const showPreview = useCallback(() => {
    if (!truncated || !anchorRef.current) return;
    const rect = anchorRef.current.getBoundingClientRect();
    const panelWidth = 384;
    const margin = 8;
    let left = rect.left;
    if (left + panelWidth > window.innerWidth - margin) {
      left = Math.max(margin, window.innerWidth - panelWidth - margin);
    }
    setPosition({ top: rect.bottom + margin, left });
    setOpen(true);
  }, [truncated]);

  const hidePreview = useCallback(() => setOpen(false), []);

  return (
    <>
      <span
        ref={anchorRef}
        className={`block max-w-[14rem] ${truncated ? "cursor-help underline decoration-dotted decoration-muted-foreground/50 underline-offset-2" : ""}`}
        onMouseEnter={showPreview}
        onMouseLeave={hidePreview}
        onClick={(e) => e.stopPropagation()}
        onFocus={showPreview}
        onBlur={hidePreview}
        tabIndex={truncated ? 0 : undefined}
      >
        {preview}
      </span>
      {open &&
        position &&
        typeof document !== "undefined" &&
        createPortal(
          <div
            role="dialog"
            aria-label="Full rationale"
            className="fixed z-50 max-h-64 w-96 overflow-y-auto rounded-lg border border-border bg-popover p-4 text-sm text-popover-foreground shadow-lg"
            style={{ top: position.top, left: position.left }}
            onMouseEnter={showPreview}
            onMouseLeave={hidePreview}
          >
            <p className="whitespace-pre-wrap leading-relaxed">{rationale}</p>
          </div>,
          document.body,
        )}
    </>
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
              <td className="max-w-[14rem] px-3 py-2.5 text-muted-foreground">
                <RationaleCell rationale={d.rationale} />
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
