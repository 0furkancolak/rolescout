import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Button, Card, ErrorState, Field, InlineError, Spinner } from "../../components/ui";
import { inputClass } from "../../components/styles";
import { data, isApiError, type Locale, type User } from "../../lib/api";
import { useI18n } from "../../lib/i18n";

const usersKey = ["admin", "users"] as const;

// UsersCard: the administrator adds the other people who may sign in. There
// is no public sign-up once the first account exists.
export function UsersCard() {
  const { t, formatDate } = useI18n();
  const client = useQueryClient();
  const users = useQuery({ queryKey: usersKey, queryFn: ({ signal }) => data<User[]>("/admin/users", { signal }) });
  const [form, setForm] = useState({ display_name: "", email: "", password: "", locale: "tr" as Locale });
  const create = useMutation({
    mutationFn: () => data<User>("/admin/users", { method: "POST", body: form }),
    onSuccess: () => {
      setForm({ display_name: "", email: "", password: "", locale: form.locale });
      return client.invalidateQueries({ queryKey: usersKey, exact: true });
    },
  });
  const fieldError = (name: string) =>
    isApiError(create.error, "invalid_input") ? create.error.fields.find((f) => f.field === name)?.message : undefined;
  const set = (k: keyof typeof form) => (e: { target: { value: string } }) => setForm({ ...form, [k]: e.target.value });

  return (
    <Card>
      <h2 className="text-lg font-semibold text-ink">{t("users.title")}</h2>
      <p className="mt-1 text-sm text-ink-muted">{t("users.hint")}</p>
      {users.isPending ? (
        <Spinner />
      ) : users.isError ? (
        <ErrorState error={users.error} onRetry={() => users.refetch()} />
      ) : (
        <ul className="mt-4 divide-y divide-line" aria-label={t("users.title")}>
          {users.data.map((u) => (
            <li key={u.id} className="flex flex-wrap items-baseline justify-between gap-2 py-2 text-sm">
              <span className="font-medium text-ink">
                {u.display_name} <span className="font-normal text-ink-muted">{u.email}</span>
              </span>
              <span className="text-xs text-ink-muted">
                {u.is_admin ? `${t("users.admin")} · ` : ""}
                {formatDate(u.created_at)}
              </span>
            </li>
          ))}
        </ul>
      )}
      <form
        className="mt-5 grid gap-3 sm:grid-cols-2"
        onSubmit={(e) => {
          e.preventDefault();
          create.mutate();
        }}
      >
        <Field id="new_user_name" label={t("auth.displayName")} error={fieldError("display_name")}>
          <input id="new_user_name" className={inputClass} value={form.display_name} onChange={set("display_name")} autoComplete="off" />
        </Field>
        <Field id="new_user_email" label={t("auth.email")} error={fieldError("email")}>
          <input id="new_user_email" type="email" className={inputClass} value={form.email} onChange={set("email")} autoComplete="off" />
        </Field>
        <Field id="new_user_password" label={t("auth.password")} error={fieldError("password")}>
          <input id="new_user_password" type="password" className={inputClass} value={form.password} onChange={set("password")} autoComplete="new-password" />
        </Field>
        <div className="flex items-end gap-3">
          <Button type="submit" busy={create.isPending}>
            {t("users.add")}
          </Button>
          {create.isSuccess && <span className="text-sm text-ink-muted">{t("users.added")}</span>}
        </div>
        {isApiError(create.error, "conflict") ? (
          <p role="alert" className="text-sm text-danger sm:col-span-2">
            {t("users.exists")}
          </p>
        ) : (
          !isApiError(create.error, "invalid_input") && <InlineError error={create.error} />
        )}
      </form>
    </Card>
  );
}
