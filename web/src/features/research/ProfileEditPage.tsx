import { Trash2 } from "lucide-react";
import { useNavigate, useParams } from "react-router-dom";
import { BackLink } from "../../components/BackLink";
import { Button, Card, ErrorState, PageTitle, Spinner } from "../../components/ui";
import { useI18n } from "../../lib/i18n";
import { useWorkspace } from "../workspaces/context";
import { useDeleteProfile, useProfile, useSaveProfile } from "./api";
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
      <Card>
        <ProfileForm
          key={profile.data?.id ?? "new"}
          initial={profile.data ? toInput(profile.data) : emptyProfile()}
          busy={save.isPending}
          error={save.error}
          onSubmit={(p) => save.mutate(p, { onSuccess: () => navigate("/research") })}
        />
      </Card>
    </div>
  );
}
