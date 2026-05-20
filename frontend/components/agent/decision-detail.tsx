"use client";

import type { AgentAction, AgentDecision } from "@/lib/agent-api";
import { useState } from "react";

function ActionChip({ action }: { action: AgentAction }) {
  const cls =
    action === "buy"
      ? "bg-green-100 text-green-800"
      : action === "sell"
        ? "bg-red-100 text-red-800"
        : "bg-gray-100 text-gray-700";
  return (
    <span className={`inline-flex items-center rounded px-2.5 py-1 text-sm font-semibold ${cls}`}>
      {action.toUpperCase()}
    </span>
  );
}

type Tab = "rationale" | "tools" | "raw";

export function DecisionDetail({ decision }: { decision: AgentDecision }) {
  const [activeTab, setActiveTab] = useState<Tab>("rationale");
  const [expandedTool, setExpandedTool] = useState<number | null>(null);
  const confidencePct = Math.round(decision.confidence * 100);

  const tabs: { id: Tab; label: string }[] = [
    { id: "rationale", label: "Rationale" },
    { id: "tools", label: `Tool calls${decision.toolCallsJson?.length ? ` (${decision.toolCallsJson.length})` : ""}` },
    { id: "raw", label: "Raw" },
  ];

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="rounded-lg border border-border bg-card p-4">
        <div className="flex flex-wrap items-center gap-3">
          <ActionChip action={decision.action} />
          <span className="text-sm text-muted-foreground">
            Confidence:{" "}
            <span className="font-medium text-foreground">{confidencePct}%</span>
          </span>
          <span className="text-sm text-muted-foreground">
            Model:{" "}
            <span className="font-mono text-xs text-foreground">{decision.model}</span>
          </span>
          <span className="text-sm text-muted-foreground">
            Prompt:{" "}
            <span className="font-mono text-xs text-foreground">{decision.promptVersion}</span>
          </span>
        </div>
        <div className="mt-3 flex flex-wrap gap-4 text-xs text-muted-foreground">
          <span>
            Latency: <span className="font-medium text-foreground">{decision.latencyMs}ms</span>
          </span>
          <span>
            Cost: <span className="font-medium text-foreground">{decision.costCents}¢</span>
          </span>
          <span>
            Symbol: <span className="font-mono font-medium text-foreground">{decision.symbol}</span>
          </span>
          <span>
            Trigger: <span className="font-medium text-foreground">{decision.triggerKind}</span>
          </span>
          {decision.sizingHintNotional != null && (
            <span>
              Sizing hint:{" "}
              <span className="font-medium text-foreground">
                ${decision.sizingHintNotional.toFixed(2)}
              </span>
            </span>
          )}
        </div>
      </div>

      {/* Tabs */}
      <div>
        <div className="flex border-b border-border">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              type="button"
              onClick={() => setActiveTab(tab.id)}
              className={`px-4 py-2 text-sm font-medium transition-colors ${
                activeTab === tab.id
                  ? "border-b-2 border-foreground text-foreground"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>

        <div className="pt-4">
          {activeTab === "rationale" && (
            <p className="whitespace-pre-wrap text-sm leading-relaxed text-foreground">
              {decision.rationale}
            </p>
          )}

          {activeTab === "tools" && (
            <>
              {!decision.toolCallsJson || decision.toolCallsJson.length === 0 ? (
                <p className="text-sm text-muted-foreground">No tool calls recorded.</p>
              ) : (
                <div className="space-y-2">
                  {decision.toolCallsJson.map((tc, i) => (
                    <div key={i} className="rounded-lg border border-border">
                      <button
                        type="button"
                        className="flex w-full items-center justify-between px-4 py-2.5 text-left text-sm"
                        onClick={() => setExpandedTool(expandedTool === i ? null : i)}
                      >
                        <span className="font-mono font-medium">{tc.name}</span>
                        <span className="text-xs text-muted-foreground">{tc.durationMs}ms</span>
                      </button>
                      {expandedTool === i && (
                        <div className="border-t border-border px-4 py-3 space-y-3">
                          <div>
                            <p className="mb-1 text-xs font-medium text-muted-foreground">Input</p>
                            <pre className="overflow-x-auto rounded bg-muted p-2 text-xs">
                              {JSON.stringify(tc.input, null, 2)}
                            </pre>
                          </div>
                          <div>
                            <p className="mb-1 text-xs font-medium text-muted-foreground">Output</p>
                            <pre className="overflow-x-auto rounded bg-muted p-2 text-xs">
                              {JSON.stringify(tc.output, null, 2)}
                            </pre>
                          </div>
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </>
          )}

          {activeTab === "raw" && (
            <details open={false}>
              <summary className="cursor-pointer text-sm text-muted-foreground hover:text-foreground">
                Show full decision JSON
              </summary>
              <pre className="mt-2 overflow-x-auto rounded bg-muted p-3 text-xs">
                {JSON.stringify(decision, null, 2)}
              </pre>
            </details>
          )}
        </div>
      </div>
    </div>
  );
}
