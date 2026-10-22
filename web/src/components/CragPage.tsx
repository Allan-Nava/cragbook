import { useState } from "react";
import { useApi } from "../api/client";
import { LEVELS, STATUS, dayLabel, many, today } from "../domain/labels";
import type { Level, Route, Status } from "../domain/types";
import { useLoad } from "./useLoad";

/** One crag: its routes by level, each with the diary's entry, editable in place. */
export function CragPage({ id }: { id: string }) {
  const api = useApi();
  const crag = useLoad(() => api.crag(id), id);
  const [shown, setShown] = useState<Level | "all">("all");
  if (crag.error) return <p role="alert" className="error">{crag.error}</p>;
  if (!crag.data) return <p className="muted">Caricamento…</p>;
  const c = crag.data;
  const groups = LEVELS.map(l => ({ ...l, routes: c.routeList.filter(r => r.level === l.level) })).filter(g => g.routes.length);
  return (
    <section aria-labelledby="crag-title">
      <p><a href="#/">← Falesie</a></p>
      <h2 id="crag-title">{c.name}</h2>
      <p className="muted">
        {[c.municipality && `${c.municipality}${c.provinceCode ? ` (${c.provinceCode})` : ""}`,
          c.elevation != null && `quota ${c.elevation} m`, c.aspect && `esposta a ${c.aspect}`,
          `${many(c.routes, "via", "vie")}, ${many(c.done, "salita", "salite")}`].filter(Boolean).join(" · ")}
        {" · "}<a href={c.osm} rel="noopener">Su OpenStreetMap</a>
      </p>
      {c.routeList.length === 0 && <p>OpenStreetMap non ha vie per questa falesia.</p>}
      {groups.length > 1 && (
        <div className="levels" role="group" aria-label="Livello">
          <button type="button" aria-pressed={shown === "all"} onClick={() => setShown("all")}>{`Tutte · ${c.routeList.length}`}</button>
          {groups.map(g => (
            <button key={g.level} type="button" aria-pressed={shown === g.level} onClick={() => setShown(g.level)}>
              <b>{g.label}</b>{g.word && ` ${g.word}`}{` · ${g.routes.length}`}
            </button>
          ))}
        </div>
      )}
      {groups.filter(g => shown === "all" || g.level === shown).map(g => (
        <div key={g.level} className="group">
          <h3>{[g.level ? `Livello ${g.label}` : g.label, g.word].filter(Boolean).join(" · ")} — {many(g.routes.length, "via", "vie")}</h3>
          <ul className="routes">
            {g.routes.map(r => <RouteItem key={r.id} route={r} onChange={crag.reload} />)}
          </ul>
        </div>
      ))}
      <p className="muted small">Vie © OpenStreetMap contributors (ODbL), tramite cragcast.</p>
    </section>
  );
}

function RouteItem({ route, onChange }: { route: Route; onChange(): void }) {
  const [open, setOpen] = useState(false);
  const facts = [route.length != null && `${route.length.toLocaleString("it-IT")} m`, route.bolts != null && `${route.bolts} spit`,
    route.pitches != null && route.pitches > 1 && `${route.pitches} tiri`, route.sector].filter(Boolean);
  return (
    <li className={route.ascent ? `has-${route.ascent.status}` : undefined}>
      <div className="route">
        <span className="grade">{route.grade ?? "—"}</span>
        <span>
          <a href={route.osm} rel="noopener">{route.name ?? "Senza nome"}</a>
          {route.gone && <span className="muted"> · non più su OpenStreetMap</span>}
          {facts.length > 0 && <span className="muted">{` · ${facts.join(" · ")}`}</span>}
        </span>
        <button type="button" className="ascent" aria-expanded={open} onClick={() => setOpen(o => !o)}>
          {route.ascent ? STATUS[route.ascent.status] + (route.ascent.climbedOn ? ` il ${dayLabel(route.ascent.climbedOn)}` : "") : "Segna"}
        </button>
      </div>
      {open && <AscentForm route={route} onSaved={() => { setOpen(false); onChange(); }} />}
    </li>
  );
}

export function AscentForm({ route, onSaved }: { route: Route; onSaved(): void }) {
  const api = useApi();
  const [status, setStatus] = useState<Status>(route.ascent?.status ?? "done");
  const [climbedOn, setClimbedOn] = useState(route.ascent?.climbedOn ?? "");
  const [notes, setNotes] = useState(route.ascent?.notes ?? "");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const act = async (f: () => Promise<unknown>) => {
    setBusy(true);
    setError(null);
    try { await f(); onSaved(); }
    catch (e) { setError(e instanceof Error ? e.message : String(e)); }
    finally { setBusy(false); }
  };
  const label = route.name ?? "la via";
  return (
    <form className="ascent-form" aria-label={`Diario per ${label}`}
          onSubmit={e => { e.preventDefault(); act(() => api.recordAscent(route.id, { status, climbedOn: status === "done" && climbedOn ? climbedOn : null, notes })); }}>
      <fieldset>
        <legend>Stato</legend>
        {(Object.keys(STATUS) as Status[]).map(s => (
          <label key={s}><input type="radio" name={`status-${route.id}`} value={s} checked={status === s} onChange={() => setStatus(s)} /> {STATUS[s]}</label>
        ))}
      </fieldset>
      {status === "done" && (
        <label>Il giorno <input type="date" value={climbedOn} max={today()} onChange={e => setClimbedOn(e.target.value)} /></label>
      )}
      <label>Note <textarea value={notes} maxLength={2000} rows={2} onChange={e => setNotes(e.target.value)} /></label>
      <div className="actions">
        <button type="submit" disabled={busy}>Salva</button>
        {route.ascent && <button type="button" disabled={busy} onClick={() => act(() => api.clearAscent(route.id))}>Togli dal diario</button>}
      </div>
      {error && <p role="alert" className="error">{error}</p>}
    </form>
  );
}
