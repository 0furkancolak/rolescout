import { FileText, Sparkles, Trash2, Upload } from "lucide-react";
import { useRef, useState, type DragEvent } from "react";
import { Button, Card, Chip, ErrorState, Spinner } from "../../components/ui";
import { inputClass } from "../../components/styles";
import { ApiError } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { enumLabel } from "../../lib/labels";
import { useWorkspace } from "../workspaces/context";
import { useDeleteCV, useMemory, useSaveAbout, useUploadCV } from "./memory";

const SKILLS_SHOWN = 24;

// MemoryCard is where the person tells the AI what they look for: a note and a
// CV. The AI draws on both when it drafts profiles, compares technologies and
// explains matches.
export function MemoryCard() {
  const { t, formatDate } = useI18n();
  const { workspace } = useWorkspace();
  const memory = useMemory(workspace.id);
  const saveAbout = useSaveAbout(workspace.id);
  const upload = useUploadCV(workspace.id);
  const remove = useDeleteCV(workspace.id);
  const [draft, setDraft] = useState<string | null>(null);
  const [over, setOver] = useState(false);
  const input = useRef<HTMLInputElement>(null);

  if (memory.isPending) return <Spinner />;
  if (memory.isError) return <ErrorState error={memory.error} onRetry={() => memory.refetch()} />;

  const ctx = memory.data;
  const about = draft ?? ctx.about;
  const dirty = about.trim() !== ctx.about;
  const cv = ctx.cv;

  const pick = (file: File | undefined) => {
    if (file) upload.mutate(file);
    if (input.current) input.current.value = "";
  };
  const onDrop = (e: DragEvent) => {
    e.preventDefault();
    setOver(false);
    pick(e.dataTransfer.files[0]);
  };

  return (
    <Card>
      <h2 className="flex items-center gap-2 text-base font-semibold text-ink">
        <Sparkles aria-hidden className="size-4" />
        {t("memory.title")}
      </h2>
      <p className="mt-1 text-sm text-ink-muted">{t("memory.hint")}</p>

      <div className="mt-4 grid gap-6 lg:grid-cols-2">
        <div className="min-w-0">
          <label htmlFor="memory-about" className="block text-sm font-medium text-ink">
            {t("memory.aboutLabel")}
          </label>
          <textarea
            id="memory-about"
            rows={6}
            maxLength={4000}
            className={`${inputClass} mt-1.5`}
            placeholder={t("memory.aboutPlaceholder")}
            value={about}
            onChange={(e) => setDraft(e.target.value)}
          />
          <div className="mt-2 flex flex-wrap items-center gap-3">
            <Button
              variant="secondary"
              busy={saveAbout.isPending}
              disabled={!dirty}
              onClick={() => saveAbout.mutate(about.trim(), { onSuccess: () => setDraft(null) })}
            >
              {t("memory.save")}
            </Button>
            {saveAbout.isSuccess && !dirty && <span className="text-sm text-ink-muted">{t("memory.saved")}</span>}
            {saveAbout.isError && (
              <span role="alert" className="text-sm text-danger">
                {t("common.error")}
              </span>
            )}
          </div>
        </div>

        <div className="min-w-0">
          <h3 className="text-sm font-medium text-ink">{t("memory.cvTitle")}</h3>
          <input
            ref={input}
            type="file"
            className="sr-only"
            tabIndex={-1}
            aria-label={t("memory.cvChoose")}
            accept=".pdf,.docx,.txt,.md,application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document,text/plain"
            onChange={(e) => pick(e.target.files?.[0])}
          />
          {upload.isPending ? (
            <p role="status" className="mt-1.5 animate-pulse rounded-card border border-dashed border-line-strong p-5 text-sm text-ink-muted">
              {t("memory.reading")}
            </p>
          ) : cv ? (
            <div className="mt-1.5 space-y-3 rounded-card border border-line p-4">
              <div className="flex flex-wrap items-start justify-between gap-2">
                <p className="flex min-w-0 items-center gap-2 text-sm font-medium text-ink">
                  <FileText aria-hidden className="size-4 shrink-0" />
                  <span className="truncate">{cv.filename}</span>
                </p>
                <div className="flex gap-2">
                  <Button variant="ghost" onClick={() => input.current?.click()}>
                    <Upload aria-hidden className="size-4" />
                    {t("memory.cvReplace")}
                  </Button>
                  <Button
                    variant="ghost"
                    busy={remove.isPending}
                    aria-label={t("memory.cvDelete")}
                    onClick={() => {
                      if (window.confirm(t("memory.cvDeleteConfirm"))) remove.mutate();
                    }}
                  >
                    <Trash2 aria-hidden className="size-4" />
                  </Button>
                </div>
              </div>
              <p className="text-xs text-ink-muted">
                {t("memory.cvUploaded", { date: formatDate(cv.uploaded_at, true), chars: cv.chars })}
              </p>
              {cv.summary && <p className="text-sm leading-relaxed text-ink">{cv.summary}</p>}
              {(cv.seniority || cv.years) && (
                <p className="text-sm text-ink-muted">
                  {[cv.seniority ? enumLabel(t, "seniority", cv.seniority) : "", cv.years ? t("memory.years", { n: cv.years }) : ""]
                    .filter(Boolean)
                    .join(" · ")}
                </p>
              )}
              {cv.titles.length > 0 && (
                <ChipRow label={t("memory.cvTitles")} items={cv.titles} />
              )}
              <ChipRow
                label={t("memory.cvSkills")}
                items={cv.skills.slice(0, SKILLS_SHOWN)}
                more={cv.skills.length - SKILLS_SHOWN}
                empty={t("memory.noSkills")}
              />
              {!ctx.ai_read && <p className="text-xs text-ink-muted">{t("memory.cvNoAI")}</p>}
            </div>
          ) : (
            <div
              onDragOver={(e) => {
                e.preventDefault();
                setOver(true);
              }}
              onDragLeave={() => setOver(false)}
              onDrop={onDrop}
              className={`mt-1.5 flex flex-col items-center gap-2 rounded-card border border-dashed p-6 text-center transition-colors ${
                over ? "border-focus bg-lavender/30" : "border-line-strong"
              }`}
            >
              <Upload aria-hidden className="size-5 text-ink-muted" />
              <p className="text-sm text-ink">{t("memory.cvDrop")}</p>
              <p className="text-xs text-ink-muted">{t("memory.cvTypes")}</p>
              <Button variant="secondary" onClick={() => input.current?.click()}>
                {t("memory.cvChoose")}
              </Button>
            </div>
          )}
          {upload.isError && (
            <p role="alert" className="mt-2 text-sm text-danger">
              {t("memory.uploadFailed")}
              {upload.error instanceof ApiError && upload.error.fields[0] ? `: ${upload.error.fields[0].message}` : ""}
            </p>
          )}
        </div>
      </div>
    </Card>
  );
}

function ChipRow({ label, items, more = 0, empty }: { label: string; items: string[]; more?: number; empty?: string }) {
  const { t } = useI18n();
  return (
    <div>
      <h4 className="text-xs font-semibold uppercase tracking-wide text-ink-muted">{label}</h4>
      {items.length === 0 ? (
        <p className="mt-1 text-xs text-ink-muted">{empty}</p>
      ) : (
        <ul className="mt-1.5 flex flex-wrap gap-1.5">
          {items.map((s) => (
            <li key={s}>
              <Chip>{s}</Chip>
            </li>
          ))}
          {more > 0 && <li className="self-center text-xs text-ink-muted">{t("memory.more", { n: more })}</li>}
        </ul>
      )}
    </div>
  );
}
