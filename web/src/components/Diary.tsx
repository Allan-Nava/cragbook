import { useApi } from "../api/client";
import { STATUS, dayLabel, many } from "../domain/labels";
import type { Status } from "../domain/types";
import { useLoad } from "./useLoad";

/** The diary: the routes climbed, latest first, then the projects and those to try. */
export function Diary() {
  const api = useApi();
  const diary = useLoad(() => api.diary(), "diary");
  if (diary.error) return <p role="alert" className="error">{diary.error}</p>;
  if (!diary.data) return <p className="muted">Caricamento…</p>;
  const entries = diary.data;
  return (
    <section aria-labelledby="diary-title">
      <h2 id="diary-title">Diario</h2>
      {entries.length === 0 && <p>Il diario è vuoto: apri una falesia e segna una via.</p>}
      {(["done", "project", "todo"] as Status[]).map(status => {
        const list = entries.filter(e => e.ascent?.status === status);
        return list.length > 0 && (
          <div key={status} className="group">
            <h3>{STATUS[status]} — {many(list.length, "via", "vie")}</h3>
            <ul className="routes">
              {list.map(e => (
                <li key={e.id}>
                  <div className="route">
                    <span className="grade">{e.grade ?? "—"}</span>
                    <span>
                      <b>{e.name ?? "Senza nome"}</b>{" · "}<a href={`#/crag/${encodeURIComponent(e.cragId)}`}>{e.cragName}</a>
                      {e.ascent?.climbedOn && <span className="muted">{` · ${dayLabel(e.ascent.climbedOn)}`}</span>}
                      {e.ascent?.notes && <span className="notes">{e.ascent.notes}</span>}
                    </span>
                  </div>
                </li>
              ))}
            </ul>
          </div>
        );
      })}
    </section>
  );
}
