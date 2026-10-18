import { X } from "lucide-react";
import { useState, type KeyboardEvent } from "react";
import { inputClass } from "../../components/styles";
import { useI18n } from "../../lib/i18n";

export const MAX_ITEMS = 30;

// ChipInput edits a list of short strings: Enter or comma adds the typed
// value, Backspace on an empty field removes the last one.
export function ChipInput({
  id,
  value,
  onChange,
  transform = (s: string) => s,
  describedBy,
}: {
  id: string;
  value: string[];
  onChange: (v: string[]) => void;
  transform?: (s: string) => string;
  describedBy?: string;
}) {
  const { t } = useI18n();
  const [text, setText] = useState("");
  const [tooMany, setTooMany] = useState(false);

  const add = () => {
    const v = transform(text.trim());
    setText("");
    if (!v || value.includes(v)) return;
    if (value.length >= MAX_ITEMS) {
      setTooMany(true);
      return;
    }
    onChange([...value, v]);
  };
  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter" || e.key === ",") {
      e.preventDefault();
      add();
    } else if (e.key === "Backspace" && text === "" && value.length) {
      onChange(value.slice(0, -1));
      setTooMany(false);
    }
  };

  return (
    <div>
      {value.length > 0 && (
        <ul className="mb-2 flex flex-wrap gap-1.5">
          {value.map((v) => (
            <li key={v} className="inline-flex items-center gap-1 rounded-md bg-lavender py-0.5 pl-2 pr-1 text-sm text-ink">
              {v}
              <button
                type="button"
                aria-label={`${t("common.remove")} ${v}`}
                onClick={() => {
                  onChange(value.filter((x) => x !== v));
                  setTooMany(false);
                }}
                className="rounded p-0.5 hover:bg-canvas"
              >
                <X aria-hidden className="size-3.5" />
              </button>
            </li>
          ))}
        </ul>
      )}
      <input
        id={id}
        value={text}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={onKeyDown}
        onBlur={add}
        placeholder={t("research.itemPlaceholder")}
        aria-describedby={describedBy}
        className={inputClass}
      />
      {tooMany && (
        <p role="alert" className="mt-1 text-xs text-danger">
          {t("research.tooMany")}
        </p>
      )}
    </div>
  );
}
