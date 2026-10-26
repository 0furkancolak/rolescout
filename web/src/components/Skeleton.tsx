import { useI18n } from "../lib/i18n";

// Skeletons stand in for content that is still loading. They keep the final
// layout's shape so the page does not jump when the data arrives.

export function Skeleton({ className = "" }: { className?: string }) {
  return <div aria-hidden className={`animate-pulse rounded-md bg-line/70 motion-reduce:animate-none ${className}`} />;
}

// Loading wraps skeleton blocks in one polite status for screen readers.
function Loading({ className = "", children }: { className?: string; children: React.ReactNode }) {
  const { t } = useI18n();
  return (
    <div role="status" aria-busy="true" className={className}>
      <span className="sr-only">{t("common.loading")}</span>
      {children}
    </div>
  );
}

// SkeletonRows is a stack of text-like rows: a list, a table or a form.
export function SkeletonRows({ rows = 3, className = "" }: { rows?: number; className?: string }) {
  return (
    <Loading className={`mt-4 space-y-3 ${className}`}>
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="space-y-2">
          <Skeleton className="h-4 w-2/5" />
          <Skeleton className="h-3 w-4/5" />
        </div>
      ))}
    </Loading>
  );
}

// SkeletonCards is a grid of cards, each with a title, a line and some chips.
export function SkeletonCards({ count = 4, className = "" }: { count?: number; className?: string }) {
  return (
    <Loading className={`grid gap-3 ${className}`}>
      {Array.from({ length: count }, (_, i) => (
        <div key={i} className="space-y-3 rounded-card bg-surface p-5 shadow-card">
          <div className="flex items-center gap-3">
            <Skeleton className="size-10 shrink-0 rounded-full" />
            <div className="flex-1 space-y-2">
              <Skeleton className="h-4 w-3/5" />
              <Skeleton className="h-3 w-2/5" />
            </div>
          </div>
          <Skeleton className="h-3 w-full" />
          <div className="flex gap-2">
            <Skeleton className="h-5 w-16 rounded-full" />
            <Skeleton className="h-5 w-12 rounded-full" />
            <Skeleton className="h-5 w-20 rounded-full" />
          </div>
        </div>
      ))}
    </Loading>
  );
}

// PageSkeleton is a whole page: a title and a few cards.
export function PageSkeleton({ cards = 3 }: { cards?: number }) {
  return (
    <Loading className="space-y-6">
      <Skeleton className="h-8 w-56" />
      {Array.from({ length: cards }, (_, i) => (
        <div key={i} className="space-y-3 rounded-card bg-surface p-5 shadow-card">
          <Skeleton className="h-5 w-1/3" />
          <Skeleton className="h-3 w-full" />
          <Skeleton className="h-3 w-4/5" />
          <Skeleton className="h-3 w-3/5" />
        </div>
      ))}
    </Loading>
  );
}

// ShellSkeleton is shown while the session and workspaces load, before the
// real layout can render. It mirrors the sidebar and content areas.
export function ShellSkeleton() {
  return (
    <div className="min-h-screen bg-canvas">
      <div aria-hidden className="fixed inset-y-4 left-4 hidden w-64 space-y-3 rounded-card bg-surface p-5 shadow-card lg:block">
        <Skeleton className="h-6 w-28" />
        <div className="space-y-2 pt-4">
          {Array.from({ length: 8 }, (_, i) => (
            <Skeleton key={i} className="h-10 w-full rounded-full" />
          ))}
        </div>
      </div>
      <div className="px-4 py-6 sm:px-6 lg:ml-72 lg:px-8 lg:py-8">
        <PageSkeleton />
      </div>
    </div>
  );
}
