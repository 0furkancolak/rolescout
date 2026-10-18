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

## Design tokens

Colours, radii and shadows are semantic tokens defined once in
`src/lib/tokens.ts` and repeated in `src/index.css` (`@theme static` for
light, a `.dark` block for dark). A test keeps the two in step and checks the
WCAG contrast of every text/background pair: 4.5:1 for text, 3:1 for
`line-strong` and `focus`. `line` is decorative only (hairlines between cards);
controls such as inputs must use `line-strong`. Use the utilities
(`bg-canvas`, `bg-surface`, `text-ink`, `text-ink-muted`, `border-line`,
`rounded-card`, `shadow-card`, ...) and never a raw colour in new code.

| token | light | dark |
|---|---|---|
| canvas / surface / raised | `#f5f1ea` / `#fffdf9` / `#ffffff` | `#121110` / `#1b1a18` / `#262421` |
| ink / ink-muted | `#17171a` / `#625d55` | `#f3eee6` / `#a8a096` |
| sage, amber, lavender (fills) | `#a9cdb4`, `#f0c777`, `#cbc3f2` | `#2f4a3a`, `#4f3b12`, `#3d3768` |
| the same, deep (text) | `#35604a`, `#8a5a06`, `#4e4390` | `#9fd0ae`, `#f0c777`, `#cbc3f2` |
