import { zodResolver } from "@hookform/resolvers/zod";
import { Controller, useForm } from "react-hook-form";
import { z } from "zod";
import { Button, Field, InlineError } from "../../components/ui";
import { inputClass } from "../../components/styles";
import type { RemotePreference } from "../../lib/api";
import { resolveCountry } from "../../lib/countries";
import { useI18n } from "../../lib/i18n";
import type { ProfileInput } from "./api";
import { ChipInput, MAX_ITEMS } from "./ChipInput";

const remoteOptions: RemotePreference[] = ["any", "remote_only", "remote_or_hybrid", "onsite_ok"];
const seniorityOptions = ["intern", "junior", "mid", "senior", "lead"] as const;

const list = z.array(z.string().max(60)).max(MAX_ITEMS);
const schema = z.object({
  name: z.string().trim().min(1, "required").max(100),
  title_keywords: list,
  keywords: list,
  technologies: list,
  countries: list,
  locations: list,
  excluded_keywords: list,
  remote: z.enum(["any", "remote_only", "remote_or_hybrid", "onsite_ok"]),
  seniority: z.array(z.string()),
  schedule_hours: z.number().int().min(1).max(336),
});

export function ProfileForm({
  initial,
  onSubmit,
  busy,
  error,
}: {
  initial: ProfileInput;
  onSubmit: (p: ProfileInput) => void;
  busy: boolean;
  error: unknown;
}) {
  const { t } = useI18n();
  const { register, control, handleSubmit, formState } = useForm<ProfileInput>({
    resolver: zodResolver(schema),
    defaultValues: initial,
  });

  const chips = (name: keyof ProfileInput, label: string, transform?: (s: string) => string, hint?: string) => (
    <Field id={`profile-${name}`} label={label} hint={hint}>
      <Controller
        control={control}
        name={name}
        render={({ field }) => (
          <ChipInput
            id={`profile-${name}`}
            value={field.value as string[]}
            onChange={field.onChange}
            transform={transform}
            describedBy={hint ? `profile-${name}-hint` : undefined}
          />
        )}
      />
    </Field>
  );

  return (
    <form onSubmit={handleSubmit(onSubmit)} noValidate className="space-y-5">
      <Field id="profile-name" label={t("research.name")} error={formState.errors.name ? t("research.nameRequired") : undefined}>
        <input
          id="profile-name"
          {...register("name")}
          aria-invalid={!!formState.errors.name}
          className={inputClass}
        />
      </Field>
      <div className="grid gap-5 md:grid-cols-2">
        {chips("title_keywords", t("research.titleKeywords"), undefined, t("research.titleKeywordsHint"))}
        {chips("technologies", t("research.technologies"), (s) => s.toLowerCase())}
        {chips("keywords", t("research.keywords"))}
        {chips("excluded_keywords", t("research.excluded"))}
        {chips("countries", t("research.countries"), resolveCountry, t("research.countriesHint"))}
        {chips("locations", t("research.locations"))}
      </div>
      <fieldset>
        <legend className="mb-2 text-sm font-medium text-ink">{t("research.remote")}</legend>
        <div className="flex flex-wrap gap-2">
          {remoteOptions.map((r) => (
            <label key={r} className="inline-flex cursor-pointer items-center gap-2 rounded-lg border border-line px-3 py-2 text-sm has-checked:border-ink has-checked:bg-lavender">
              <input type="radio" value={r} {...register("remote")} className="accent-[var(--color-ink)]" />
              {t(`research.remote.${r}`)}
            </label>
          ))}
        </div>
      </fieldset>
      <fieldset>
        <legend className="mb-1 text-sm font-medium text-ink">{t("research.seniority")}</legend>
        <p className="mb-2 text-xs text-ink-muted">{t("research.seniorityAny")}</p>
        <div className="flex flex-wrap gap-2">
          {seniorityOptions.map((s) => (
            <label key={s} className="inline-flex cursor-pointer items-center gap-2 rounded-lg border border-line px-3 py-2 text-sm has-checked:border-ink has-checked:bg-lavender">
              <input type="checkbox" value={s} {...register("seniority")} className="accent-[var(--color-ink)]" />
              {t(`seniority.${s}`)}
            </label>
          ))}
        </div>
      </fieldset>
      <Field id="profile-schedule" label={t("research.schedule")}>
        <input
          id="profile-schedule"
          type="number"
          min={1}
          max={336}
          {...register("schedule_hours", { valueAsNumber: true })}
          className={`${inputClass} max-w-32`}
        />
      </Field>
      <InlineError error={error} />
      <Button type="submit" busy={busy}>
        {t("common.save")}
      </Button>
    </form>
  );
}
