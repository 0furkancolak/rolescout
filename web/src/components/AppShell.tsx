import { useEffect, useRef, useState } from "react";
import {
  Bookmark,
  Briefcase,
  Building2,
  LayoutDashboard,
  ListChecks,
  Menu,
  Radar,
  Settings,
  X,
} from "lucide-react";
import { NavLink, Outlet, useLocation } from "react-router-dom";
import { useCurrentUser } from "../features/auth/api";
import { useI18n } from "../lib/i18n";
import type { MessageKey } from "../lib/messages";
import { UserMenu } from "./UserMenu";

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
  const items = navItems.filter((i) => !i.admin || me.data?.is_admin);

  const toggleRef = useRef<HTMLButtonElement>(null);
  const sheetRef = useRef<HTMLElement>(null);

  // Keep Tab inside the open sheet and give focus back to its button on close.
  useEffect(() => {
    if (!menuOpen) return;
    const first = sheetRef.current?.querySelector<HTMLElement>("a, button, select");
    first?.focus();
    const onTab = (e: KeyboardEvent) => {
      if (e.key !== "Tab" || !sheetRef.current) return;
      const items = [...sheetRef.current.querySelectorAll<HTMLElement>("a, button, select")];
      if (!items.length) return;
      const [head, tail] = [items[0], items[items.length - 1]];
      if (e.shiftKey && document.activeElement === head) {
        e.preventDefault();
        tail.focus();
      } else if (!e.shiftKey && document.activeElement === tail) {
        e.preventDefault();
        head.focus();
      }
    };
    document.addEventListener("keydown", onTab);
    return () => document.removeEventListener("keydown", onTab);
  }, [menuOpen]);

  const closeMenu = () => {
    setMenuOpen(false);
    toggleRef.current?.focus();
  };

  return (
    <div className="min-h-screen bg-canvas text-ink">
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:absolute focus:left-2 focus:top-2 focus:z-50 focus:rounded-control focus:bg-surface focus:px-3 focus:py-2 focus:shadow-raised"
      >
        {t("nav.skip")}
      </a>

      <header className="sticky top-0 z-30 flex items-center gap-2 bg-canvas/90 px-4 py-2 backdrop-blur lg:hidden">
        <button
          ref={toggleRef}
          type="button"
          aria-expanded={menuOpen}
          aria-controls="sidebar"
          aria-label={menuOpen ? t("nav.closeMenu") : t("nav.openMenu")}
          onClick={() => (menuOpen ? closeMenu() : setMenuOpen(true))}
          className="inline-flex size-11 items-center justify-center rounded-control text-ink hover:bg-surface focus-visible:outline-2 focus-visible:outline-focus"
        >
          {menuOpen ? <X aria-hidden className="size-5" /> : <Menu aria-hidden className="size-5" />}
        </button>
        <Brand />
      </header>

      {menuOpen && <div aria-hidden className="fixed inset-0 z-30 bg-ink/40 lg:hidden" onClick={closeMenu} />}

      <aside
        ref={sheetRef}
        id="sidebar"
        onKeyDown={(e) => {
          if (e.key === "Escape" && menuOpen) {
            e.stopPropagation();
            closeMenu();
          }
        }}
        className={`fixed inset-y-0 left-0 z-40 flex w-72 max-w-[85vw] flex-col bg-surface shadow-raised transition-transform motion-reduce:transition-none lg:inset-y-4 lg:left-4 lg:w-64 lg:max-w-none lg:translate-x-0 lg:rounded-card lg:shadow-card ${
          menuOpen ? "translate-x-0" : "-translate-x-full"
        }`}
      >
        <div className="hidden px-5 pb-2 pt-6 lg:block">
          <Brand />
        </div>
        <nav aria-label={t("nav.main")} className="flex-1 overflow-y-auto px-3 py-4">
          <ul className="space-y-1">
            {items.map(({ to, key, icon: Icon, end }) => (
              <li key={to}>
                <NavLink
                  to={to}
                  end={end}
                  className={({ isActive }) =>
                    `flex min-h-11 items-center gap-3 rounded-full px-4 py-2 text-sm font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus ${
                      isActive ? "bg-ink text-ink-inverse" : "text-ink-muted hover:bg-canvas hover:text-ink"
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
        <div className="p-3">
          <UserMenu />
        </div>
      </aside>

      <main id="main" className="px-4 py-6 sm:px-6 lg:ml-72 lg:px-8 lg:py-8">
        <Outlet />
      </main>
    </div>
  );
}

function Brand() {
  const { t } = useI18n();
  return (
    <NavLink to="/" className="flex items-center gap-2 text-lg font-semibold tracking-tight text-ink">
      <span aria-hidden className="grid size-8 place-items-center rounded-control bg-ink text-ink-inverse">
        <Radar className="size-4.5" />
      </span>
      {t("app.name")}
    </NavLink>
  );
}
