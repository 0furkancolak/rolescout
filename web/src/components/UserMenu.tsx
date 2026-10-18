import { useEffect, useRef, useState } from "react";
import { ChevronUp, LogOut } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { useCurrentUser, useLogout } from "../features/auth/api";
import { useWorkspace } from "../features/workspaces/context";
import { useI18n } from "../lib/i18n";
import { CompanyTile } from "./CompanyTile";
import { LanguageSwitcher } from "./LanguageSwitcher";
import { ThemeToggle } from "./ThemeToggle";
import { inputClass } from "./styles";

// The account menu: workspace, language, theme and sign out. A disclosure
// popover: Escape or an outside click closes it and focus returns to the
// button.
export function UserMenu() {
  const { t } = useI18n();
  const me = useCurrentUser();
  const logout = useLogout();
  const navigate = useNavigate();
  const { workspace, workspaces, select } = useWorkspace();
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const name = me.data?.display_name ?? "";

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      e.stopPropagation();
      setOpen(false);
      trigger.current?.focus();
    };
    const onDown = (e: MouseEvent) => {
      if (root.current && !root.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("keydown", onKey);
    document.addEventListener("mousedown", onDown);
    return () => {
      document.removeEventListener("keydown", onKey);
      document.removeEventListener("mousedown", onDown);
    };
  }, [open]);

  return (
    <div ref={root} className="relative">
      {open && (
        <div id="account-menu" className="absolute inset-x-0 bottom-full mb-2 space-y-3 rounded-card bg-raised p-4 shadow-raised">
          {workspaces.length > 1 && (
            <label className="block text-xs text-ink-muted">
              {t("nav.workspace")}
              <select value={workspace.id} onChange={(e) => select(e.target.value)} className={`${inputClass} mt-1`}>
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
            <ThemeToggle />
          </div>
          <button
            type="button"
            onClick={() => logout.mutate(undefined, { onSettled: () => navigate("/login", { replace: true }) })}
            className="inline-flex min-h-10 w-full items-center gap-2 rounded-control px-2 text-sm text-ink hover:bg-canvas focus-visible:outline-2 focus-visible:outline-focus"
          >
            <LogOut aria-hidden className="size-4" />
            {t("nav.signOut")}
          </button>
        </div>
      )}
      <button
        ref={trigger}
        type="button"
        aria-expanded={open}
        aria-controls={open ? "account-menu" : undefined}
        aria-label={`${t("nav.account")}: ${name}`}
        onClick={() => setOpen((o) => !o)}
        className="flex min-h-12 w-full items-center gap-3 rounded-control px-2 py-1.5 text-left hover:bg-canvas focus-visible:outline-2 focus-visible:outline-focus"
      >
        <CompanyTile name={name || "?"} size={32} />
        <span className="min-w-0 flex-1 truncate text-sm font-medium text-ink">{name}</span>
        <ChevronUp aria-hidden className={`size-4 text-ink-muted transition-transform ${open ? "" : "rotate-180"}`} />
      </button>
    </div>
  );
}
