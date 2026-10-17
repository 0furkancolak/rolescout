import { type RouteObject } from "react-router-dom";
import { AppShell } from "../components/AppShell";
import { NotFound } from "../components/NotFound";
import { ApplicationDetailPage } from "../features/applications/ApplicationDetailPage";
import { ApplicationsPage } from "../features/applications/ApplicationsPage";
import { LoginPage } from "../features/auth/LoginPage";
import { RegisterPage } from "../features/auth/RegisterPage";
import { RequireAuth } from "../features/auth/RequireAuth";
import { JobRedirect } from "../features/jobs/JobRedirect";
import { JobSearchPage } from "../features/jobs/JobSearchPage";
import { DashboardPage } from "../features/dashboard/DashboardPage";
import { SavedJobsPage } from "../features/saved/SavedJobsPage";
import { WorkspaceProvider } from "../features/workspaces/WorkspaceProvider";

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
      { path: "jobs", element: <JobSearchPage /> },
      { path: "jobs/:jobId", element: <JobRedirect /> },
      { path: "saved", element: <SavedJobsPage /> },
      { path: "applications", element: <ApplicationsPage /> },
      { path: "applications/:applicationId", element: <ApplicationDetailPage /> },
      { path: "*", element: <NotFound /> },
    ],
  },
];

