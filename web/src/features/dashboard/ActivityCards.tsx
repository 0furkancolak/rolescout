import { CalendarClock, Sparkles, Briefcase, Send } from "lucide-react";
import { MiniBars } from "../../components/MiniBars";
import { StatCard } from "../../components/StatCard";
import type { Application, ResearchRun } from "../../lib/api";
import { useI18n } from "../../lib/i18n";
import { useWorkspace } from "../workspaces/context";
import { useApplications } from "../applications/api";
import { useActivity } from "./api";

const DASH = "–";
const DAYS = 14;

// The four headline numbers on the dashboard. Each degrades to a dash while its
// data is loading or unavailable, so one failing request never hides the rest.
export function ActivityCards({ profileID, lastRun }: { profileID?: string; lastRun?: ResearchRun }) {
  const { t } = useI18n();
  const { workspace } = useWorkspace();
  const activity = useActivity(workspace.id, DAYS).data;
  const apps = useApplications(workspace.id).data;

  const matches = activity?.new_matches;
  const todayMatches = matches?.length ? matches[matches.length - 1] : undefined;
  const openJobs = lastRun?.summary.jobs_open;
  const stages = inProgress(apps);

  return (
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <StatCard
        label={t("dash.stat.newToday")}
        value={todayMatches === undefined ? DASH : todayMatches.toLocaleString()}
        delta={matches ? t("dash.stat.last14", { n: matches.reduce((a, b) => a + b, 0).toLocaleString() }) : undefined}
        series={matches}
        icon={<Sparkles className="size-4" />}
        href={profileID ? `/jobs?profile=${profileID}&sort=recent&posted=24h` : "/jobs"}
      />
      <StatCard
        label={t("dash.stat.openJobs")}
        value={openJobs === undefined ? DASH : openJobs.toLocaleString()}
        series={activity?.jobs_collected}
        icon={<Briefcase className="size-4" />}
      />
      <StatCard
        label={t("dash.stat.inProgress")}
        value={apps ? stages.reduce((a, b) => a + b, 0).toLocaleString() : DASH}
        chart={apps ? <MiniBars values={stages} label={t("dash.stat.inProgress")} /> : undefined}
        icon={<Send className="size-4" />}
        href="/applications?view=board"
      />
      <StatCard
        label={t("dash.stat.interviews")}
        value={activity ? activity.upcoming_interviews.toLocaleString() : DASH}
        icon={<CalendarClock className="size-4" />}
        href="/applications"
      />
    </div>
  );
}

// Applied, interviewing and offer, in that order.
function inProgress(apps?: Application[]): number[] {
  const counts = [0, 0, 0];
  const index: Record<string, number> = { applied: 0, interviewing: 1, offer: 2 };
  for (const a of apps ?? []) if (a.status in index) counts[index[a.status]]++;
  return counts;
}
