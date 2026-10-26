import { PageTitle } from "../../components/ui";
import { useI18n } from "../../lib/i18n";
import { MemoryCard } from "./MemoryCard";

// AboutPage is where the person introduces themselves to the AI: a note and
// a CV. Research profiles and match explanations draw on both.
export function AboutPage() {
  const { t } = useI18n();
  return (
    <div className="mx-auto max-w-5xl space-y-6">
      <PageTitle>{t("about.title")}</PageTitle>
      <MemoryCard />
    </div>
  );
}
