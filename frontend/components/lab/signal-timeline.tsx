"use client";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import type { LabOpenClawRun, LabSignalEvent } from "@/lib/lab-api";

type Props = {
  signals: LabSignalEvent[];
  runs: LabOpenClawRun[];
};

export function SignalTimeline({ signals, runs }: Props) {
  const runBySignal = new Map(runs.map((run) => [run.signalId, run]));

  return (
    <Card className="border-border/70">
      <CardHeader>
        <CardTitle>Timeline</CardTitle>
        <CardDescription>Recent qualifying signals and their Lab response state.</CardDescription>
      </CardHeader>
      <CardContent>
        {signals.length === 0 ? (
          <p className="text-sm text-muted-foreground">No signal timeline yet.</p>
        ) : (
          <ol className="relative space-y-4 border-l pl-5">
            {signals.slice(0, 12).map((signal) => {
              const run = runBySignal.get(signal.id);
              return (
                <li key={signal.id} className="relative">
                  <span className="absolute -left-[1.8rem] top-1 h-3 w-3 rounded-full border bg-background" />
                  <div className="rounded-lg border bg-muted/20 p-3">
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <p className="text-sm font-semibold">
                          {signal.symbol} {signal.deltaPct >= 0 ? "+" : ""}
                          {signal.deltaPct.toFixed(2)}%
                        </p>
                        <p className="text-xs text-muted-foreground">{new Date(signal.firedAt).toLocaleString()}</p>
                      </div>
                      <span className="rounded-full border px-2 py-1 text-xs">{run?.status ?? "not queued"}</span>
                    </div>
                    <p className="mt-2 text-xs text-muted-foreground">
                      Discord: {signal.discordStatus || "signal_only"}; threshold {signal.thresholdPct.toFixed(2)}%
                    </p>
                    {run?.recommendation ? <p className="mt-2 text-sm">{run.recommendation}</p> : null}
                  </div>
                </li>
              );
            })}
          </ol>
        )}
      </CardContent>
    </Card>
  );
}
