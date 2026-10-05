import th from "../th/manualAnnouncements";
const en = {
  title: "Administrator announcements", description: "Administrator-written notices based on cited news, separate from official agency announcements and automatic alerts.",
  publisher: "Published by an administrator · Cites media reports", disclaimer: "Compiled from cited news. This is not a highway authority road-closure notice or a system-verified live water measurement.",
  preview: "Preview — these changes have not been published", untitled: "Title not supplied", original: "Original content · Not automatically translated",
  DRAFT: "Draft", PUBLISHED: "Published", EXPIRED: "Expired", WITHDRAWN: "Withdrawn",
  area: "Area", observedAt: "Reported situation time", publishedAt: "Published at", updatedAt: "Announcement updated at", expiresAt: "Expires at", withdrawnAt: "Withdrawn at", recordedAt: "Manually recorded at",
  referenceTime: "Source report time", references: "References", details: "Details and references", archiveDetails: "Read historical details and references (not current conditions)", advice: "Advice from the cited reports",
  unknown: "This information does not confirm current road or water conditions.", expired: "Historical information — no new report available", withdrawn: "This announcement has been withdrawn and is not a current status.", reason: "Withdrawal reason",
  previousSeverity: "Original administrator-assigned level: {severity}", INFO: "Information", WATCH: "Watch", WARNING: "Warning", CRITICAL: "Critical",
  empty: "No administrator announcements yet", noSafety: "The absence of announcements does not confirm that an area or road is safe.", loading: "Loading administrator announcements", unavailable: "Unable to load administrator announcements",
  unavailableHelp: "Information is unavailable. Current conditions are unknown. Please try again.", retry: "Retry administrator announcements", refresh: "Refresh administrator announcements", previous: "Previous administrator announcements page", next: "Next administrator announcements page", pagination: "Administrator announcement pages", page: "Showing {from}–{to} of {total} announcements"
} satisfies Record<keyof typeof th, string>;
export default en;
