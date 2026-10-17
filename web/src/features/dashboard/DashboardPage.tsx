import { useI18n } from "../../lib/i18n";

export function DashboardPage() {
  const { t } = useI18n();
  return <h1 className="text-2xl font-semibold tracking-tight">{t("dash.title")}</h1>;
}
