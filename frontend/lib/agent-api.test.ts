import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { fetchAgentDecisions, fetchAgentBenchmark, fetchAgentCost } from "./agent-api";

const mockDecisionsResponse = {
  decisions: [
    {
      id: 1,
      triggerKind: "signal",
      triggerAt: "2026-05-18T10:00:00Z",
      symbol: "BTC-USD",
      action: "buy",
      confidence: 0.82,
      rationale: "Strong momentum signal detected.",
      model: "claude-3-5-sonnet-20241022",
      promptVersion: "abc123",
      latencyMs: 1500,
      costCents: 3,
      createdAt: "2026-05-18T10:00:01Z",
    },
  ],
};

const mockBenchmarkResponse = {
  window: "14d",
  headlineExcessPct: 2.45,
  points: [
    {
      asOf: "2026-05-04T00:00:00Z",
      realizedReturnPct: 5.1,
      btcReturnPct: 2.65,
      excessReturnPct: 2.45,
      decisionCount: 3,
      ignoreCount: 1,
    },
  ],
  noteShadowFeesNotPaid: true,
  noteWindowIncomplete: false,
};

const mockCostResponse = {
  window: "7d",
  capCents: 500,
  today: {
    day: "2026-05-18",
    costCents: 12,
    decisions: 4,
  },
  points: [
    { day: "2026-05-12", costCents: 8, decisions: 2 },
    { day: "2026-05-13", costCents: 10, decisions: 3 },
  ],
};

function makeFetchMock(body: unknown, ok = true) {
  return vi.fn().mockResolvedValue({
    ok,
    json: () => Promise.resolve(body),
    text: () => Promise.resolve(ok ? "" : "error"),
  });
}

beforeEach(() => {
  vi.stubGlobal("fetch", makeFetchMock(mockDecisionsResponse));
  vi.stubGlobal("process", {
    ...process,
    env: { ...process.env, NEXT_PUBLIC_API_URL: undefined },
  });
  // stub sessionStorage
  vi.stubGlobal("sessionStorage", { getItem: () => null, setItem: () => {}, removeItem: () => {} });
  vi.stubGlobal("window", { location: { href: "/" } });
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("fetchAgentDecisions", () => {
  it("returns typed decisions array", async () => {
    vi.stubGlobal("fetch", makeFetchMock(mockDecisionsResponse));
    const res = await fetchAgentDecisions();
    expect(res.decisions).toHaveLength(1);
    expect(res.decisions[0].action).toBe("buy");
    expect(res.decisions[0].confidence).toBe(0.82);
  });

  it("passes symbol filter as query param", async () => {
    const mockFetch = makeFetchMock(mockDecisionsResponse);
    vi.stubGlobal("fetch", mockFetch);
    await fetchAgentDecisions({ symbol: "ETH-USD", limit: 10 });
    const calledUrl = mockFetch.mock.calls[0][0] as string;
    expect(calledUrl).toContain("symbol=ETH-USD");
    expect(calledUrl).toContain("limit=10");
  });

  it("throws on non-ok response", async () => {
    vi.stubGlobal("fetch", makeFetchMock("bad request", false));
    await expect(fetchAgentDecisions()).rejects.toThrow();
  });
});

describe("fetchAgentBenchmark", () => {
  it("parses headlineExcessPct", async () => {
    vi.stubGlobal("fetch", makeFetchMock(mockBenchmarkResponse));
    const res = await fetchAgentBenchmark("14d");
    expect(res.headlineExcessPct).toBe(2.45);
    expect(res.points).toHaveLength(1);
    expect(res.noteShadowFeesNotPaid).toBe(true);
  });

  it("uses default window of 14d", async () => {
    const mockFetch = makeFetchMock(mockBenchmarkResponse);
    vi.stubGlobal("fetch", mockFetch);
    await fetchAgentBenchmark();
    const calledUrl = mockFetch.mock.calls[0][0] as string;
    expect(calledUrl).toContain("window=14d");
  });
});

describe("fetchAgentCost", () => {
  it("parses today.costCents", async () => {
    vi.stubGlobal("fetch", makeFetchMock(mockCostResponse));
    const res = await fetchAgentCost();
    expect(res.today.costCents).toBe(12);
    expect(res.capCents).toBe(500);
  });

  it("passes window param", async () => {
    const mockFetch = makeFetchMock(mockCostResponse);
    vi.stubGlobal("fetch", mockFetch);
    await fetchAgentCost("30d");
    const calledUrl = mockFetch.mock.calls[0][0] as string;
    expect(calledUrl).toContain("window=30d");
  });
});
