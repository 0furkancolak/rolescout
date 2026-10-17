import { Plus, X } from "lucide-react";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { Button, EmptyState, ErrorState, Field, InlineError, PageTitle, Spinner } from "../../components/ui";
import { inputClass } from "../../components/styles";
import type { Company, CompanyStatus } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { hostOf } from "../../lib/labels";
import { useCurrentUser } from "../auth/api";
import { useWorkspace } from "../workspaces/context";
import { useAddCompany, useBlockCompany, useCompanies, useDisableCompany, type CompanyFilters } from "./api";

const statuses: CompanyStatus[] = ["pending", "active", "empty", "unreachable", "blocked_by_robots", "disabled"];
const atsList = ["greenhouse", "lever", "ashby", "workable", "recruitee", "personio", "bamboohr", "workday", "jsonld", "unknown"];

const statusColor: Record<CompanyStatus, string> = {
  pending: "bg-slate-100 text-slate-700",
  active: "bg-green-100 text-green-800",
  empty: "bg-slate-100 text-slate-700",
  unreachable: "bg-red-100 text-red-800",
  blocked_by_robots: "bg-amber-100 text-amber-900",
  disabled: "bg-slate-200 text-slate-600",
};

export function CompaniesPage() {
  const { t, formatDate } = useI18n();
  const { workspace } = useWorkspace();
  const me = useCurrentUser();
  const [filters, setFilters] = useState<CompanyFilters>({ q: "", ats: "", status: "" });
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
        <p role="status" className="rounded-lg border border-green-200 bg-green-50 px-4 py-2 text-sm text-green-800">
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
        <label className="text-sm text-slate-600">
          <span className="sr-only">{t("companies.ats")}</span>
          <select
            aria-label={t("companies.ats")}
            value={filters.ats}
            onChange={(e) => setFilters({ ...filters, ats: e.target.value })}
            className="min-h-10 rounded-md border border-slate-300 bg-white px-2 text-sm text-slate-900"
          >
            <option value="">{t("companies.anyAts")}</option>
            {atsList.map((a) => (
              <option key={a} value={a}>
                {a}
              </option>
            ))}
          </select>
        </label>
        <label className="text-sm text-slate-600">
          <span className="sr-only">{t("companies.status")}</span>
          <select
            aria-label={t("companies.status")}
            value={filters.status}
            onChange={(e) => setFilters({ ...filters, status: e.target.value })}
            className="min-h-10 rounded-md border border-slate-300 bg-white px-2 text-sm text-slate-900"
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
        <Spinner />
      ) : companies.isError ? (
        <ErrorState error={companies.error} onRetry={() => companies.refetch()} />
      ) : list.length === 0 ? (
        <EmptyState title={t("companies.empty")} />
      ) : (
        <div className="overflow-x-auto rounded-xl border border-slate-200 bg-white shadow-sm">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-slate-200 bg-slate-50 text-xs uppercase tracking-wide text-slate-500">
              <tr>
                <th className="px-4 py-2 font-medium">{t("companies.title")}</th>
                <th className="px-4 py-2 font-medium">{t("companies.ats")}</th>
                <th className="px-4 py-2 font-medium">{t("companies.status")}</th>
                <th className="px-4 py-2 text-right font-medium">{t("companies.jobs")}</th>
                <th className="hidden px-4 py-2 font-medium md:table-cell">{t("companies.checked")}</th>
                <th className="px-4 py-2" />
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200">
              {list.map((c) => (
                <tr key={c.id} className={c.blocked ? "opacity-60" : ""}>
                  <td className="max-w-72 px-4 py-2.5">
                    <a href={c.careers_url} target="_blank" rel="noopener noreferrer" className="font-medium text-slate-900 hover:text-blue-700">
                      {c.name}
                    </a>
                    <div className="truncate text-xs text-slate-500">{hostOf(c.careers_url)}</div>
                    {c.last_error && <div className="truncate text-xs text-red-700" title={c.last_error}>{c.last_error}</div>}
                    {c.blocked && <div className="text-xs text-slate-500">{t("companies.blocked")}</div>}
                  </td>
                  <td className="px-4 py-2.5 text-slate-700">{c.ats}</td>
                  <td className="px-4 py-2.5">
                    <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${statusColor[c.status]}`}>{t(`cstatus.${c.status}`)}</span>
                  </td>
                  <td className="px-4 py-2.5 text-right tabular-nums text-slate-900">{c.open_jobs}</td>
                  <td className="hidden px-4 py-2.5 text-slate-500 md:table-cell">{c.last_checked_at ? formatDate(c.last_checked_at, true) : "–"}</td>
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
      className="fixed inset-0 z-50 m-auto w-[min(32rem,calc(100%-2rem))] rounded-xl border border-slate-200 bg-white p-0 text-slate-900 shadow-xl backdrop:bg-slate-900/40"
    >
      <form onSubmit={submit} noValidate className="space-y-4 p-5">
        <div className="flex items-start justify-between gap-3">
          <h2 id="add-company-title" className="text-lg font-semibold">
            {t("companies.addTitle")}
          </h2>
          <button type="button" onClick={onClose} aria-label={t("common.cancel")} className="rounded p-1 text-slate-500 hover:bg-slate-100">
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
