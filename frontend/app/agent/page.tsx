"use client";

import { BenchmarkChart } from "@/components/agent/benchmark-chart";
import { CostMeter } from "@/components/agent/cost-meter";
import { DecisionList } from "@/components/agent/decision-list";
import { SiteHeader } from "@/components/site-header";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  fetchAgentBenchmark,
  fetchAgentCost,
  fetchAgentDecisions,
  type AgentBenchmarkResponse,
  type AgentCostResponse,
  type AgentDecision,
} from "@/lib/agent-api";
import { useCallback, useEffect, useState } from "react";

export default function AgentPage() {
  const [decisions, setDecisions] = useState<AgentDecision[]>([]);
  const [benchmark, setBenchmark] = useState<AgentBenchmarkResponse | null>(null);
  const [cost, setCost] = useState<AgentCostResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async (silent = false) => {
    if (!silent) setLoading(true);
    setError(null);
    try {
      const [decisionsSettled, benchmarkSettled, costSettled] = await Promise.allSettled([
        fetchAgentDecisions({ limit: 50 }),
        fetchAgentBenchmark("14d"),
        fetchAgentCost("7d"),
      ]);
      const errors: string[] = [];
      if (decisionsSettled.status === "fulfilled") {
        setDecisions(decisionsSettled.value.decisions ?? []);
      } else {
        errors.push("decisions");
      }
      if (benchmarkSettled.status === "fulfilled") {
        setBenchmark(benchmarkSettled.value);
      } else {
        errors.push("benchmark");
      }
      if (costSettled.status === "fulfilled") {
        setCost(costSettled.value);
      } else {
        errors.push("cost");
      }
      if (errors.length > 0) {
        const detail =
          decisionsSettled.status === "rejected"
            ? decisionsSettled.reason
            : benchmarkSettled.status === "rejected"
              ? benchmarkSettled.reason
              : costSettled.status === "rejected"
                ? costSettled.reason
                : null;
        const msg =
          detail instanceof Error && detail.message
            ? detail.message
            : `Failed to load: ${errors.join(", ")}`;
        setError(msg);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load agent data");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load(false);
    const id = window.setInterval(() => void load(true), 60_000);
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
          <h1 className="text-2xl font-semibold tracking-tight">The Agent</h1>
          <p className="text-sm text-muted-foreground">
            Shadow decision history, benchmark performance, and cost tracking.
          </p>
        </div>

        {error ? (
          <p className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
            {error}
          </p>
        ) : null}

        {/* Top row: benchmark + cost */}
        <div className="grid gap-6 xl:grid-cols-[1fr_320px]">
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-base font-medium">Benchmark vs BTC</CardTitle>
            </CardHeader>
            <CardContent>
              {loading ? (
                <div className="flex h-[260px] items-center justify-center text-sm text-muted-foreground">
                  Loading…
                </div>
              ) : benchmark ? (
                <BenchmarkChart data={benchmark} />
              ) : (
                <p className="text-sm text-muted-foreground">No benchmark data.</p>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-base font-medium">Cost tracker</CardTitle>
            </CardHeader>
            <CardContent>
              {loading ? (
                <p className="text-sm text-muted-foreground">Loading…</p>
              ) : cost ? (
                <CostMeter data={cost} />
              ) : (
                <p className="text-sm text-muted-foreground">No cost data.</p>
              )}
            </CardContent>
          </Card>
        </div>

        {/* Decision list */}
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base font-medium">
              Decision history{decisions.length > 0 ? ` (${decisions.length})` : ""}
            </CardTitle>
          </CardHeader>
          <CardContent>
            {loading ? (
              <div className="flex h-40 items-center justify-center text-sm text-muted-foreground">
                Loading…
              </div>
            ) : (
              <DecisionList decisions={decisions} />
            )}
          </CardContent>
        </Card>
      </main>
    </div>
  );
}
