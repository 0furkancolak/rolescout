import type { ApplicationStatus } from "../../lib/api";

// Statuses that are still going, and those that ended the application.
export const openStatuses: ApplicationStatus[] = ["planned", "applied", "interviewing", "offer"];
export const closedStatuses: ApplicationStatus[] = ["accepted", "rejected", "withdrawn"];
export const isClosed = (s: ApplicationStatus) => closedStatuses.includes(s);
