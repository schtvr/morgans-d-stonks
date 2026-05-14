"use client";

import { LabStatusCard } from "@/components/lab/lab-status-card";
import { RunDetailPanel } from "@/components/lab/run-detail-panel";
import { SignalRunTable } from "@/components/lab/signal-run-table";
import { SignalSettingsLabCard } from "@/components/lab/signal-settings-lab-card";
import { SignalTelemetryChart } from "@/components/lab/signal-telemetry-chart";
import { SignalTimeline } from "@/components/lab/signal-timeline";
import { SiteHeader } from "@/components/site-header";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  createLabNote,
  fetchLabOverview,
  fetchLabRuns,
  fetchLabSignals,
  fetchLabTelemetry,
  pauseOpenClaw,
  resetOpenClawCircuit,
  resumeOpenClaw,
  retryLabRun,
  type LabControlState,
  type LabOpenClawRun,
  type LabSignalEvent,
  type LabTelemetryPoint,
} from "@/lib/lab-api";
import { useCallback, useEffect, useMemo, useState } from "react";

export default function LabPage() {
  const [control, setControl] = useState<LabControlState | null>(null);
  const [signals, setSignals] = useState<LabSignalEvent[]>([]);
  const [runs, setRuns] = useState<LabOpenClawRun[]>([]);
  const [telemetry, setTelemetry] = useState<LabTelemetryPoint[]>([]);
  const [selectedRun, setSelectedRun] = useState<LabOpenClawRun | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async (silent = false) => {
    if (!silent) setLoading(true);
    setError(null);
    try {
      const [overview, signalResp, runResp, telemetryResp] = await Promise.all([
        fetchLabOverview(),
        fetchLabSignals(50),
        fetchLabRuns(50),
        fetchLabTelemetry("24h"),
      ]);
      setControl(overview.control);
      setSignals(signalResp.signals ?? overview.recentSignals ?? []);
      setRuns(runResp.runs ?? overview.recentRuns ?? []);
      setTelemetry(telemetryResp.points ?? []);
      setSelectedRun((current) => {
        if (!current) return current;
        return (runResp.runs ?? []).find((run) => run.requestId === current.requestId) ?? current;
      });
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load The Lab");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load(false);
    const id = window.setInterval(() => void load(true), 30_000);
    const onVis = () => {
      if (document.visibilityState === "visible") void load(true);
    };
    document.addEventListener("visibilitychange", onVis);
    return () => {
      window.clearInterval(id);
      document.removeEventListener("visibilitychange", onVis);
    };
  }, [load]);

  const stats = useMemo(
    () => [
      { label: "Signals", value: String(signals.length), hint: "Last 50 qualifying events" },
      { label: "Queued runs", value: String(runs.filter((run) => run.status === "queued" || run.status === "retrying").length), hint: "Waiting or retrying" },
      { label: "Errors", value: String(runs.filter((run) => run.status === "failed").length), hint: "OpenClaw failures" },
    ],
    [runs, signals.length],
  );

  async function operate(action: "pause" | "resume" | "reset") {
    const labels = { pause: "Pause OpenClaw forwarding?", resume: "Resume OpenClaw forwarding?", reset: "Reset OpenClaw circuit?" };
    if (!window.confirm(labels[action])) return;
    setBusy(true);
    setError(null);
    try {
      const res = action === "pause" ? await pauseOpenClaw() : action === "resume" ? await resumeOpenClaw() : await resetOpenClawCircuit();
      setControl(res.control);
      await load(true);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Operation failed");
    } finally {
      setBusy(false);
    }
  }

  async function retry(run: LabOpenClawRun) {
    setBusy(true);
    setError(null);
    try {
      const next = await retryLabRun(run.requestId);
      setSelectedRun(next);
      await load(true);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Retry failed");
    } finally {
      setBusy(false);
    }
  }

  async function saveNote(run: LabOpenClawRun, body: string) {
    setBusy(true);
    setError(null);
    try {
      await createLabNote({ requestId: run.requestId, body });
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to save note");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="min-h-screen bg-background text-foreground">
      <SiteHeader />
      <main className="mx-auto max-w-6xl space-y-6 px-4 py-6">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">The Lab</h1>
            <p className="text-sm text-muted-foreground">Crypto-first signal telemetry, OpenClaw operations, and experiment history.</p>
          </div>
          <Button type="button" variant="outline" disabled={loading || busy} onClick={() => void load(false)}>
            Refresh
          </Button>
        </div>

        {error ? <p className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">{error}</p> : null}

        <div className="grid gap-4 md:grid-cols-3">
          {stats.map((stat) => (
            <Card key={stat.label}>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium text-muted-foreground">{stat.label}</CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-2xl font-semibold">{loading ? "..." : stat.value}</p>
                <p className="mt-1 text-xs text-muted-foreground">{stat.hint}</p>
              </CardContent>
            </Card>
          ))}
        </div>

        <div className="grid gap-6 xl:grid-cols-[1fr_0.9fr]">
          <LabStatusCard
            control={control}
            busy={busy}
            onPause={() => void operate("pause")}
            onResume={() => void operate("resume")}
            onResetCircuit={() => void operate("reset")}
          />
          <SignalSettingsLabCard onChanged={() => void load(true)} />
        </div>

        <SignalTelemetryChart points={telemetry} />

        <div className="grid gap-6 xl:grid-cols-[1.4fr_0.8fr]">
          <SignalRunTable signals={signals} runs={runs} onSelectRun={setSelectedRun} />
          <RunDetailPanel run={selectedRun} busy={busy} onRetry={(run) => void retry(run)} onNote={(run, body) => void saveNote(run, body)} />
        </div>

        <SignalTimeline signals={signals} runs={runs} />
      </main>
    </div>
  );
}
