import type { ReactNode } from "react";
import { LanguageSwitcher } from "../../components/LanguageSwitcher";
import { useI18n } from "../../lib/i18n";

export function AuthLayout({ title, children }: { title: string; children: ReactNode }) {
  const { t } = useI18n();
  return (
    <main className="flex min-h-screen items-center justify-center px-4 py-10">
      <div className="w-full max-w-sm space-y-6">
        <div className="space-y-2 text-center">
          <p className="text-sm font-semibold tracking-widest text-ink-muted">{t("app.name").toUpperCase()}</p>
          <h1 className="text-2xl font-bold tracking-tight text-ink">{title}</h1>
          <p className="text-sm text-ink-muted">{t("app.tagline")}</p>
        </div>
        <div className="rounded-hero bg-surface p-6 shadow-card sm:p-8">{children}</div>
        <div className="flex justify-center">
          <LanguageSwitcher />
        </div>
      </div>
    </main>
  );
}
