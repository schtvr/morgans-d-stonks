"use client";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import type { LabControlState } from "@/lib/lab-api";

type Props = {
  control: LabControlState | null;
  busy?: boolean;
  onPause: () => void;
  onResume: () => void;
  onResetCircuit: () => void;
};

export function LabStatusCard({ control, busy, onPause, onResume, onResetCircuit }: Props) {
  const paused = control?.openclawPaused ?? false;
  const circuitOpen = control?.circuitOpen ?? false;

  return (
    <Card className="relative overflow-hidden border-border/70">
      <div className="absolute inset-x-0 top-0 h-1 bg-gradient-to-r from-cyan-400 via-sky-500 to-indigo-500" />
      <CardHeader>
        <CardTitle>OpenClaw operations</CardTitle>
        <CardDescription>Observe and control signal forwarding without editing bot internals.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-3 sm:grid-cols-3">
          <StatusPill label="Forwarding" value={paused ? "Paused" : "Active"} tone={paused ? "warn" : "ok"} />
          <StatusPill label="Circuit" value={circuitOpen ? "Open" : "Closed"} tone={circuitOpen ? "warn" : "ok"} />
          <StatusPill label="Updated" value={control?.updatedAt ? new Date(control.updatedAt).toLocaleTimeString() : "n/a"} />
        </div>
        {control?.circuitReason ? <p className="text-sm text-muted-foreground">Circuit reason: {control.circuitReason}</p> : null}
        <div className="flex flex-wrap gap-2">
          {paused ? (
            <Button type="button" disabled={busy} onClick={onResume}>
              Resume forwarding
            </Button>
          ) : (
            <Button type="button" variant="outline" disabled={busy} onClick={onPause}>
              Pause forwarding
            </Button>
          )}
          <Button type="button" variant="outline" disabled={busy || !circuitOpen} onClick={onResetCircuit}>
            Reset circuit
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

function StatusPill({ label, value, tone = "neutral" }: { label: string; value: string; tone?: "ok" | "warn" | "neutral" }) {
  const toneClass =
    tone === "ok"
      ? "border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300"
      : tone === "warn"
        ? "border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-300"
        : "border-border bg-muted/30";
  return (
    <div className={`rounded-lg border p-3 ${toneClass}`}>
      <p className="text-xs uppercase tracking-[0.16em] opacity-75">{label}</p>
      <p className="mt-1 text-lg font-semibold">{value}</p>
    </div>
  );
}
