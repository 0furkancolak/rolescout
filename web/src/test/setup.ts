import "@testing-library/jest-dom/vitest";
import { configure } from "@testing-library/react";
import { afterEach } from "vitest";
import { resetSummaryCache } from "../features/jobs/api";

// The suite runs many jsdom workers at once; the default 1 s wait for async
// queries is too tight on a busy machine and made tests flaky.
configure({ asyncUtilTimeout: 4000 });

// A finished summary is remembered for the session; tests start without it.
afterEach(() => resetSummaryCache());
