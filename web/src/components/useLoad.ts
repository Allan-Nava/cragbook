import { useCallback, useEffect, useState } from "react";

/** Loads with fn whenever key changes; reload() loads again. A stale answer never wins: fn is
 * captured per key, so callers pass in key everything fn reads. */
export function useLoad<T>(fn: () => Promise<T>, key: string) {
  const [state, setState] = useState<{ data?: T; error?: string; loading: boolean }>({ loading: true });
  const [tick, setTick] = useState(0);
  const run = useCallback(fn, [key, tick]);
  useEffect(() => {
    let live = true;
    setState(s => ({ ...s, loading: true }));
    run().then(data => { if (live) setState({ data, loading: false }); },
               (e: unknown) => { if (live) setState({ error: e instanceof Error ? e.message : String(e), loading: false }); });
    return () => { live = false; };
  }, [run]);
  return { ...state, reload: () => setTick(t => t + 1) };
}
