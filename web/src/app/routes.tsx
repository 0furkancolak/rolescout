import { lazy } from "react";
import { type RouteObject } from "react-router-dom";
import { AppShell } from "../components/AppShell";
import { NotFound } from "../components/NotFound";
import { LoginPage } from "../features/auth/LoginPage";
import { RegisterPage } from "../features/auth/RegisterPage";
import { RequireAuth } from "../features/auth/RequireAuth";
import { JobRedirect } from "../features/jobs/JobRedirect";
import { JobsPage } from "../features/jobs/JobsPage";
import { DashboardPage } from "../features/dashboard/DashboardPage";
import { WorkspaceProvider } from "../features/workspaces/WorkspaceProvider";

const ApplicationDetailPage = lazy(() => import("../features/applications/ApplicationDetailPage").then((m) => ({ default: m.ApplicationDetailPage })));
const AutofillPage = lazy(() => import("../features/autofill/AutofillPage").then((m) => ({ default: m.AutofillPage })));
const ApplicationsPage = lazy(() => import("../features/applications/ApplicationsPage").then((m) => ({ default: m.ApplicationsPage })));
const CompaniesPage = lazy(() => import("../features/companies/CompaniesPage").then((m) => ({ default: m.CompaniesPage })));
const SettingsPage = lazy(() => import("../features/settings/SettingsPage").then((m) => ({ default: m.SettingsPage })));
const ProfileEditPage = lazy(() => import("../features/research/ProfileEditPage").then((m) => ({ default: m.ProfileEditPage })));
const ResearchPage = lazy(() => import("../features/research/ResearchPage").then((m) => ({ default: m.ResearchPage })));
const RunPage = lazy(() => import("../features/research/RunPage").then((m) => ({ default: m.RunPage })));
const SavedJobsPage = lazy(() => import("../features/saved/SavedJobsPage").then((m) => ({ default: m.SavedJobsPage })));
const AboutPage = lazy(() => import("../features/research/AboutPage").then((m) => ({ default: m.AboutPage })));

export const routes: RouteObject[] = [
  { path: "/login", element: <LoginPage /> },
  { path: "/register", element: <RegisterPage /> },
  {
    path: "/",
    element: (
      <RequireAuth>
        <WorkspaceProvider>
          <AppShell />
        </WorkspaceProvider>
      </RequireAuth>
    ),
    children: [
      { index: true, element: <DashboardPage /> },
      { path: "research", element: <ResearchPage /> },
      { path: "about", element: <AboutPage /> },
      { path: "research/profiles/:profileId", element: <ProfileEditPage /> },
      { path: "research/runs/:runId", element: <RunPage /> },
      { path: "jobs", element: <JobsPage /> },
      { path: "jobs/:jobId", element: <JobRedirect /> },
      { path: "saved", element: <SavedJobsPage /> },
      { path: "applications", element: <ApplicationsPage /> },
      { path: "applications/:applicationId", element: <ApplicationDetailPage /> },
      { path: "autofill", element: <AutofillPage /> },
      { path: "companies", element: <CompaniesPage /> },
      { path: "settings", element: <SettingsPage /> },
      { path: "*", element: <NotFound /> },
    ],
  },
];

