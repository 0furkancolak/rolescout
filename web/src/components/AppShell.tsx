import { useEffect, useState } from "react";
import {
  Bookmark,
  Briefcase,
  Building2,
  LayoutDashboard,
  ListChecks,
  LogOut,
  Menu,
  Radar,
  Settings,
  X,
} from "lucide-react";
import { NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import { useCurrentUser, useLogout } from "../features/auth/api";
import { useWorkspace } from "../features/workspaces/context";
import { useI18n } from "../lib/i18n";
import type { MessageKey } from "../lib/messages";
import { LanguageSwitcher } from "./LanguageSwitcher";
import { ThemeToggle } from "./ThemeToggle";

interface NavItem {
  to: string;
  key: MessageKey;
  icon: typeof Briefcase;
  end?: boolean;
  admin?: boolean;
}

const navItems: NavItem[] = [
  { to: "/", key: "nav.dashboard", icon: LayoutDashboard, end: true },
  { to: "/research", key: "nav.research", icon: Radar },
  { to: "/jobs", key: "nav.jobs", icon: Briefcase },
  { to: "/saved", key: "nav.saved", icon: Bookmark },
  { to: "/applications", key: "nav.applications", icon: ListChecks },
  { to: "/companies", key: "nav.companies", icon: Building2 },
  { to: "/settings", key: "nav.settings", icon: Settings, admin: true },
];

// AppShell is the signed-in layout: a sidebar on wide screens and a top bar
// with a menu on narrow ones.
export function AppShell() {
  const { t } = useI18n();
  const me = useCurrentUser();
  const [menuOpen, setMenuOpen] = useState(false);
  const location = useLocation();

  // Close the mobile menu after navigating.
  const [lastPath, setLastPath] = useState(location.pathname);
  if (lastPath !== location.pathname) {
    setLastPath(location.pathname);
    setMenuOpen(false);
  }
  useEffect(() => {
    if (!menuOpen) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setMenuOpen(false);
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [menuOpen]);

  const items = navItems.filter((i) => !i.admin || me.data?.is_admin);

  return (
    <div className="min-h-screen bg-slate-50 text-slate-900">
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:absolute focus:left-2 focus:top-2 focus:z-50 focus:rounded focus:bg-white focus:px-3 focus:py-2"
      >
        {t("nav.skip")}
      </a>

      <header className="sticky top-0 z-30 flex items-center gap-2 border-b border-slate-200 bg-white/90 px-3 py-2 backdrop-blur lg:hidden">
        <button
          type="button"
          aria-expanded={menuOpen}
          aria-controls="sidebar"
          aria-label={menuOpen ? t("nav.closeMenu") : t("nav.openMenu")}
          onClick={() => setMenuOpen((o) => !o)}
          className="inline-flex size-10 items-center justify-center rounded-lg text-slate-700 hover:bg-slate-100"
        >
          {menuOpen ? <X aria-hidden className="size-5" /> : <Menu aria-hidden className="size-5" />}
        </button>
        <Brand />
        <div className="ml-auto">
          <ThemeToggle />
        </div>
      </header>

      {menuOpen && (
        <div aria-hidden className="fixed inset-0 z-30 bg-slate-900/40 lg:hidden" onClick={() => setMenuOpen(false)} />
      )}

      <aside
        id="sidebar"
        className={`fixed inset-y-0 left-0 z-40 flex w-64 flex-col border-r border-slate-200 bg-white transition-transform lg:translate-x-0 ${
          menuOpen ? "translate-x-0" : "-translate-x-full"
        }`}
      >
        <div className="hidden px-5 pb-2 pt-5 lg:block">
          <Brand />
        </div>
        <nav aria-label={t("nav.main")} className="flex-1 overflow-y-auto px-3 py-3">
          <ul className="space-y-1">
            {items.map(({ to, key, icon: Icon, end }) => (
              <li key={to}>
                <NavLink
                  to={to}
                  end={end}
                  className={({ isActive }) =>
                    `flex min-h-10 items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors ${
                      isActive ? "bg-blue-50 text-blue-800" : "text-slate-600 hover:bg-slate-100 hover:text-slate-900"
                    }`
                  }
                >
                  <Icon aria-hidden className="size-4.5" />
                  {t(key)}
                </NavLink>
              </li>
            ))}
          </ul>
        </nav>
        <SidebarFooter />
      </aside>

      <main id="main" className="px-4 py-6 sm:px-6 lg:ml-64 lg:px-8 lg:py-8">
        <Outlet />
      </main>
    </div>
  );
}

function Brand() {
  const { t } = useI18n();
  return (
    <NavLink to="/" className="flex items-center gap-2 text-lg font-semibold tracking-tight text-slate-900">
      <span aria-hidden className="grid size-8 place-items-center rounded-lg bg-blue-700 text-white">
        <Radar className="size-4.5" />
      </span>
      {t("app.name")}
    </NavLink>
  );
}

function SidebarFooter() {
  const { t } = useI18n();
  const me = useCurrentUser();
  const logout = useLogout();
  const navigate = useNavigate();
  const { workspace, workspaces, select } = useWorkspace();
  return (
    <div className="space-y-3 border-t border-slate-200 p-4">
      {workspaces.length > 1 && (
        <label className="block text-xs text-slate-500">
          {t("nav.workspace")}
          <select
            value={workspace.id}
            onChange={(e) => select(e.target.value)}
            className="mt-1 block w-full rounded-md border border-slate-300 bg-white px-2 py-1.5 text-sm text-slate-900"
          >
            {workspaces.map((w) => (
              <option key={w.id} value={w.id}>
                {w.name}
              </option>
            ))}
          </select>
        </label>
      )}
      <div className="flex items-center gap-2">
        <LanguageSwitcher />
        <ThemeToggle className="hidden lg:inline-flex" />
      </div>
      <div className="flex items-center justify-between gap-2">
        <span className="truncate text-sm text-slate-600">{me.data?.display_name}</span>
        <button
          type="button"
          onClick={() => logout.mutate(undefined, { onSettled: () => navigate("/login", { replace: true }) })}
          className="inline-flex min-h-9 items-center gap-1 rounded-md px-2 text-sm text-slate-600 hover:bg-slate-100"
        >
          <LogOut aria-hidden className="size-4" />
          <span>{t("nav.signOut")}</span>
        </button>
      </div>
    </div>
  );
}
