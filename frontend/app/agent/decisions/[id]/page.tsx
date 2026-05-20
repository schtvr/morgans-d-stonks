"use client";

import { DecisionDetail } from "@/components/agent/decision-detail";
import { SiteHeader } from "@/components/site-header";
import { fetchAgentDecision, type AgentDecision } from "@/lib/agent-api";
import Link from "next/link";
import { use, useEffect, useState } from "react";

export default function DecisionDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const [decision, setDecision] = useState<AgentDecision | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    void (async () => {
      try {
        const d = await fetchAgentDecision(Number(id));
        setDecision(d);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load decision");
      } finally {
        setLoading(false);
      }
    })();
  }, [id]);

  return (
    <div className="min-h-screen bg-background text-foreground">
      <SiteHeader />
      <main className="mx-auto max-w-4xl space-y-6 px-4 py-6">
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Link href="/agent" className="hover:text-foreground">
            ← The Agent
          </Link>
          <span>/</span>
          <span>Decision #{id}</span>
        </div>

        {loading ? (
          <div className="flex h-40 items-center justify-center text-sm text-muted-foreground">
            Loading…
          </div>
        ) : error ? (
          <p className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
            {error}
          </p>
        ) : decision ? (
          <>
            <div>
              <h1 className="text-2xl font-semibold tracking-tight">
                {decision.symbol} — {decision.action}
              </h1>
              <p className="text-sm text-muted-foreground">
                {new Date(decision.triggerAt).toLocaleString()} · {decision.triggerKind} trigger
              </p>
            </div>
            <DecisionDetail decision={decision} />
          </>
        ) : null}
      </main>
    </div>
  );
}
