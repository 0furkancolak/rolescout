import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { Link, Navigate, useLocation, useNavigate } from "react-router-dom";
import { z } from "zod";
import { Button, Field, InlineError } from "../../components/ui";
import { inputClass } from "../../components/styles";
import { isApiError } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import type { MessageKey } from "../../lib/messages";
import { useCurrentUser, useLogin } from "./api";
import { AuthLayout } from "./AuthLayout";
import { redirectTarget } from "./redirect";

const schema = z.object({
  email: z.string().trim().min(1, "auth.required"),
  password: z.string().min(1, "auth.required"),
});

type FormValues = z.infer<typeof schema>;

export function LoginPage() {
  const { t } = useI18n();
  const me = useCurrentUser();
  const login = useLogin();
  const navigate = useNavigate();
  const location = useLocation();
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<FormValues>({ resolver: zodResolver(schema) });

  if (me.data) return <Navigate to={redirectTarget(location.state)} replace />;

  const onSubmit = handleSubmit((values) =>
    login.mutate(values, { onSuccess: () => navigate(redirectTarget(location.state), { replace: true }) }),
  );

  return (
    <AuthLayout title={t("auth.signInTitle")}>
      <form onSubmit={onSubmit} noValidate className="space-y-4">
        <Field id="email" label={t("auth.email")} error={errors.email && t(errors.email.message as MessageKey)}>
          <input
            id="email"
            type="email"
            autoComplete="email"
            className={inputClass}
            aria-invalid={!!errors.email}
            {...register("email")}
          />
        </Field>
        <Field
          id="password"
          label={t("auth.password")}
          error={errors.password && t(errors.password.message as MessageKey)}
        >
          <input
            id="password"
            type="password"
            autoComplete="current-password"
            className={inputClass}
            aria-invalid={!!errors.password}
            {...register("password")}
          />
        </Field>
        {isApiError(login.error, "invalid_credentials") ? (
          <p role="alert" className="text-sm text-danger">
            {t("auth.invalidCredentials")}
          </p>
        ) : (
          <InlineError error={login.error} />
        )}
        <Button type="submit" busy={login.isPending} className="w-full">
          {t("auth.signIn")}
        </Button>
      </form>
      <p className="mt-4 text-center text-sm text-ink-muted">
        {t("auth.noAccount")}{" "}
        <Link to="/register" className="font-medium text-lavender-deep hover:underline">
          {t("auth.register")}
        </Link>
      </p>
    </AuthLayout>
  );
}
