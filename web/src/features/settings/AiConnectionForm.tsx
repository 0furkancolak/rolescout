import { AlertTriangle } from "lucide-react";
import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Button, ErrorState, Field } from "../../components/ui";
import { inputClass } from "../../components/styles";
import { ApiError, data, type LlmConfig, type LlmConfigUpdate } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { SkeletonRows } from "../../components/Skeleton";

const configKey = ["admin", "llm", "config"] as const;

const PRESETS = {
  ollama: { base_url: "http://host.docker.internal:11434/v1", model: "qwen3:4b-instruct-2507-q4_K_M" },
  openai: { base_url: "https://api.openai.com/v1", model: "gpt-4o-mini" },
} as const;

// AiConnectionForm edits the AI connection: which service, which model and the
// API key. Saved values live in the database and apply within seconds.
export function AiConnectionForm() {
  const config = useQuery({
    queryKey: configKey,
    queryFn: ({ signal }) => data<LlmConfig>("/admin/llm/config", { signal }),
  });
  if (config.isPending) return <SkeletonRows rows={3} />;
  if (config.isError) return <ErrorState error={config.error} onRetry={() => config.refetch()} />;
  return <Form config={config.data} />;
}

function Form({ config }: { config: LlmConfig }) {
  const { t } = useI18n();
  const client = useQueryClient();
  const [enabled, setEnabled] = useState(config.enabled);
  const [baseUrl, setBaseUrl] = useState(config.base_url);
  const [model, setModel] = useState(config.model);
  const [apiKey, setApiKey] = useState("");
  const [background, setBackground] = useState(config.background);
  const [chosenBackground, setChosenBackground] = useState(!config.background_auto);

  const save = useMutation({
    mutationFn: (body: LlmConfigUpdate) => data<LlmConfig>("/admin/llm/config", { method: "PUT", body }),
    onSuccess: (next) => {
      client.setQueryData(configKey, next);
      setApiKey("");
      void client.invalidateQueries({ queryKey: ["admin", "llm"], exact: true });
    },
  });
  const fieldError = (name: string) => (save.error instanceof ApiError ? save.error.fields.find((f) => f.field === name)?.message : undefined);

  const hosted = !isLocalUrl(baseUrl);
  const host = hostOf(baseUrl);

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const body: LlmConfigUpdate = { enabled, base_url: baseUrl.trim(), model: model.trim() };
    if (apiKey.trim() !== "") body.api_key = apiKey.trim();
    if (chosenBackground) body.background = background;
    else body.reset_background = true;
    save.mutate(body);
  }

  return (
    <form onSubmit={submit} className="mt-4 space-y-4" aria-label={t("aiConn.title")}>
      <label className="flex items-center gap-2 text-sm font-medium text-ink">
        <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} className="size-4" />
        {t("aiConn.enabled")}
      </label>
      <p className="text-xs text-ink-muted">{t("aiConn.hint")}</p>
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-xs font-medium text-ink-muted">{t("aiConn.preset")}</span>
        {(["ollama", "openai"] as const).map((key) => (
          <Button
            key={key}
            type="button"
            variant="ghost"
            onClick={() => {
              setBaseUrl(PRESETS[key].base_url);
              setModel(PRESETS[key].model);
              if (!chosenBackground) setBackground(key === "ollama");
            }}
          >
            {t(key === "ollama" ? "aiConn.presetOllama" : "aiConn.presetOpenAI")}
          </Button>
        ))}
      </div>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field id="ai-base-url" label={t("aiConn.baseUrl")} error={fieldError("base_url")}>
          <input id="ai-base-url" value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} spellCheck={false} className={`${inputClass} font-mono`} />
        </Field>
        <Field id="ai-model" label={t("aiConn.model")} error={fieldError("model")}>
          <input id="ai-model" value={model} onChange={(e) => setModel(e.target.value)} spellCheck={false} className={`${inputClass} font-mono`} />
        </Field>
      </div>
      <Field
        id="ai-api-key"
        label={t("aiConn.apiKey")}
        error={fieldError("api_key")}
        hint={config.has_api_key ? t("aiConn.apiKeyKeep", { hint: config.api_key_hint ?? "" }) : t("aiConn.apiKeyNone")}
      >
        <input
          id="ai-api-key"
          type="password"
          autoComplete="off"
          value={apiKey}
          onChange={(e) => setApiKey(e.target.value)}
          placeholder={config.has_api_key ? (config.api_key_hint ?? "") : "sk-…"}
          className={`${inputClass} font-mono`}
        />
      </Field>
      {config.has_api_key && (
        <Button type="button" variant="ghost" busy={save.isPending} onClick={() => save.mutate({ api_key: "" })}>
          {t("aiConn.apiKeyRemove")}
        </Button>
      )}
      <div className="space-y-1">
        <label className="flex items-center gap-2 text-sm font-medium text-ink">
          <input
            type="checkbox"
            checked={background}
            onChange={(e) => {
              setBackground(e.target.checked);
              setChosenBackground(true);
            }}
            className="size-4"
          />
          {t("aiConn.background")}
        </label>
        <p className="text-xs text-ink-muted">
          {chosenBackground ? t("aiConn.backgroundChosen") : hosted ? t("aiConn.backgroundAutoHosted") : t("aiConn.backgroundAutoLocal")}
        </p>
      </div>
      {hosted && (
        <p role="note" className="flex items-start gap-2 rounded-control bg-amber p-3 text-sm text-ink">
          <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
          {t("aiConn.hostedWarning", { host })}
        </p>
      )}
      <div className="flex items-center gap-3">
        <Button type="submit" busy={save.isPending}>
          {t("aiConn.save")}
        </Button>
        <span role="status" className="text-sm text-ink">
          {save.isSuccess ? t("aiConn.saved") : null}
        </span>
        {save.isError && !(save.error instanceof ApiError && save.error.fields.length > 0) && (
          <p role="alert" className="text-sm text-danger">
            {t("common.error")}
          </p>
        )}
      </div>
    </form>
  );
}

function hostOf(raw: string): string {
  try {
    return new URL(raw).host;
  } catch {
    return raw;
  }
}

// Mirrors enrich.IsLocalURL on the server: this machine or a private network.
function isLocalUrl(raw: string): boolean {
  let host: string;
  try {
    host = new URL(raw).hostname.replace(/^\[|\]$/g, "").toLowerCase();
  } catch {
    return true; // nothing typed yet: no warning
  }
  if (["localhost", "host.docker.internal", "host.containers.internal"].includes(host)) return true;
  const m = host.match(/^(\d+)\.(\d+)\.\d+\.\d+$/);
  if (m) {
    const [a, b] = [Number(m[1]), Number(m[2])];
    return a === 127 || a === 10 || (a === 192 && b === 168) || (a === 172 && b >= 16 && b <= 31) || (a === 169 && b === 254);
  }
  return host === "::1";
}
