import "@testing-library/jest-dom/vitest";
import { configure } from "@testing-library/react";

// The suite runs many jsdom workers at once; the default 1 s wait for async
// queries is too tight on a busy machine and made tests flaky.
configure({ asyncUtilTimeout: 4000 });
