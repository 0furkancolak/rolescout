import { Sparkles, Trash2 } from "lucide-react";
import { useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { BackLink } from "../../components/BackLink";
import { Button, Card, ErrorState, PageTitle, Spinner } from "../../components/ui";
import { inputClass } from "../../components/styles";
import { isApiError } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { useWorkspace } from "../workspaces/context";
import { useDeleteProfile, useProfile, useSaveProfile, useSuggestProfile, type ProfileInput } from "./api";
import { ProfileForm } from "./ProfileForm";
import { emptyProfile, toInput } from "./profileInput";

export function ProfileEditPage() {
  const { t } = useI18n();
  const { profileId = "new" } = useParams();
  const isNew = profileId === "new";
  const { workspace } = useWorkspace();
  const profile = useProfile(workspace.id, isNew ? "" : profileId);
  const save = useSaveProfile(workspace.id, isNew ? undefined : profileId);
  const remove = useDeleteProfile(workspace.id);
  const navigate = useNavigate();
  const suggest = useSuggestProfile(workspace.id);
  const [description, setDescription] = useState("");
  // A draft from the AI replaces the form's starting values; the key remounts it.
  const [draft, setDraft] = useState<{ n: number; input: ProfileInput } | null>(null);

  if (!isNew && profile.isPending) return <Spinner />;
  if (!isNew && profile.isError) return <ErrorState error={profile.error} onRetry={() => profile.refetch()} />;

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <BackLink to="/research">{t("research.title")}</BackLink>
      <div className="flex items-center justify-between gap-3">
        <PageTitle>{isNew ? t("research.newProfile") : t("research.editProfile")}</PageTitle>
        {!isNew && (
          <Button
            variant="danger"
            busy={remove.isPending}
            onClick={() => {
              if (window.confirm(t("research.deleteConfirm"))) {
                remove.mutate(profileId, { onSuccess: () => navigate("/research", { replace: true }) });
              }
            }}
          >
            <Trash2 aria-hidden className="size-4" />
            {t("research.delete")}
          </Button>
        )}
      </div>
      {isNew && (
        <Card>
          <h2 className="flex items-center gap-2 text-base font-semibold text-ink">
            <Sparkles aria-hidden className="size-4" />
            {t("research.aiTitle")}
          </h2>
          <p className="mt-1 text-sm text-ink-muted">{t("research.aiHint")}</p>
          <label htmlFor="profile-ai-description" className="sr-only">
            {t("research.aiTitle")}
          </label>
          <textarea
            id="profile-ai-description"
            rows={3}
            maxLength={2000}
            className={`${inputClass} mt-3`}
            placeholder={t("research.aiPlaceholder")}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
          <div className="mt-3 flex flex-wrap items-center gap-3">
            <Button
              type="button"
              variant="secondary"
              busy={suggest.isPending}
              disabled={description.trim().length < 10}
              onClick={() =>
                suggest.mutate(description.trim(), {
                  onSuccess: (input) => setDraft({ n: (draft?.n ?? 0) + 1, input }),
                })
              }
            >
              {t("research.aiDraft")}
            </Button>
            {suggest.isSuccess && <span className="text-sm text-ink-muted">{t("research.aiDone")}</span>}
          </div>
          {suggest.isError && (
            <p role="alert" className="mt-3 text-sm text-danger">
              {suggestError(t, suggest.error)}
            </p>
          )}
        </Card>
      )}
      <Card>
        <ProfileForm
          key={draft ? `draft-${draft.n}` : (profile.data?.id ?? "new")}
          initial={draft ? draft.input : profile.data ? toInput(profile.data) : emptyProfile()}
          busy={save.isPending}
          error={save.error}
          onSubmit={(p) => save.mutate(p, { onSuccess: () => navigate("/research") })}
        />
      </Card>
    </div>
  );
}

function suggestError(t: ReturnType<typeof useI18n>["t"], err: unknown): string {
  if (isApiError(err, "ai_disabled")) return t("ai.generateOff");
  if (isApiError(err, "ai_unavailable")) return t("ai.generateUnavailable");
  if (isApiError(err, "ai_failed")) return t("research.aiFailed");
  if (isApiError(err, "rate_limited")) return t("research.aiBusy");
  return t("common.error");
}
