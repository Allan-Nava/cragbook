import { expect, test, type Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

// The made-up crags of e2e/cragcast. Each test starts from an empty diary.
test.beforeEach(async ({ request }) => {
  const entries: { id: string }[] = await (await request.get("/api/diary")).json();
  for (const e of entries) expect((await request.delete(`/api/routes/${e.id}/ascent`)).status()).toBe(204);
});

async function accessible(page: Page, view: string) {
  const { violations } = await new AxeBuilder({ page }).analyze();
  expect(violations.map(v => `${view}: ${v.id} — ${v.help} (${v.nodes.map(n => n.target).join(", ")})`)).toEqual([]);
}

test("a route marked stays marked after a reload and is in the diary", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("searchbox", { name: "Cerca una falesia o un comune" }).fill("prova");
  await page.getByRole("link", { name: "Parete di Prova" }).click();
  await expect(page.getByRole("heading", { level: 2, name: "Parete di Prova" })).toBeVisible();

  const route = page.getByRole("listitem").filter({ hasText: "Diedro Finto" });
  await route.getByRole("button", { name: "Segna" }).click();
  const form = page.getByRole("form", { name: "Diario per Diedro Finto" });
  await form.getByRole("radio", { name: "Salita" }).check();
  await form.getByLabel("Il giorno").fill("2026-06-14");
  await form.getByLabel("Note").fill("a vista, il passo duro al quinto spit");
  await form.getByRole("button", { name: "Salva" }).click();
  await expect(route.getByRole("button", { name: "Salita il 14 giu 2026" })).toBeVisible();

  await page.reload();
  await expect(page.getByRole("listitem").filter({ hasText: "Diedro Finto" }).getByRole("button", { name: "Salita il 14 giu 2026" })).toBeVisible();
  await expect(page.getByText("4 vie, 1 salita")).toBeVisible();

  await page.getByRole("navigation", { name: "Sezioni" }).getByRole("link", { name: "Diario" }).click();
  await expect(page.getByRole("heading", { level: 3, name: "Salita — 1 via" })).toBeVisible();
  const entry = page.getByRole("listitem").filter({ hasText: "Diedro Finto" });
  await expect(entry).toContainText("Parete di Prova");
  await expect(entry).toContainText("14 giu 2026");
  await expect(entry).toContainText("a vista, il passo duro al quinto spit");
});

test("a route taken out of the diary leaves it", async ({ page }) => {
  await page.goto("/#/crag/osm-node-1");
  const route = page.getByRole("listitem").filter({ hasText: "Placca Immaginaria" });
  await route.getByRole("button", { name: "Segna" }).click();
  await page.getByRole("form", { name: "Diario per Placca Immaginaria" }).getByRole("radio", { name: "Progetto" }).check();
  await page.getByRole("button", { name: "Salva" }).click();
  await route.getByRole("button", { name: "Progetto" }).click();
  await page.getByRole("button", { name: "Togli dal diario" }).click();
  await expect(route.getByRole("button", { name: "Segna" })).toBeVisible();

  await page.goto("/#/diario");
  await expect(page.getByText("Il diario è vuoto")).toBeVisible();
});

test("every view passes axe", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByText("1 falesia")).toBeVisible();
  await accessible(page, "the crags");

  await page.getByRole("checkbox", { name: "Solo con vie" }).uncheck();
  await expect(page.getByText("2 falesie")).toBeVisible();
  await accessible(page, "every crag");

  await page.goto("/#/diario");
  await expect(page.getByText("Il diario è vuoto")).toBeVisible();
  await accessible(page, "the diary, empty");

  await page.goto("/#/crag/osm-node-9");
  await expect(page.getByText("OpenStreetMap non ha vie per questa falesia.")).toBeVisible();
  await accessible(page, "a crag without routes");

  await page.goto("/#/crag/osm-node-1");
  await page.getByRole("button", { name: "6 impegnativo · 1" }).click();
  await accessible(page, "a crag, one level shown");

  await page.getByRole("button", { name: /^Tutte/ }).click();
  await page.getByRole("listitem").filter({ hasText: "Diedro Finto" }).getByRole("button", { name: "Segna" }).click();
  await accessible(page, "a crag, the diary's form open");
  await page.getByRole("button", { name: "Salva" }).click();
  await expect(page.getByRole("button", { name: /^Salita/ })).toBeVisible();
  await accessible(page, "a crag, a route climbed");

  await page.goto("/#/diario");
  await expect(page.getByRole("heading", { level: 3, name: "Salita — 1 via" })).toBeVisible();
  await accessible(page, "the diary");
});
