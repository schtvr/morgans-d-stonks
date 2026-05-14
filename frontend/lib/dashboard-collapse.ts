export type CollapseState = Record<string, boolean>;

type CollapseStorage = Pick<Storage, "getItem" | "setItem" | "removeItem">;

export function readCollapseState(storage: CollapseStorage | null | undefined, key: string): CollapseState {
  if (!storage) return {};
  const raw = storage.getItem(key);
  if (!raw) return {};
  try {
    const parsed = JSON.parse(raw) as unknown;
    if (!parsed || Array.isArray(parsed) || typeof parsed !== "object") {
      return {};
    }
    return Object.fromEntries(
      Object.entries(parsed).filter((entry): entry is [string, boolean] => typeof entry[1] === "boolean"),
    );
  } catch {
    return {};
  }
}

export function writeCollapseState(storage: CollapseStorage | null | undefined, key: string, state: CollapseState) {
  if (!storage) return;
  storage.setItem(key, JSON.stringify(state));
}

export function toggleCollapseState(storage: CollapseStorage | null | undefined, key: string, id: string) {
  const state = readCollapseState(storage, key);
  const next = !(state[id] ?? false);
  writeCollapseState(storage, key, { ...state, [id]: next });
  return next;
}
