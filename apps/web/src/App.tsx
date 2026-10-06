import { lazy, Suspense, useEffect, useState } from "react";
import { NavLink, Route, Routes, useLocation } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import {
  Activity as ActivityIcon,
  ArrowUpRight,
  Bot,
  ChevronRight,
  CircleHelp,
  Command,
  GitBranch,
  LayoutDashboard,
  Menu,
  MessageSquare,
  Moon,
  Radar,
  Settings as SettingsIcon,
  ShieldCheck,
  SlidersHorizontal,
  Sparkles,
  X,
  Zap,
} from "lucide-react";
import { useAPI } from "./api";
import type { Overview as OverviewType } from "./types";
import Overview from "./pages/Overview";
import Operator from "./components/Operator";
import { Badge, Empty } from "./components/primitives";
import { Button } from "./components/ui/button";
import { Dialog } from "./components/ui/dialog";
import Discovery from "./components/Discovery";
const Login = lazy(() => import("./pages/Login"));
const Configuration = lazy(() => import("./pages/Configuration"));
const Opportunities = lazy(() => import("./pages/Opportunities"));
const Activity = lazy(() =>
  import("./pages/System").then((m) => ({ default: m.Activity })),
);
const Agents = lazy(() =>
  import("./pages/System").then((m) => ({ default: m.Agents })),
);
const Contributions = lazy(() =>
  import("./pages/System").then((m) => ({ default: m.Contributions })),
);
const Settings = lazy(() =>
  import("./pages/System").then((m) => ({ default: m.Settings })),
);
const Usage = lazy(() =>
  import("./pages/System").then((m) => ({ default: m.Usage })),
);
const navigation = [
  { to: "/", label: "Overview", icon: LayoutDashboard },
  { to: "/opportunities", label: "Opportunities", icon: Radar },
  { to: "/contributions", label: "Contributions", icon: GitBranch },
  { to: "/agents", label: "Agents", icon: Bot },
  { to: "/activity", label: "Activity", icon: ActivityIcon },
  { to: "/usage", label: "Usage & effort", icon: Zap },
  { to: "/configuration", label: "Configuration", icon: SlidersHorizontal },
  { to: "/settings", label: "Settings", icon: SettingsIcon },
  { to: "/login", label: "GitHub account", icon: GitBranch },
];
export default function App() {
  const [theme, setTheme] = useState(
    () => localStorage.getItem("forgeflow-theme") ?? "dark",
  );
  const [operatorOpen, setOperatorOpen] = useState(false);
  const [help, setHelp] = useState(false);
  const [menu, setMenu] = useState(false);
  const [connected, setConnected] = useState(false);
  const overview = useAPI<OverviewType>("/overview");
  const client = useQueryClient();
  const location = useLocation();
  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    localStorage.setItem("forgeflow-theme", theme);
  }, [theme]);
  useEffect(() => {
    const stream = new EventSource("/api/events/stream");
    stream.onopen = () => setConnected(true);
    stream.onerror = () => setConnected(false);
    let timer: ReturnType<typeof setTimeout> | undefined;
    const refresh = () => {
      clearTimeout(timer);
      timer = setTimeout(() => {
        void client.invalidateQueries();
      }, 150);
    };
    stream.addEventListener("activity", refresh);
    return () => {
      stream.close();
      clearTimeout(timer);
    };
  }, [client]);
  useEffect(() => {
    setMenu(false);
    setOperatorOpen(false);
  }, [location.pathname]);
  const label =
    navigation.find((n) => n.to === location.pathname)?.label ?? "Operator";
  return (
    <div className="app">
      <a className="skip-link" href="#main">
        Skip to main content
      </a>
      <aside className={"sidebar" + (menu ? " sidebar-open" : "")}>
        <div className="brand">
          <span className="brand-mark">
            <Command size={19} />
          </span>
          <strong>ForgeFlow</strong>
          <span className="version">α</span>
          <button
            className="icon-button mobile-close"
            aria-label="Close navigation"
            onClick={() => setMenu(false)}
          >
            <X size={16} />
          </button>
        </div>
        <div className="workspace-switch">
          <span className="workspace-avatar">R</span>
          <div>
            <strong>Personal workspace</strong>
            <span>Local contribution control</span>
          </div>
        </div>
        <span className="nav-label">WORKSPACE</span>
        <nav aria-label="Main navigation">
          {navigation.map((n) => (
            <NavLink end={n.to === "/"} to={n.to} key={n.to}>
              <n.icon size={17} />
              <span>{n.label}</span>
              {n.to === "/opportunities" && (
                <span className="nav-count">
                  {overview.data?.opportunities ?? "—"}
                </span>
              )}
            </NavLink>
          ))}
        </nav>
        <div className="sidebar-bottom">
          <div className="local-card">
            <span className="status-dot" />
            <div>
              <strong>Running locally</strong>
              <p>Your workspace. Your approval.</p>
            </div>
          </div>
          <button onClick={() => setHelp(true)}>
            <CircleHelp size={16} /> Getting started <ArrowUpRight size={13} />
          </button>
          <button onClick={() => setTheme(theme === "dark" ? "light" : "dark")}>
            <Moon size={16} />{" "}
            {theme === "dark" ? "Switch to light mode" : "Switch to dark mode"}
          </button>
        </div>
      </aside>
      {menu && (
        <button
          className="menu-backdrop"
          aria-label="Close navigation"
          onClick={() => setMenu(false)}
        />
      )}
      <div className="main-shell">
        <header className="topbar">
          <div>
            <button
              className="icon-button mobile-menu"
              aria-label="Open navigation"
              onClick={() => setMenu(true)}
            >
              <Menu size={19} />
            </button>
            <span className="muted">Workspace</span>
            <ChevronRight size={13} />
            <span>{label}</span>
          </div>
          <div>
            <span className={"connection " + (connected ? "connected" : "")}>
              <i />
              {connected ? "Event stream connected" : "Reconnecting events"}
            </span>
            <span className="top-divider" />
            <Button
              variant="secondary"
              size="small"
              onClick={() => setOperatorOpen(true)}
            >
              <MessageSquare size={14} /> Operator
            </Button>
          </div>
        </header>
        <div
          className={
            "mode-banner " +
            (overview.data?.mode === "demo" ? "demo-banner" : "")
          }
        >
          <span>
            <Sparkles size={14} />{" "}
            {overview.isPending
              ? "Connecting to your local control plane…"
              : overview.error
                ? "Backend unavailable. Start the Go server to connect."
                : overview.data?.mode === "demo"
                  ? "DEMO WORKSPACE · Issues, scores and contribution states are illustrative."
                  : "LIVE WORKSPACE · GitHub discovery, isolated Codex contributions and independent review. PR submission requires approval."}
          </span>
          {overview.data && (
            <Badge>Config v{overview.data.config_version}</Badge>
          )}
        </div>
        <main id="main" tabIndex={-1}>
          <Suspense
            fallback={<div className="skeleton" aria-label="Loading page" />}
          >
            <Routes>
              <Route path="/" element={<Overview />} />
              <Route
                path="/opportunities"
                element={
                  <>
                    <div className="page-intro">
                      <div>
                        <span className="eyebrow">
                          DISCOVER BEFORE YOU BUILD
                        </span>
                        <h1>Opportunities</h1>
                        <p>
                          Quality first. Inspect the score, scope and effort
                          before making a decision.
                        </p>
                      </div>
                    </div>
                    <Discovery />
                    <section className="surface">
                      <Opportunities />
                    </section>
                  </>
                }
              />
              <Route path="/contributions" element={<Contributions />} />
              <Route path="/login" element={<Login />} />
              <Route path="/agents" element={<Agents />} />
              <Route path="/activity" element={<Activity />} />
              <Route path="/usage" element={<Usage />} />
              <Route path="/configuration" element={<Configuration />} />
              <Route
                path="/settings"
                element={<Settings theme={theme} setTheme={setTheme} />}
              />
              <Route
                path="/operator"
                element={
                  <section className="surface operator-page">
                    <Operator />
                  </section>
                }
              />
              <Route
                path="*"
                element={
                  <Empty title="Page not found">
                    Use the sidebar to return to your workspace.
                  </Empty>
                }
              />
            </Routes>
          </Suspense>
        </main>
        <footer className="app-footer">
          <span>
            <ShieldCheck size={12} /> Human approval at every boundary that
            matters.
          </span>
          <span>ForgeFlow · discovery 0.2</span>
        </footer>
      </div>
      <Dialog
        open={operatorOpen}
        onOpenChange={setOperatorOpen}
        title="Operator"
        description="Controlled actions, grounded in your persisted system state."
      >
        <Operator />
      </Dialog>
      <Dialog
        open={help}
        onOpenChange={setHelp}
        title="Welcome to ForgeFlow"
        description="A local-first contribution control plane."
      >
        <p className="summary">
          ForgeFlow includes real GitHub discovery, approved Codex execution,
          real verification, independent review, reports and replayable audit events.
        </p>
        <p>
          Use Opportunities to inspect quality factors. Use Configuration to
          propose, review, apply and restore settings. Ask Operator about your
          profile or try “Add Rust”.
        </p>
        <p className="muted">
          GitHub discovery is available in live mode. Isolated execution, Codex
          sessions, real contribution tests, independent review and PR
          preparation are later milestones. Demo data is clearly labeled and
          stored separately.
        </p>
      </Dialog>
    </div>
  );
}
