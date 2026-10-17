import { Briefcase, Bookmark, ListChecks, LogOut } from "lucide-react";
import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { useCurrentUser, useLogout } from "../features/auth/api";
import { useWorkspace } from "../features/workspaces/context";
import { useI18n } from "../lib/i18n";
import { LanguageSwitcher } from "./LanguageSwitcher";

const navItems = [
  { to: "/jobs", key: "nav.jobs", icon: Briefcase },
  { to: "/saved", key: "nav.saved", icon: Bookmark },
  { to: "/applications", key: "nav.applications", icon: ListChecks },
] as const;

export function Layout() {
  const { t } = useI18n();
  const me = useCurrentUser();
  const logout = useLogout();
  const navigate = useNavigate();
  const { workspace, workspaces, select } = useWorkspace();

  return (
    <div className="min-h-screen">
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:absolute focus:left-2 focus:top-2 focus:z-50 focus:rounded focus:bg-white focus:px-3 focus:py-2"
      >
        {t("nav.skip")}
      </a>
      <header className="border-b border-slate-200 bg-white">
        <div className="mx-auto flex max-w-6xl flex-wrap items-center gap-x-6 gap-y-2 px-4 py-3">
          <NavLink to="/jobs" className="text-lg font-semibold tracking-tight text-slate-900">
            {t("app.name")}
          </NavLink>
          <nav aria-label={t("nav.main")} className="order-last w-full overflow-x-auto sm:order-none sm:w-auto">
            <ul className="flex gap-1">
              {navItems.map(({ to, key, icon: Icon }) => (
                <li key={to}>
                  <NavLink
                    to={to}
                    className={({ isActive }) =>
                      `flex min-h-10 items-center gap-2 whitespace-nowrap rounded-md px-3 py-2 text-sm font-medium ${
                        isActive ? "bg-blue-50 text-blue-800" : "text-slate-600 hover:bg-slate-100"
                      }`
                    }
                  >
                    <Icon aria-hidden className="size-4" />
                    {t(key)}
                  </NavLink>
                </li>
              ))}
            </ul>
          </nav>
          <div className="ml-auto flex items-center gap-3">
            {workspaces.length > 1 && (
              <label className="text-sm">
                <span className="sr-only">{t("nav.workspace")}</span>
                <select
                  value={workspace.id}
                  onChange={(e) => select(e.target.value)}
                  className="max-w-40 rounded-md border border-slate-300 bg-white px-2 py-1 text-sm"
                >
                  {workspaces.map((w) => (
                    <option key={w.id} value={w.id}>
                      {w.name}
                    </option>
                  ))}
                </select>
              </label>
            )}
            <LanguageSwitcher />
            <span className="hidden text-sm text-slate-600 md:inline">{me.data?.display_name}</span>
            <button
              type="button"
              onClick={() => logout.mutate(undefined, { onSettled: () => navigate("/login", { replace: true }) })}
              className="inline-flex min-h-10 items-center gap-1 rounded-md px-2 text-sm text-slate-600 hover:bg-slate-100"
            >
              <LogOut aria-hidden className="size-4" />
              <span>{t("nav.signOut")}</span>
            </button>
          </div>
        </div>
      </header>
      <main id="main" className="mx-auto max-w-6xl px-4 py-6 sm:py-8">
        <Outlet />
      </main>
    </div>
  );
}
