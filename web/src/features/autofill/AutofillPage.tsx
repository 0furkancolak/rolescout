import { useEffect, useState, type FormEvent } from "react";
import { Button, Card, ErrorState, Field, InlineError, PageTitle, Spinner } from "../../components/ui";
import { inputClass } from "../../components/styles";
import { autofillFields, bookmarklet, type AutofillProfile } from "../../lib/autofill";
import { useI18n } from "../../lib/i18n";
import { usePutUserState, useUserState } from "../state/api";
import { useWorkspace } from "../workspaces/context";

const types: Partial<Record<(typeof autofillFields)[number], string>> = { email: "email", phone: "tel", linkedin: "url", github: "url", website: "url" };

// AutofillPage keeps the details used to fill in application forms.
export function AutofillPage() {
  const { t } = useI18n();
  const { workspace } = useWorkspace();
  const state = useUserState<AutofillProfile>(workspace.id, "autofill");
  const put = usePutUserState<AutofillProfile>(workspace.id, "autofill");
  const [form, setForm] = useState<AutofillProfile>({});
  const [saved, setSaved] = useState(false);
  useEffect(() => {
    if (state.data) setForm(state.data);
  }, [state.data]);

  if (state.isPending) return <Spinner />;
  if (state.isError) return <ErrorState error={state.error} onRetry={() => void state.refetch()} />;

  const submit = (e: FormEvent) => {
    e.preventDefault();
    const clean = Object.fromEntries(autofillFields.map((f) => [f, (form[f] ?? "").trim()]).filter(([, v]) => v)) as AutofillProfile;
    setSaved(false);
    put.mutate(clean, { onSuccess: () => setSaved(true) });
  };

  return (
    <div className="max-w-3xl space-y-6">
      <PageTitle>{t("autofill.title")}</PageTitle>
      <p className="text-sm text-ink-muted">{t("autofill.intro")}</p>
      <Card>
        <form onSubmit={submit} className="grid gap-4 sm:grid-cols-2">
          {autofillFields.map((f) => (
            <Field key={f} id={`af-${f}`} label={t(`autofill.field.${f}`)}>
              <input
                id={`af-${f}`}
                type={types[f] ?? "text"}
                value={form[f] ?? ""}
                maxLength={200}
                onChange={(e) => {
                  setSaved(false);
                  setForm({ ...form, [f]: e.target.value });
                }}
                className={inputClass}
              />
            </Field>
          ))}
          <div className="flex items-center gap-3 sm:col-span-2">
            <Button type="submit" busy={put.isPending}>
              {t("common.save")}
            </Button>
            {saved && (
              <span role="status" className="text-sm text-sage-deep">
                {t("autofill.saved")}
              </span>
            )}
          </div>
          <div className="sm:col-span-2">
            <InlineError error={put.error} />
          </div>
        </form>
      </Card>
      <Card>
        <h2 className="text-lg font-semibold text-ink">{t("autofill.bookmarklet")}</h2>
        <p className="mt-1 text-sm text-ink-muted">{t("autofill.howTo")}</p>
        <a
          ref={(el) => el?.setAttribute("href", bookmarklet(form))}
          onClick={(e) => e.preventDefault()}
          draggable
          className="mt-3 inline-flex min-h-10 cursor-grab items-center rounded-control border border-dashed border-line-strong bg-surface px-3 text-sm font-medium text-ink"
        >
          {t("autofill.bookmarklet")}
        </a>
        <p className="mt-3 text-xs text-ink-muted">{t("autofill.privacy")}</p>
      </Card>
    </div>
  );
}
