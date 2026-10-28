import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import { ApiContext, HttpApi } from "./api/client";
import { BrowserApi } from "./api/browser";
import "./style.css";

// `vite build --mode demo` is the page without a server, as GitHub Pages serves it.
const demo = import.meta.env.MODE === "demo";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ApiContext.Provider value={demo ? new BrowserApi() : new HttpApi()}><App demo={demo} /></ApiContext.Provider>
  </StrictMode>,
);
