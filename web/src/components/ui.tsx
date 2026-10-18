import type { ButtonHTMLAttributes, HTMLAttributes, ReactNode } from "react";
import { AlertCircle, Inbox, Loader2 } from "lucide-react";
import type { ApplicationStatus } from "../lib/api";
import { buttonClass, errorMessage, type Variant } from "./styles";
import { useI18n } from "../lib/i18n";

export function Button({
  variant = "primary",
  busy = false,
  className = "",
  children,
  disabled,
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant; busy?: boolean }) {
  return (
    <button
      type="button"
      className={buttonClass(variant, className)}
      disabled={disabled || busy}
      aria-busy={busy || undefined}
      {...props}
    >
      {busy && <Loader2 aria-hidden className="size-4 animate-spin" />}
      {children}
    </button>
  );
}

export function Field({
  id,
  label,
  error,
  hint,
  children,
}: {
  id: string;
  label: string;
  error?: string;
  hint?: string;
  children: ReactNode;
}) {
  return (
    <div className="space-y-1">
      <label htmlFor={id} className="block text-sm font-medium text-ink">
        {label}
      </label>
      {children}
      {hint && !error && (
        <p id={`${id}-hint`} className="text-xs text-ink-muted">
          {hint}
        </p>
      )}
      {error && (
        <p id={`${id}-error`} role="alert" className="text-xs text-danger">
          {error}
        </p>
      )}
    </div>
  );
}

export function Spinner({ label }: { label?: string }) {
  const { t } = useI18n();
  return (
    <div role="status" className="flex items-center justify-center gap-2 py-10 text-sm text-ink-muted">
      <Loader2 aria-hidden className="size-5 animate-spin" />
      <span>{label ?? t("common.loading")}</span>
    </div>
  );
}

export function ErrorState({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  const { t } = useI18n();
  return (
    <div role="alert" className="flex flex-col items-center gap-3 rounded-lg border border-danger bg-danger-soft px-4 py-8 text-center">
      <AlertCircle aria-hidden className="size-6 text-danger" />
      <p className="text-sm text-danger">{errorMessage(error, t)}</p>
      {onRetry && (
        <Button variant="secondary" onClick={onRetry}>
          {t("common.retry")}
        </Button>
      )}
    </div>
  );
}

export function InlineError({ error }: { error: unknown }) {
  const { t } = useI18n();
  if (!error) return null;
  return (
    <p role="alert" className="text-sm text-danger">
      {errorMessage(error, t)}
    </p>
  );
}

export function EmptyState({ title, hint, children }: { title: string; hint?: string; children?: ReactNode }) {
  return (
    <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed border-line-strong bg-surface px-4 py-10 text-center">
      <Inbox aria-hidden className="size-6 text-ink-muted" />
      <p className="font-medium text-ink">{title}</p>
      {hint && <p className="text-sm text-ink-muted">{hint}</p>}
      {children}
    </div>
  );
}

const statusColors: Record<ApplicationStatus, string> = {
  planned: "bg-canvas text-ink",
  applied: "bg-lavender text-ink",
  interviewing: "bg-amber text-ink",
  offer: "bg-sage text-ink",
  accepted: "bg-sage-deep text-ink-inverse",
  rejected: "bg-danger-soft text-danger",
  withdrawn: "bg-canvas text-ink-muted",
};

export function StatusBadge({ status }: { status: ApplicationStatus }) {
  const { t } = useI18n();
  return (
    <span className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium ${statusColors[status]}`}>
      {t(`status.${status}`)}
    </span>
  );
}

export function Chip({ children }: { children: ReactNode }) {
  return <span className="inline-flex rounded-md bg-canvas px-2 py-0.5 text-xs text-ink">{children}</span>;
}

export function Card({ children, className = "", ...props }: HTMLAttributes<HTMLElement> & { children: ReactNode }) {
  return (
    <section {...props} className={`rounded-card bg-surface p-4 shadow-card sm:p-6 ${className}`}>
      {children}
    </section>
  );
}

export function PageTitle({ children }: { children: ReactNode }) {
  return <h1 className="text-2xl font-semibold tracking-tight text-ink">{children}</h1>;
}
