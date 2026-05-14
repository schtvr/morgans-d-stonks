import { apiFetch } from "@/lib/api";

export type LabControlState = {
  openclawPaused: boolean;
  circuitOpen: boolean;
  circuitReason?: string;
  updatedAt: string;
};

export type LabSignalEvent = {
  id: number;
  type: string;
  symbol: string;
  productId?: string;
  source?: string;
  currentPrice: number;
  previousPrice?: number | null;
  deltaAmount?: number | null;
  deltaPct: number;
  thresholdPct: number;
  firedAt: string;
  discordStatus: string;
  createdAt: string;
};

export type LabOpenClawRun = {
  requestId: string;
  signalId: number;
  status: string;
  attempts: number;
  analysis?: string;
  recommendation?: string;
  confidence?: number | null;
  toolNames: string[];
  errorText?: string;
  createdAt: string;
  updatedAt: string;
};

export type LabOverview = {
  control: LabControlState;
  recentSignals: LabSignalEvent[];
  recentRuns: LabOpenClawRun[];
};

export type LabTelemetryPoint = {
  bucket: string;
  symbol: string;
  currentPrice: number;
  deltaPct: number;
  thresholdPct: number;
  signalCount: number;
};

export type SignalSettings = {
  moveThresholdPct: number;
  cooldown: string;
  updatedAt?: string;
};

export type SignalSettingsVersion = {
  id: number;
  moveThresholdPct: number;
  cooldown: string;
  reason?: string;
  createdAt: string;
};

async function readJSON<T>(res: Response): Promise<T> {
  if (!res.ok) {
    throw new Error(await res.text());
  }
  return (await res.json()) as T;
}

export async function fetchLabOverview() {
  return readJSON<LabOverview>(await apiFetch("/api/lab/overview"));
}

export async function fetchLabSignals(limit = 50) {
  return readJSON<{ signals: LabSignalEvent[] }>(await apiFetch(`/api/lab/signals?limit=${limit}`));
}

export async function fetchLabRuns(limit = 50) {
  return readJSON<{ runs: LabOpenClawRun[] }>(await apiFetch(`/api/lab/runs?limit=${limit}`));
}

export async function fetchLabTelemetry(window = "24h") {
  return readJSON<{ points: LabTelemetryPoint[] }>(await apiFetch(`/api/lab/telemetry?window=${encodeURIComponent(window)}`));
}

export async function pauseOpenClaw() {
  return readJSON<{ control: LabControlState; message: string }>(await apiFetch("/api/lab/openclaw/pause", { method: "POST" }));
}

export async function resumeOpenClaw() {
  return readJSON<{ control: LabControlState; message: string }>(await apiFetch("/api/lab/openclaw/resume", { method: "POST" }));
}

export async function resetOpenClawCircuit() {
  return readJSON<{ control: LabControlState; message: string }>(await apiFetch("/api/lab/openclaw/circuit/reset", { method: "POST" }));
}

export async function retryLabRun(requestId: string) {
  return readJSON<LabOpenClawRun>(await apiFetch(`/api/lab/runs/${encodeURIComponent(requestId)}/retry`, { method: "POST" }));
}

export async function createLabNote(body: { signalId?: number; requestId?: string; body: string }) {
  return readJSON(await apiFetch("/api/lab/notes", { method: "POST", body: JSON.stringify(body) }));
}

export async function fetchSignalSettingsHistory() {
  return readJSON<{ versions: SignalSettingsVersion[] }>(await apiFetch("/api/lab/signal-settings/history?limit=10"));
}

export async function revertSignalSettings(versionId: number) {
  return readJSON<SignalSettings>(
    await apiFetch("/api/lab/signal-settings/revert", {
      method: "POST",
      body: JSON.stringify({ versionId }),
    }),
  );
}
