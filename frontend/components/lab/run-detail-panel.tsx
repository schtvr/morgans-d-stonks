"use client";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import type { LabOpenClawRun } from "@/lib/lab-api";
import { useState } from "react";

type Props = {
  run: LabOpenClawRun | null;
  busy?: boolean;
  onRetry: (run: LabOpenClawRun) => void;
  onNote: (run: LabOpenClawRun, body: string) => void;
};

export function RunDetailPanel({ run, busy, onRetry, onNote }: Props) {
  const [note, setNote] = useState("");

  if (!run) {
    return (
      <Card className="border-border/70">
        <CardHeader>
          <CardTitle>Run detail</CardTitle>
          <CardDescription>Select a run to inspect the rationale and controls.</CardDescription>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">No run selected.</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="border-border/70">
      <CardHeader>
        <CardTitle>Run detail</CardTitle>
        <CardDescription>{run.requestId}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-3 sm:grid-cols-3">
          <Metric label="Status" value={run.status} />
          <Metric label="Attempts" value={String(run.attempts)} />
          <Metric label="Confidence" value={run.confidence == null ? "n/a" : `${Math.round(run.confidence * 100)}%`} />
        </div>
        {run.recommendation ? (
          <section>
            <p className="text-sm font-medium">Recommendation</p>
            <p className="mt-1 text-sm text-muted-foreground">{run.recommendation}</p>
          </section>
        ) : null}
        {run.analysis ? (
          <section>
            <p className="text-sm font-medium">Rationale</p>
            <p className="mt-1 whitespace-pre-wrap text-sm text-muted-foreground">{run.analysis}</p>
          </section>
        ) : null}
        {run.errorText ? <p className="text-sm text-destructive">{run.errorText}</p> : null}
        <section>
          <p className="text-sm font-medium">Tools</p>
          <p className="mt-1 text-sm text-muted-foreground">{run.toolNames?.length ? run.toolNames.join(", ") : "No tool calls recorded."}</p>
        </section>
        <div className="flex flex-wrap gap-2">
          <Button
            type="button"
            variant="outline"
            disabled={busy}
            onClick={() => {
              if (window.confirm(`Retry ${run.requestId}?`)) onRetry(run);
            }}
          >
            Retry run
          </Button>
        </div>
        <div className="space-y-2">
          <p className="text-sm font-medium">Note</p>
          <div className="flex gap-2">
            <Input value={note} onChange={(e) => setNote(e.target.value)} placeholder="Add an observation for later analysis" />
            <Button
              type="button"
              disabled={busy || note.trim() === ""}
              onClick={() => {
                onNote(run, note);
                setNote("");
              }}
            >
              Save
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border bg-muted/20 p-3">
      <p className="text-xs uppercase tracking-[0.16em] text-muted-foreground">{label}</p>
      <p className="mt-1 font-semibold">{value}</p>
    </div>
  );
}
