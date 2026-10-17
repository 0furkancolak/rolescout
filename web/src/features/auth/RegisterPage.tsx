import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { Link, Navigate, useNavigate } from "react-router-dom";
import { z } from "zod";
import { Button, Field, InlineError } from "../../components/ui";
import { inputClass } from "../../components/styles";
import { isApiError } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import type { MessageKey } from "../../lib/messages";
import { useCurrentUser, useRegister } from "./api";
import { AuthLayout } from "./AuthLayout";

// Mirrors the server rules (auth.ValidatePassword counts characters, not bytes).
const schema = z.object({
  display_name: z.string().trim().min(1, "auth.required").max(100),
  email: z.string().trim().min(1, "auth.required").pipe(z.email("auth.emailInvalid")),
  password: z
    .string()
    .refine((p) => [...p].length >= 12, "auth.passwordShort")
    .refine((p) => [...p].length <= 128, "auth.passwordLong"),
  locale: z.enum(["tr", "en"]),
});

type FormValues = z.infer<typeof schema>;

export function RegisterPage() {
  const { t, locale, setLocale } = useI18n();
  const me = useCurrentUser();
  const registerUser = useRegister();
  const navigate = useNavigate();
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<FormValues>({ resolver: zodResolver(schema), defaultValues: { locale } });

  if (me.data) return <Navigate to="/" replace />;

  const onSubmit = handleSubmit((values) =>
    registerUser.mutate(values, {
      onSuccess: (user) => {
        setLocale(user.locale);
        navigate("/", { replace: true });
      },
    }),
  );

  const message = (key?: string) => (key ? t(key as MessageKey) : undefined);
  const err = registerUser.error;

  return (
    <AuthLayout title={t("auth.registerTitle")}>
      <form onSubmit={onSubmit} noValidate className="space-y-4">
        <Field id="display_name" label={t("auth.displayName")} error={message(errors.display_name?.message)}>
          <input
            id="display_name"
            autoComplete="name"
            className={inputClass}
            aria-invalid={!!errors.display_name}
            {...register("display_name")}
          />
        </Field>
        <Field id="email" label={t("auth.email")} error={message(errors.email?.message)}>
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
          hint={t("auth.passwordHint")}
          error={message(errors.password?.message)}
        >
          <input
            id="password"
            type="password"
            autoComplete="new-password"
            className={inputClass}
            aria-invalid={!!errors.password}
            aria-describedby="password-hint"
            {...register("password")}
          />
        </Field>
        <Field id="locale" label={t("auth.language")}>
          <select id="locale" className={inputClass} {...register("locale")}>
            <option value="tr">Türkçe</option>
            <option value="en">English</option>
          </select>
        </Field>
        {isApiError(err, "registration_closed") ? (
          <p role="alert" className="text-sm text-red-700">
            {t("auth.registrationClosed")}
          </p>
        ) : isApiError(err, "conflict") ? (
          <p role="alert" className="text-sm text-red-700">
            {t("auth.emailTaken")}
          </p>
        ) : isApiError(err, "invalid_input") ? (
          <p role="alert" className="text-sm text-red-700">
            {err.fields.map((f) => `${f.field}: ${f.message}`).join(" · ")}
          </p>
        ) : (
          <InlineError error={err} />
        )}
        <Button type="submit" busy={registerUser.isPending} className="w-full">
          {t("auth.register")}
        </Button>
      </form>
      <p className="mt-4 text-center text-sm text-slate-600">
        {t("auth.haveAccount")}{" "}
        <Link to="/login" className="font-medium text-blue-700 hover:underline">
          {t("auth.signIn")}
        </Link>
      </p>
    </AuthLayout>
  );
}
