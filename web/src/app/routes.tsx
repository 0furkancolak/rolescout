import { Navigate, type RouteObject } from "react-router-dom";
import { Layout } from "../components/Layout";
import { NotFound } from "../components/NotFound";
import { ApplicationDetailPage } from "../features/applications/ApplicationDetailPage";
import { ApplicationsPage } from "../features/applications/ApplicationsPage";
import { LoginPage } from "../features/auth/LoginPage";
import { RegisterPage } from "../features/auth/RegisterPage";
import { RequireAuth } from "../features/auth/RequireAuth";
import { JobDetailPage } from "../features/jobs/JobDetailPage";
import { JobSearchPage } from "../features/jobs/JobSearchPage";
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
          <Layout />
        </WorkspaceProvider>
      </RequireAuth>
    ),
    children: [
      { index: true, element: <Navigate to="/jobs" replace /> },
      { path: "jobs", element: <JobSearchPage /> },
      { path: "jobs/:jobId", element: <JobDetailPage /> },
      { path: "saved", element: <SavedJobsPage /> },
      { path: "applications", element: <ApplicationsPage /> },
      { path: "applications/:applicationId", element: <ApplicationDetailPage /> },
      { path: "*", element: <NotFound /> },
    ],
  },
];
