import { useEffect } from "react";

// useJobKeyboard binds j/k (next/previous), s (save), o (open) and /
// (search) while focus is not in a form field.
export function useJobKeyboard(handlers: {
  next: () => void;
  previous: () => void;
  save: () => void;
  open: () => void;
  search: () => void;
}) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.metaKey || e.ctrlKey || e.altKey) return;
      const el = e.target as HTMLElement | null;
      if (el && (el.isContentEditable || ["INPUT", "TEXTAREA", "SELECT"].includes(el.tagName))) return;
      const action = { j: handlers.next, k: handlers.previous, s: handlers.save, o: handlers.open, "/": handlers.search }[e.key];
      if (action) {
        e.preventDefault();
        action();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [handlers]);
}
