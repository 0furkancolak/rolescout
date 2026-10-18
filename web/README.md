# RoleScout Web

React + TypeScript + Vite frontend for RoleScout. It talks to the Go API at
`/api/v1` on the same origin, so the session cookie stays first-party.

## Development

```bash
npm ci
npm run dev        # http://localhost:5173, proxies /api to http://127.0.0.1:8080
npm run lint
npm test
npm run build
```

Set `API_PROXY_TARGET` to point the dev server at another API address. In
Docker, nginx serves the build and proxies `/api/` to the `api` service
(`nginx.conf`).

## Structure

```
src/
  app/          router, providers, query client
  components/   layout and shared UI (ui.tsx, styles.ts)
  features/     one folder per feature, each with api.ts hooks and pages
    auth/  workspaces/  dashboard/  research/  jobs/  saved/
    applications/  companies/  settings/
  lib/          API client and types, i18n, labels
  test/         test setup and helpers (mockApi, renderApp)
```

- Server state uses TanStack Query; each feature's `api.ts` owns its query
  keys. Mutations invalidate the queries they affect.
- The job browser (`features/jobs/JobsPage.tsx`) keeps its whole state in the
  URL (`searchParams.ts`), including the selected job, so views can be
  bookmarked and shared. Keys: `j`/`k`, `s`, `o`, `/`.
- Research runs are polled every 2 seconds while queued or running
  (`features/research/api.ts`); the log fetches only lines after the last one
  it has.
- Dark mode: `lib/ThemeProvider.tsx` sets the `dark` class on `<html>`;
  `index.css` remaps the slate scale and the few light tints in use. Prefer
  those classes (or add an override) when introducing new colors.
- Forms use React Hook Form + Zod where client validation mirrors server rules
  (sign-in, registration). The server stays the authority for every rule.
- `lib/api.ts` mirrors the status transitions in
  `api/internal/domain/rules.go`; update both together.

## Languages

UI strings live in `src/lib/messages.ts`. English is the reference and
TypeScript requires Turkish to define the same keys. The language is taken
from the saved choice, then the browser language, and can be switched in the
header. Dates are stored in UTC and shown in the viewer's time zone.

## Tests

Tests live in `__tests__` folders next to the code they cover and run with
Vitest and Testing Library. `src/test/utils.tsx` provides `mockApi`, an
in-memory replacement for `fetch` keyed by `"METHOD /path"`, and `renderApp`,
which renders the real routes at a given URL.
