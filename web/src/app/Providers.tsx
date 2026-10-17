import { QueryClientProvider, type QueryClient } from "@tanstack/react-query";
import type { ReactNode } from "react";
import type { Locale } from "../lib/api";
import { I18nProvider } from "../lib/I18nProvider";
import { ThemeProvider } from "../lib/ThemeProvider";

export function Providers({
  client,
  locale,
  children,
}: {
  client: QueryClient;
  locale?: Locale;
  children: ReactNode;
}) {
  return (
    <QueryClientProvider client={client}>
      <I18nProvider initialLocale={locale}>
        <ThemeProvider>{children}</ThemeProvider>
      </I18nProvider>
    </QueryClientProvider>
  );
}
