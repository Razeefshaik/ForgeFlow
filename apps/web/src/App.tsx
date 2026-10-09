import { lazy, Suspense, useEffect, useRef, useState } from "react";
import {
  NavLink,
  Route,
  Routes,
  useLocation,
  useNavigate,
} from "react-router-dom";
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
  Search,
  X,
  ChevronRight,
  Zap,
} from "./components/icons";
import { useAPI } from "./api";
import type { Overview as OverviewType } from "./types";
import Overview from "./pages/Overview";
import { Badge, Empty } from "./components/primitives";
import { Button } from "./components/ui/button";
import { Dialog } from "./components/ui/dialog";
import { Tooltip } from "./components/ui/tooltip";
import { usePresence } from "./components/ui/presence";
const Operator = lazy(() => import("./components/Operator"));
const Discovery = lazy(() => import("./components/Discovery"));
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
    () => localStorage.getItem("forgeflow-theme") ?? "dark",
  );
  const [operatorOpen, setOperatorOpen] = useState(false);
  const [resolvedTheme, setResolvedTheme] = useState(() =>
    localStorage.getItem("forgeflow-theme") === "light" ? "light" : "dark",
  );
  const [help, setHelp] = useState(false);
  const [menu, setMenu] = useState(false);
  const menuPresent = usePresence(menu, 140);
  const [navCollapsed, setNavCollapsed] = useState(
    () => localStorage.getItem("forgeflow-sidebar-collapsed") === "true",
  );
  const [connected, setConnected] = useState(false);
  const [searchOpen, setSearchOpen] = useState(false);
  const [navigationSearch, setNavigationSearch] = useState("");
  const [compact, setCompact] = useState(
    () => window.matchMedia("(max-width: 767px)").matches,
  );
  const sidebarRef = useRef<HTMLElement>(null);
  const menuTriggerRef = useRef<HTMLButtonElement>(null);
  const overview = useAPI<OverviewType>("/overview");
  const account = useAPI<{ connected: boolean; login: string }>("/auth/github");
  const client = useQueryClient();
  const location = useLocation();
  const navigate = useNavigate();
  useEffect(() => {
    if (!compact || !menu) return;
    const previous = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = previous;
    };
  }, [compact, menu]);
  const page =
    location.pathname === "/"
      ? "overview"
      : location.pathname.startsWith("/contributions/")
        ? "workspace"
        : location.pathname.split("/")[1];
  useEffect(() => {
    const media = window.matchMedia("(max-width: 767px)");
    const update = () => setCompact(media.matches);
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);
  useEffect(() => {
    const handle = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        setSearchOpen((value) => !value);
      }
    };
    document.addEventListener("keydown", handle);
    return () => document.removeEventListener("keydown", handle);
  }, []);
  useEffect(() => {
    if (!menu || !compact) return;
    const sidebar = sidebarRef.current;
    sidebar?.querySelector<HTMLElement>(".mobile-close")?.focus();
    const handle = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setMenu(false);
        menuTriggerRef.current?.focus();
      }
      if (event.key !== "Tab" || !sidebar) return;
      const elements = [
        ...sidebar.querySelectorAll<HTMLElement>("a, button"),
      ].filter((el) => el.getClientRects().length);
      const first = elements[0],
        last = elements[elements.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last?.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first?.focus();
      }
    };
    document.addEventListener("keydown", handle);
    return () => {
      document.removeEventListener("keydown", handle);
      queueMicrotask(() => {
        if (window.matchMedia("(max-width: 767px)").matches)
          menuTriggerRef.current?.focus();
      });
    };
  }, [menu, compact]);
  useEffect(() => {
    const keyboard = () => {
      document.documentElement.dataset.input = "keyboard";
    };
    const pointer = () => {
      document.documentElement.dataset.input = "pointer";
    };
    // Resolve input modality before Radix or React handles the same event.
    // Otherwise Escape can cancel an exit after Presence starts waiting for it.
    document.addEventListener("keydown", keyboard, true);
    document.addEventListener("pointerdown", pointer, true);
    return () => {
      document.removeEventListener("keydown", keyboard, true);
      document.removeEventListener("pointerdown", pointer, true);
    };
  }, []);
  useEffect(() => {
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const apply = () => {
      const resolved =
        theme === "system" ? (media.matches ? "dark" : "light") : theme;
      document.documentElement.dataset.theme = resolved;
      setResolvedTheme(resolved);
      document
        .querySelector('meta[name="theme-color"]')
        ?.setAttribute("content", resolved === "dark" ? "#101014" : "#f5f5fa");
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
    <div className="app" data-page={page} data-nav-collapsed={navCollapsed}>
      <a className="skip-link" href="#main">
        Skip to main content
      </a>
      <aside
        id="navigation-panel"
        ref={sidebarRef}
        inert={compact && !menu ? true : undefined}
        className={"sidebar" + (menu ? " sidebar-open" : "")}
      >
        <div className="brand">
          <span className="brand-mark">
            <Command size={19} />
          </span>
          <strong>ForgeFlow</strong>
          <span className="version">LOCAL</span>
          <button
            className="icon-button mobile-close"
            aria-label="Close navigation"
            onClick={() => {
              setMenu(false);
              menuTriggerRef.current?.focus();
            }}
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
                <Tooltip
                  key={n.to}
                  content={n.label}
                  side="right"
                  disabled={!navCollapsed || compact}
                >
                  <NavLink
                    aria-label={n.label}
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
                </Tooltip>
              ))}
            </div>
          ))}
        </nav>
        <div className="sidebar-bottom">
          <Tooltip
            content={navCollapsed ? "Expand navigation" : "Collapse navigation"}
            side={navCollapsed ? "right" : "top"}
          >
            <button
              className="sidebar-toggle"
              aria-label={
                navCollapsed ? "Expand navigation" : "Collapse navigation"
              }
              aria-expanded={!navCollapsed}
              aria-controls="navigation-panel"
              onClick={() =>
                setNavCollapsed((collapsed) => {
                  localStorage.setItem(
                    "forgeflow-sidebar-collapsed",
                    String(!collapsed),
                  );
                  return !collapsed;
                })
              }
            >
              <ChevronRight size={18} className="nav-toggle-icon" />
              <span>
                {navCollapsed ? "Expand navigation" : "Collapse navigation"}
              </span>
            </button>
          </Tooltip>
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
          <Tooltip
            content={
              account.data?.connected
                ? `GitHub: ${account.data.login || "connected"}`
                : "Connect GitHub"
            }
            disabled={!navCollapsed || compact}
            side="right"
          >
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
          </Tooltip>
          <Tooltip
            content="Getting started"
            side="right"
            disabled={!navCollapsed || compact}
          >
            <button
              onClick={() => {
                setMenu(false);
                setHelp(true);
              }}
            >
              <CircleHelp size={16} /> <span>Getting started</span>{" "}
              <ArrowUpRight size={13} />
            </button>
          </Tooltip>
          <Tooltip
            content={
              resolvedTheme === "dark"
                ? "Switch to light mode"
                : "Switch to dark mode"
            }
            side="right"
            disabled={!navCollapsed || compact}
          >
            <button
              onClick={() =>
                setTheme(resolvedTheme === "dark" ? "light" : "dark")
              }
            >
              {resolvedTheme === "dark" ? (
                <Sun size={16} />
              ) : (
                <Moon size={16} />
              )}{" "}
              <span>
                {resolvedTheme === "dark"
                  ? "Switch to light mode"
                  : "Switch to dark mode"}
              </span>
            </button>
          </Tooltip>
        </div>
      </aside>
      {menuPresent && (
        <button
          className="menu-backdrop"
          data-state={menu ? "open" : "closed"}
          aria-hidden={!menu}
          inert={!menu ? true : undefined}
          aria-label="Close navigation"
          onClick={() => setMenu(false)}
        />
      )}
      <div className="main-shell" inert={compact && menu ? true : undefined}>
        <header className="topbar">
          <div>
            <button
              ref={menuTriggerRef}
              className="icon-button mobile-menu"
              aria-label="Open navigation"
              onClick={() => setMenu(true)}
            >
              <Menu size={19} />
            </button>
            <span className="toolbar-breadcrumb">
              Workspace <span>/</span>
            </span>
            <strong className="toolbar-title">{label}</strong>
          </div>
          <div>
            <button
              className="command-search"
              aria-label="Open command search"
              onClick={() => {
                setNavigationSearch("");
                setSearchOpen(true);
              }}
            >
              <Search size={16} />
              <span>Jump to a page</span>
              <kbd>Ctrl K</kbd>
            </button>
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
                          <h1>Opportunities</h1>
                          <p>
                            Quality first. Inspect the score, scope and effort
                            before making a decision.
                          </p>
                        </div>
                      </div>
                      <Suspense
                        fallback={
                          <div
                            className="skeleton"
                            aria-label="Loading discovery controls"
                          />
                        }
                      >
                        <Discovery />
                      </Suspense>
                      <section className="opportunity-surface">
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
                    <>
                      <div className="page-intro">
                        <div>
                          <h1>Operator</h1>
                          <p>
                            Ask about your workspace. Review every proposed
                            action.
                          </p>
                        </div>
                      </div>
                      <section className="surface surface-elevated operator-page">
                        <Suspense
                          fallback={
                            <div
                              className="skeleton"
                              aria-label="Loading Operator"
                            />
                          }
                        >
                          <Operator />
                        </Suspense>
                      </section>
                    </>
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
        open={searchOpen}
        instant
        onOpenChange={setSearchOpen}
        title="Jump to a page"
        description="Navigate your local contribution control plane."
      >
        <label className="search command-input">
          <Search size={18} />
          <input
            data-autofocus
            aria-label="Search pages"
            placeholder="Search pages…"
            value={navigationSearch}
            onChange={(event) => setNavigationSearch(event.target.value)}
          />
        </label>
        <div className="command-results">
          {[
            ...navigation,
            { to: "/operator", label: "Operator", icon: MessageSquare },
          ]
            .filter((item) =>
              item.label.toLowerCase().includes(navigationSearch.toLowerCase()),
            )
            .map((item) => (
              <button
                key={item.to}
                onClick={() => {
                  setSearchOpen(false);
                  navigate(item.to);
                }}
              >
                <item.icon size={19} />
                <span>{item.label}</span>
                <ArrowUpRight size={14} />
              </button>
            ))}
          {![...navigation, { label: "Operator" }].some((item) =>
            item.label.toLowerCase().includes(navigationSearch.toLowerCase()),
          ) && <p className="muted">No matching pages.</p>}
        </div>
      </Dialog>
      <Dialog
        open={operatorOpen}
        onOpenChange={setOperatorOpen}
        drawer
        title="Operator"
        description="Controlled actions, grounded in your persisted system state."
      >
        <Suspense
          fallback={<div className="skeleton" aria-label="Loading Operator" />}
        >
          <Operator />
        </Suspense>
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
