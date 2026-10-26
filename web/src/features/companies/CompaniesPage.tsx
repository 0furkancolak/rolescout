import { CompanyTile } from "../../components/CompanyTile";
import { Plus, X } from "lucide-react";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { Button, EmptyState, ErrorState, Field, InlineError, PageTitle } from "../../components/ui";
import { inputClass } from "../../components/styles";
import type { Company, CompanyStatus } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { atsLabel, hostOf } from "../../lib/labels";
import { useCurrentUser } from "../auth/api";
import { useWorkspace } from "../workspaces/context";
import { useAddCompany, useBlockCompany, useCompanies, useDisableCompany, type CompanyFilters } from "./api";
import { SkeletonCards } from "../../components/Skeleton";

const statuses: CompanyStatus[] = ["pending", "active", "empty", "unreachable", "blocked_by_robots", "disabled"];
const atsList = ["greenhouse", "lever", "ashby", "workable", "recruitee", "personio", "bamboohr", "workday", "teamtailor", "breezy", "rippling", "gem", "htmlpage", "jsonld", "unknown"];

const statusColor: Record<CompanyStatus, string> = {
  pending: "bg-canvas text-ink",
  active: "bg-sage text-ink",
  empty: "bg-canvas text-ink",
  unreachable: "bg-danger-soft text-danger",
  blocked_by_robots: "bg-amber text-ink",
  disabled: "bg-canvas text-ink-muted",
};

export function CompaniesPage() {
  const { t, formatDate } = useI18n();
  const { workspace } = useWorkspace();
  const me = useCurrentUser();
  const [filters, setFilters] = useState<CompanyFilters>({ q: "", ats: "", status: "active" });
  const [text, setText] = useState("");
  const [adding, setAdding] = useState(false);
  const [notice, setNotice] = useState("");
  const companies = useCompanies(workspace.id, filters);
  const block = useBlockCompany(workspace.id);
  const disable = useDisableCompany(workspace.id);
  const list = companies.data?.pages.flatMap((p) => p.data) ?? [];

  const onSearch = (e: FormEvent) => {
    e.preventDefault();
    setFilters({ ...filters, q: text.trim() });
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <PageTitle>{t("companies.title")}</PageTitle>
        <Button onClick={() => setAdding(true)}>
          <Plus aria-hidden className="size-4" />
          {t("companies.add")}
        </Button>
      </div>
      {notice && (
        <p role="status" className="rounded-control bg-sage px-4 py-2 text-sm text-ink">
          {notice}
        </p>
      )}
      {adding && (
        <AddCompanyDialog
          onClose={() => setAdding(false)}
          onAdded={(c) => {
            setAdding(false);
            setNotice(t("companies.added", { name: c.name }));
          }}
        />
      )}

      <div className="flex flex-wrap items-end gap-3">
        <form role="search" onSubmit={onSearch} className="min-w-56 flex-1">
          <label htmlFor="companies-q" className="sr-only">
            {t("companies.search")}
          </label>
          <input
            id="companies-q"
            type="search"
            value={text}
            onChange={(e) => setText(e.target.value)}
            placeholder={t("companies.search")}
            className={inputClass}
          />
        </form>
        <label className="text-sm text-ink-muted">
          <span className="sr-only">{t("companies.ats")}</span>
          <select
            aria-label={t("companies.ats")}
            value={filters.ats}
            onChange={(e) => setFilters({ ...filters, ats: e.target.value })}
            className="min-h-10 rounded-control border border-line-strong bg-surface px-2 text-sm text-ink"
          >
            <option value="">{t("companies.anyAts")}</option>
            {atsList.map((a) => (
              <option key={a} value={a}>
                {atsLabel(t, a)}
              </option>
            ))}
          </select>
        </label>
        <label className="text-sm text-ink-muted">
          <span className="sr-only">{t("companies.status")}</span>
          <select
            aria-label={t("companies.status")}
            value={filters.status}
            onChange={(e) => setFilters({ ...filters, status: e.target.value })}
            className="min-h-10 rounded-control border border-line-strong bg-surface px-2 text-sm text-ink"
          >
            <option value="">{t("companies.anyStatus")}</option>
            {statuses.map((s) => (
              <option key={s} value={s}>
                {t(`cstatus.${s}`)}
              </option>
            ))}
          </select>
        </label>
      </div>
      <InlineError error={block.error ?? disable.error} />

      {companies.isPending ? (
        <SkeletonCards count={6} className="md:grid-cols-2" />
      ) : companies.isError ? (
        <ErrorState error={companies.error} onRetry={() => companies.refetch()} />
      ) : list.length === 0 ? (
        <EmptyState title={t("companies.empty")} />
      ) : (
        <div className="overflow-x-auto rounded-card bg-surface shadow-card">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-line bg-canvas text-xs uppercase tracking-wide text-ink-muted">
              <tr>
                <th className="px-4 py-2 font-medium">{t("companies.title")}</th>
                <th className="px-4 py-2 font-medium">{t("companies.ats")}</th>
                <th className="px-4 py-2 font-medium">{t("companies.status")}</th>
                <th className="px-4 py-2 text-right font-medium">{t("companies.jobs")}</th>
                <th className="hidden px-4 py-2 font-medium md:table-cell">{t("companies.checked")}</th>
                <th className="px-4 py-2" />
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {list.map((c) => (
                <tr key={c.id} className={c.blocked ? "opacity-60" : ""}>
                  <td className="max-w-72 px-4 py-2.5">
                    <div className="flex items-center gap-3">
                      <CompanyTile name={c.name} size={32} />
                      <div className="min-w-0">
                        <a href={c.careers_url} target="_blank" rel="noopener noreferrer" className="block truncate font-medium text-ink hover:text-lavender-deep">
                          {c.name}
                        </a>
                        <div className="truncate text-xs text-ink-muted">{hostOf(c.careers_url)}</div>
                        {c.last_error && <div className="truncate text-xs text-danger" title={c.last_error}>{c.last_error}</div>}
                        {c.blocked && <div className="text-xs text-ink-muted">{t("companies.blocked")}</div>}
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-2.5 text-ink">{atsLabel(t, c.ats)}</td>
                  <td className="px-4 py-2.5">
                    <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${statusColor[c.status]}`}>{t(`cstatus.${c.status}`)}</span>
                  </td>
                  <td className="px-4 py-2.5 text-right tabular-nums text-ink">{c.open_jobs}</td>
                  <td className="hidden px-4 py-2.5 text-ink-muted md:table-cell">{c.last_checked_at ? formatDate(c.last_checked_at, true) : "–"}</td>
                  <td className="whitespace-nowrap px-4 py-2.5 text-right">
                    <Button variant="ghost" onClick={() => block.mutate({ id: c.id, blocked: !c.blocked })}>
                      {c.blocked ? t("companies.unblock") : t("companies.block")}
                    </Button>
                    {me.data?.is_admin && (
                      <Button variant="ghost" onClick={() => disable.mutate({ id: c.id, disabled: c.status !== "disabled" })}>
                        {c.status === "disabled" ? t("companies.enable") : t("companies.disable")}
                      </Button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {companies.hasNextPage && (
        <div className="flex justify-center">
          <Button variant="secondary" busy={companies.isFetchingNextPage} onClick={() => companies.fetchNextPage()}>
            {t("companies.loadMore")}
          </Button>
        </div>
      )}
    </div>
  );
}

function AddCompanyDialog({ onClose, onAdded }: { onClose: () => void; onAdded: (c: Company) => void }) {
  const { t } = useI18n();
  const { workspace } = useWorkspace();
  const add = useAddCompany(workspace.id);
  const [url, setUrl] = useState("");
  const [name, setName] = useState("");
  const [invalid, setInvalid] = useState(false);
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const d = ref.current;
    if (d && !d.open) d.showModal?.();
  }, []);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    let ok = false;
    try {
      const u = new URL(url.trim());
      ok = u.protocol === "http:" || u.protocol === "https:";
    } catch {
      ok = false;
    }
    setInvalid(!ok);
    if (!ok) return;
    add.mutate({ careers_url: url.trim(), ...(name.trim() ? { name: name.trim() } : {}) }, { onSuccess: onAdded });
  };

  return (
    <dialog
      ref={ref}
      open
      aria-labelledby="add-company-title"
      onCancel={onClose}
      className="fixed inset-0 z-50 m-auto w-[min(32rem,calc(100%-2rem))] rounded-hero bg-surface p-0 text-ink shadow-raised backdrop:bg-scrim"
    >
      <form onSubmit={submit} noValidate className="space-y-4 p-5">
        <div className="flex items-start justify-between gap-3">
          <h2 id="add-company-title" className="text-lg font-semibold">
            {t("companies.addTitle")}
          </h2>
          <button type="button" onClick={onClose} aria-label={t("common.cancel")} className="rounded p-1 text-ink-muted hover:bg-canvas">
            <X aria-hidden className="size-4" />
          </button>
        </div>
        <Field id="company-url" label={t("companies.url")} error={invalid ? t("companies.urlInvalid") : undefined}>
          <input
            id="company-url"
            type="url"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder="https://acme.example/careers"
            aria-invalid={invalid}
            className={inputClass}
          />
        </Field>
        <Field id="company-name" label={t("companies.name")}>
          <input id="company-name" value={name} onChange={(e) => setName(e.target.value)} className={inputClass} />
        </Field>
        <InlineError error={add.error} />
        <div className="flex justify-end gap-2">
          <Button variant="secondary" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button type="submit" busy={add.isPending}>
            {t("common.add")}
          </Button>
        </div>
      </form>
    </dialog>
  );
}
