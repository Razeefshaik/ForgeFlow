import { lazy, Suspense, useEffect, useState } from "react";
import { NavLink, Route, Routes, useLocation } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import {
  Activity as ActivityIcon,
  ArrowUpRight,
  Bot,
  CircleHelp,
  Command,
  GitBranch,
  Github,
  LayoutDashboard,
  Menu,
  MessageSquare,
  Moon,
  Sun,
  Radar,
  Settings as SettingsIcon,
  ShieldCheck,
  SlidersHorizontal,
  Sparkles,
  X,
  Zap,
} from "./components/icons";
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
const Contributions = lazy(() => import("./pages/Contributions"));
const ContributionPage = lazy(() =>
  import("./pages/Contributions").then((m) => ({
    default: m.ContributionPage,
  })),
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
  { to: "/login", label: "GitHub account", icon: Github },
];
export default function App() {
  const [theme, setTheme] = useState(
    () => localStorage.getItem("forgeflow-theme") ?? "system",
  );
  const [operatorOpen, setOperatorOpen] = useState(false);
  const [help, setHelp] = useState(false);
  const [menu, setMenu] = useState(false);
  const [connected, setConnected] = useState(false);
  const overview = useAPI<OverviewType>("/overview");
  const account = useAPI<{ connected: boolean; login: string }>("/auth/github");
  const client = useQueryClient();
  const location = useLocation();
  useEffect(() => {
    const keyboard = () => {
      document.documentElement.dataset.input = "keyboard";
    };
    const pointer = () => {
      document.documentElement.dataset.input = "pointer";
    };
    document.addEventListener("keydown", keyboard);
    document.addEventListener("pointerdown", pointer);
    return () => {
      document.removeEventListener("keydown", keyboard);
      document.removeEventListener("pointerdown", pointer);
    };
  }, []);
  useEffect(() => {
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const apply = () => {
      const resolved =
        theme === "system" ? (media.matches ? "dark" : "light") : theme;
      document.documentElement.dataset.theme = resolved;
      document
        .querySelector('meta[name="theme-color"]')
        ?.setAttribute("content", resolved === "dark" ? "#0D1F2D" : "#f8f9fc");
    };
    apply();
    localStorage.setItem("forgeflow-theme", theme);
    media.addEventListener("change", apply);
    return () => media.removeEventListener("change", apply);
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
    navigation.find(
      (n) =>
        n.to === location.pathname ||
        (n.to !== "/" && location.pathname.startsWith(n.to + "/")),
    )?.label ?? "Operator";
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
          <span className="version">LOCAL</span>
          <button
            className="icon-button mobile-close"
            aria-label="Close navigation"
            onClick={() => setMenu(false)}
          >
            <X size={16} />
          </button>
        </div>
        <div className="workspace-switch">
          <span className="workspace-avatar">
            <GitBranch size={16} />
          </span>
          <div>
            <strong>Personal workspace</strong>
            <span>Local contribution control</span>
          </div>
        </div>
        <nav aria-label="Main navigation">
          {[
            { label: "Workspace", items: navigation.slice(0, 4) },
            { label: "Insights", items: navigation.slice(4, 6) },
            { label: "Preferences", items: navigation.slice(6) },
          ].map((group) => (
            <div className="nav-group" key={group.label}>
              <span className="nav-label">{group.label}</span>
              {group.items.map((n) => (
                <NavLink
                  end={n.to === "/"}
                  to={n.to}
                  key={n.to}
                  onClick={() => setMenu(false)}
                >
                  <n.icon size={20} />
                  <span>{n.label}</span>
                  {n.to === "/opportunities" && (
                    <span className="nav-count">
                      {overview.data?.opportunities ?? "—"}
                    </span>
                  )}
                </NavLink>
              ))}
            </div>
          ))}
        </nav>
        <div className="sidebar-bottom">
          <div className="local-card">
            <span
              className={"status-dot" + (connected ? "" : " status-offline")}
            />
            <div>
              <strong>
                {connected ? "Connected locally" : "Connection interrupted"}
              </strong>
              <p>App connection</p>
            </div>
          </div>
          <NavLink to="/login" className="account-link">
            <Github size={18} />
            <span>
              {account.data?.connected
                ? account.data.login || "GitHub connected"
                : "Connect GitHub"}
              <small>GitHub connection</small>
            </span>
            <ArrowUpRight size={14} />
          </NavLink>
          <button onClick={() => setHelp(true)}>
            <CircleHelp size={16} /> Getting started <ArrowUpRight size={13} />
          </button>
          <button onClick={() => setTheme(theme === "dark" ? "light" : "dark")}>
            {theme === "dark" ? <Sun size={16} /> : <Moon size={16} />}{" "}
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
            <strong className="toolbar-title">{label}</strong>
          </div>
          <div>
            <span className={"connection " + (connected ? "connected" : "")}>
              <i />
              {connected ? "App online" : "Reconnecting events"}
            </span>
            <NavLink to="/login" className="toolbar-account">
              <Github size={17} />
              {account.data?.connected
                ? account.data.login || "GitHub connected"
                : "Connect GitHub"}
            </NavLink>
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
                  : "LIVE WORKSPACE · Your contributions. Your approval."}
          </span>
          {overview.data && (
            <Badge>Config v{overview.data.config_version}</Badge>
          )}
        </div>
        <main id="main" tabIndex={-1}>
          <div className="page-transition" key={location.pathname}>
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
                      <section className="surface opportunity-surface">
                        <Opportunities />
                      </section>
                    </>
                  }
                />
                <Route path="/contributions" element={<Contributions />} />
                <Route
                  path="/contributions/:id"
                  element={<ContributionPage />}
                />
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
          </div>
        </main>
        <footer className="app-footer">
          <span>
            <ShieldCheck size={12} /> Human approval at every boundary that
            matters.
          </span>
          <span>ForgeFlow · Local workspace</span>
        </footer>
      </div>
      <Dialog
        open={operatorOpen}
        onOpenChange={setOperatorOpen}
        drawer
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
          real verification, independent review, reports and replayable audit
          events.
        </p>
        <p>
          Use Opportunities to inspect quality factors. Use Configuration to
          propose, review, apply and restore settings. Ask Operator about your
          profile or try “Add Rust”.
        </p>
        <p className="muted">
          Connect GitHub to discover issues. Each approved contribution has its
          own workspace, real test evidence and independent review. PR
          submission requires your approval. Demo data is clearly labeled and
          stored separately.
        </p>
      </Dialog>
    </div>
  );
}
