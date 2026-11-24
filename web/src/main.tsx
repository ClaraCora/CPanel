import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import PortalApp from "./PortalApp";
import "./public-surfaces.css";
import "./admin-palette.css";
import { initializeTheme, isPortal } from "./theme";

initializeTheme();
const app = isPortal ? <PortalApp /> : <App />;

createRoot(document.getElementById("root")!).render(<StrictMode>{app}</StrictMode>);
