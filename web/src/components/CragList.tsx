import { useState } from "react";
import { useApi } from "../api/client";
import { many } from "../domain/labels";
import type { ImportSummary } from "../domain/types";
import { useLoad } from "./useLoad";

/** The crags, searched by name or municipality; by default only those with routes. */
export function CragList() {
  const api = useApi();
  const [q, setQ] = useState("");
  const [withRoutes, setWithRoutes] = useState(true);
  const list = useLoad(() => api.crags({ q: q.trim(), withRoutes }), `${q.trim()}|${withRoutes}`);
  return (
    <section aria-labelledby="crags-title">
      <h2 id="crags-title">Falesie</h2>
      <div className="toolbar">
        <input type="search" aria-label="Cerca una falesia o un comune" placeholder="Cerca una falesia o un comune"
               value={q} maxLength={100} onChange={e => setQ(e.target.value)} />
        <label><input type="checkbox" checked={withRoutes} onChange={e => setWithRoutes(e.target.checked)} /> Solo con vie</label>
        <ImportButton onDone={list.reload} />
      </div>
      {list.error && <p role="alert" className="error">{list.error}</p>}
      {!list.data && !list.error && <p className="muted">Caricamento…</p>}
      {list.data && (
        <>
          <p className="muted" aria-live="polite">{many(list.data.length, "falesia", "falesie")}</p>
          <ul className="cards">
            {list.data.map(c => (
              <li key={c.id}>
                <a href={`#/crag/${encodeURIComponent(c.id)}`}><b>{c.name}</b></a>
                <span className="muted">{[c.municipality && `${c.municipality}${c.provinceCode ? ` (${c.provinceCode})` : ""}`,
                  c.routes ? `${many(c.routes, "via", "vie")}, ${many(c.done, "salita", "salite")}` : "nessuna via su OSM"].filter(Boolean).join(" · ")}</span>
              </li>
            ))}
          </ul>
        </>
      )}
    </section>
  );
}

/** Refreshes the catalogue from cragcast; the diary is never touched by it. */
export function ImportButton({ onDone }: { onDone(): void }) {
  const api = useApi();
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<{ ok?: ImportSummary; error?: string } | null>(null);
  const run = async () => {
    setBusy(true);
    try { setResult({ ok: await api.runImport() }); onDone(); }
    catch (e) { setResult({ error: e instanceof Error ? e.message : String(e) }); }
    finally { setBusy(false); }
  };
  return (
    <>
      <button type="button" onClick={run} disabled={busy}>{busy ? "Aggiornamento…" : "Aggiorna da cragcast"}</button>
      <span role="status">
        {result?.ok && ` ${many(result.ok.crags, "falesia", "falesie")} e ${many(result.ok.routes, "via", "vie")} aggiornate` +
          (result.ok.failed.length ? `, ${result.ok.failed.length} non riuscite` : "") + "."}
        {result?.error && ` Aggiornamento non riuscito: ${result.error}`}
      </span>
    </>
  );
}
