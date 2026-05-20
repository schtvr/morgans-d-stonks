import { apiFetch } from "@/lib/api";

export type AgentAction = "buy" | "sell" | "ignore";
export type TriggerKind = "signal" | "daily";

export interface ToolCall {
  name: string;
  input: unknown;
  output: unknown;
  durationMs: number;
}

export interface AgentDecision {
  id: number;
  triggerKind: TriggerKind;
  triggerAt: string;
  symbol: string;
  action: AgentAction;
  confidence: number;
  rationale: string;
  sizingHintNotional?: number;
  model: string;
  promptVersion: string;
  latencyMs: number;
  costCents: number;
  requestJson?: unknown;
  responseJson?: unknown;
  toolCallsJson?: ToolCall[];
  createdAt: string;
}

export interface AgentDecisionsResponse {
  decisions: AgentDecision[];
}

export interface AgentBenchmarkPoint {
  asOf: string;
  realizedReturnPct: number;
  btcReturnPct: number;
  excessReturnPct: number;
  decisionCount: number;
  ignoreCount: number;
}

export interface AgentBenchmarkResponse {
  window: string;
  headlineExcessPct: number;
  points: AgentBenchmarkPoint[];
  noteShadowFeesNotPaid: boolean;
  noteWindowIncomplete: boolean;
}

export interface AgentCostPoint {
  day: string;
  costCents: number;
  decisions: number;
}

export interface AgentCostResponse {
  window: string;
  capCents: number;
  today: AgentCostPoint;
  points: AgentCostPoint[];
}

async function readJSON<T>(res: Response): Promise<T> {
  if (!res.ok) {
    throw new Error(await res.text());
  }
  return (await res.json()) as T;
}

export async function fetchAgentDecisions(params?: {
  symbol?: string;
  action?: string;
  limit?: number;
  from?: string;
  to?: string;
}): Promise<AgentDecisionsResponse> {
  const qs = new URLSearchParams();
  if (params?.symbol) qs.set("symbol", params.symbol);
  if (params?.action) qs.set("action", params.action);
  if (params?.limit != null) qs.set("limit", String(params.limit));
  if (params?.from) qs.set("from", params.from);
  if (params?.to) qs.set("to", params.to);
  const query = qs.toString() ? `?${qs.toString()}` : "";
  return readJSON<AgentDecisionsResponse>(await apiFetch(`/api/agent/decisions${query}`));
}

export async function fetchAgentDecision(id: number): Promise<AgentDecision> {
  return readJSON<AgentDecision>(await apiFetch(`/api/agent/decisions/${id}`));
}

export async function fetchAgentBenchmark(window = "14d"): Promise<AgentBenchmarkResponse> {
  return readJSON<AgentBenchmarkResponse>(
    await apiFetch(`/api/agent/benchmark?window=${encodeURIComponent(window)}`),
  );
}

export async function fetchAgentCost(window = "7d"): Promise<AgentCostResponse> {
  return readJSON<AgentCostResponse>(
    await apiFetch(`/api/agent/cost?window=${encodeURIComponent(window)}`),
  );
}
