import { QueryClientProvider, type QueryClient } from "@tanstack/react-query";
import type { ReactNode } from "react";
import type { Locale } from "../lib/api";
import { I18nProvider } from "../lib/I18nProvider";

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
      <I18nProvider initialLocale={locale}>{children}</I18nProvider>
    </QueryClientProvider>
  );
}
