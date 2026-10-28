import { afterEach, describe, expect, it } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { ApiContext } from "./api/client";
import { App, route } from "./App";
import { fakeApi } from "./test/fakeApi";
import { today } from "./domain/labels";

afterEach(() => { cleanup(); location.hash = ""; });

function show(hash: string) {
  location.hash = hash;
  const api = fakeApi();
  render(<ApiContext.Provider value={api}><App /></ApiContext.Provider>);
  return api;
}

describe("routing", () => {
  it("reads the view from the hash", () => {
    expect(route("")).toEqual({ view: "crags" });
    expect(route("#/diario")).toEqual({ view: "diary" });
    expect(route("#/crag/osm-node-1")).toEqual({ view: "crag", id: "osm-node-1" });
    expect(route("#/qualcosa")).toEqual({ view: "crags" });
  });

  it("follows the hash as it changes", async () => {
    show("#/");
    await screen.findByText("Test Wall");
    act(() => { location.hash = "#/diario"; dispatchEvent(new HashChangeEvent("hashchange")); });
    expect(await screen.findByText(/Il diario è vuoto/)).toBeTruthy();
  });
});

it("says, in a demo, that the diary stays in this browser", () => {
  render(<ApiContext.Provider value={fakeApi()}><App demo /></ApiContext.Provider>);
  expect(screen.getByRole("note").textContent).toMatch(/il diario resta in questo browser/);
});

describe("the crag list", () => {
  it("shows the crags with routes, then all, then a search", async () => {
    const api = show("#/");
    expect(await screen.findByText("Test Wall")).toBeTruthy();
    expect(screen.getByText("Lecco (LC) · 4 vie, 0 salite")).toBeTruthy();
    expect(screen.queryByText("Empty Wall")).toBeNull();
    fireEvent.click(screen.getByLabelText(/Solo con vie/));
    expect(await screen.findByText("Empty Wall")).toBeTruthy();
    expect(screen.getByText("nessuna via su OSM")).toBeTruthy();
    fireEvent.change(screen.getByLabelText("Cerca una falesia o un comune"), { target: { value: "empty" } });
    await waitFor(() => expect(screen.queryByText("Test Wall")).toBeNull());
    expect(api.calls).toContain("crags empty|false");
  });

  it("refreshes from cragcast and says how it went", async () => {
    const api = show("#/");
    await screen.findByText("Test Wall");
    fireEvent.click(screen.getByRole("button", { name: "Aggiorna da cragcast" }));
    expect(await screen.findByText(/220 falesie e 116 vie aggiornate, 1 non riuscite\./)).toBeTruthy();
    expect(api.calls).toContain("import");
  });
});

describe("a crag", () => {
  it("groups its routes by level and filters one level", async () => {
    show("#/crag/osm-node-1");
    expect(await screen.findByRole("heading", { name: "Test Wall" })).toBeTruthy();
    const buttons = within(screen.getByRole("group", { name: "Livello" })).getAllByRole("button").map(b => b.textContent);
    expect(buttons).toEqual(["Tutte · 4", "4 e meno facile · 1", "6 impegnativo · 2", "Senza grado · 1"]);
    expect(screen.getAllByRole("heading", { level: 3 }).map(h => h.textContent))
      .toEqual(["Livello 4 e meno · facile — 1 via", "Livello 6 · impegnativo — 2 vie", "Senza grado — 1 via"]);
    expect(screen.getByText(/25 m · 11 spit · Settore Alto/)).toBeTruthy();
    expect(screen.getByText(/3 tiri/)).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: /^6 impegnativo/ }));
    expect(screen.getAllByRole("heading", { level: 3 })).toHaveLength(1);
    expect(screen.queryByText("Placca")).toBeNull();
  });

  it("records an ascent with its day and notes, then clears it", async () => {
    const api = show("#/crag/osm-node-1");
    await screen.findByText("Diedro");
    const row = screen.getByText("Diedro").closest("li")!;
    fireEvent.click(within(row).getByRole("button", { name: "Segna" }));
    const form = within(row).getByRole("form", { name: "Diario per Diedro" });
    fireEvent.change(within(form).getByLabelText("Il giorno"), { target: { value: "2026-10-20" } });
    fireEvent.change(within(form).getByLabelText("Note"), { target: { value: "a vista" } });
    fireEvent.click(within(form).getByRole("button", { name: "Salva" }));
    expect(await within(row).findByRole("button", { name: "Salita il 20 ott 2026" })).toBeTruthy();
    expect(api.calls).toContain(`record 3 {"status":"done","climbedOn":"2026-10-20","notes":"a vista"}`);
    expect(row.className).toBe("has-done");
    fireEvent.click(within(row).getByRole("button", { name: /Salita/ }));
    fireEvent.click(within(row).getByRole("button", { name: "Togli dal diario" }));
    expect(await within(row).findByRole("button", { name: "Segna" })).toBeTruthy();
    expect(api.calls).toContain("clear 3");
  });

  it("sends no day for a project, and shows the API's refusal", async () => {
    const api = show("#/crag/osm-node-1");
    await screen.findByText("Tetto");
    const row = screen.getByText("Tetto").closest("li")!;
    fireEvent.click(within(row).getByRole("button", { name: "Segna" }));
    expect(within(row).getByLabelText("Il giorno").getAttribute("max")).toBe(today());   // no day after today
    fireEvent.change(within(row).getByLabelText("Note"), { target: { value: "rifiutata" } });
    fireEvent.click(within(row).getByRole("button", { name: "Salva" }));
    expect((await within(row).findByRole("alert")).textContent).toContain("refused by the test");
    fireEvent.change(within(row).getByLabelText("Note"), { target: { value: "" } });
    fireEvent.click(within(row).getByLabelText(/Progetto/));
    expect(within(row).queryByLabelText("Il giorno")).toBeNull();
    fireEvent.click(within(row).getByRole("button", { name: "Salva" }));
    await within(row).findByRole("button", { name: "Progetto" });
    expect(api.calls).toContain(`record 4 {"status":"project","climbedOn":null,"notes":""}`);
  });

  it("says so for a crag without routes, and for one that does not exist", async () => {
    show("#/crag/osm-node-9");
    expect(await screen.findByText("OpenStreetMap non ha vie per questa falesia.")).toBeTruthy();
    cleanup();
    show("#/crag/nowhere");
    expect((await screen.findByRole("alert")).textContent).toBe("crag not found");
  });
});

describe("the diary", () => {
  it("lists the routes by status, with day, notes and a link to the crag", async () => {
    const api = fakeApi();
    await api.recordAscent("3", { status: "done", climbedOn: "2026-10-20", notes: "a vista" });
    await api.recordAscent("4", { status: "project", climbedOn: null, notes: "" });
    location.hash = "#/diario";
    render(<ApiContext.Provider value={api}><App /></ApiContext.Provider>);
    expect((await screen.findAllByRole("heading", { level: 3 })).map(h => h.textContent)).toEqual(["Salita — 1 via", "Progetto — 1 via"]);
    expect(screen.getByText("a vista")).toBeTruthy();
    expect(screen.getByText(/20 ott 2026/)).toBeTruthy();
    expect(screen.getAllByRole("link", { name: "Test Wall" })[0].getAttribute("href")).toBe("#/crag/osm-node-1");
  });
});
