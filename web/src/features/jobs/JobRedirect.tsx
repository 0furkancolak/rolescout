import { Navigate, useParams } from "react-router-dom";

// Old job detail links open the job in the job browser.
export function JobRedirect() {
  const { jobId = "" } = useParams();
  return <Navigate to={`/jobs?job=${encodeURIComponent(jobId)}`} replace />;
}
