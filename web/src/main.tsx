import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import PortalApp from "./PortalApp";
import "./styles.css";
import "./theme-glass.css";
import "./theme-modes.css";
import { initializeTheme } from "./theme";

initializeTheme();
const app = window.location.pathname === "/edu" || window.location.pathname === "/edu/" ? <PortalApp /> : <App />;

createRoot(document.getElementById("root")!).render(<StrictMode>{app}</StrictMode>);
