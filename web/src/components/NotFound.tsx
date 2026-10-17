import { Link } from "react-router-dom";
import { useI18n } from "../lib/i18n";
import { buttonClass } from "./styles";

export function NotFound() {
  const { t } = useI18n();
  return (
    <div className="mx-auto max-w-md space-y-3 py-16 text-center">
      <h1 className="text-2xl font-semibold">{t("common.notFound")}</h1>
      <p className="text-slate-600">{t("common.notFoundText")}</p>
      <Link to="/jobs" className={buttonClass("secondary")}>
        {t("common.goHome")}
      </Link>
    </div>
  );
}
