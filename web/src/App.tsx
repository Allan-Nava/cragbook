import { useEffect, useState } from "react";
import { CragList } from "./components/CragList";
import { CragPage } from "./components/CragPage";
import { Diary } from "./components/Diary";

/** The view from the URL's hash: #/ the crags, #/crag/<id> one crag, #/diario the diary. */
export function route(hash: string): { view: "crags" } | { view: "crag"; id: string } | { view: "diary" } {
  const crag = /^#\/crag\/(.+)$/.exec(hash);
  if (crag) return { view: "crag", id: decodeURIComponent(crag[1]) };
  if (hash === "#/diario") return { view: "diary" };
  return { view: "crags" };
}

export function App() {
  const [hash, setHash] = useState(location.hash);
  useEffect(() => {
    const on = () => setHash(location.hash);
    addEventListener("hashchange", on);
    return () => removeEventListener("hashchange", on);
  }, []);
  const r = route(hash);
  return (
    <>
      <header>
        <h1><a href="#/">cragbook</a></h1>
        <p className="muted">Il diario delle tue vie, sulle falesie di <a href="https://allan-nava.github.io/cragcast/" rel="noopener">cragcast</a>.</p>
        <nav aria-label="Sezioni">
          <a href="#/" aria-current={r.view !== "diary" ? "page" : undefined}>Falesie</a>
          <a href="#/diario" aria-current={r.view === "diary" ? "page" : undefined}>Diario</a>
        </nav>
      </header>
      <main>
        {r.view === "crags" && <CragList />}
        {r.view === "crag" && <CragPage key={r.id} id={r.id} />}
        {r.view === "diary" && <Diary />}
      </main>
    </>
  );
}
