import type { ReactNode } from "react";

type HeroKind = "water" | "map" | "risk" | "announcement" | "report";

function HeroIcon({ kind }: { kind: HeroKind }) {
  const paths: Record<HeroKind, ReactNode> = {
    water: <><path d="M3 7c3-4 6 4 9 0s6 4 9 0M3 12c3-4 6 4 9 0s6 4 9 0M3 17c3-4 6 4 9 0s6 4 9 0" /></>,
    map: <><path d="m3 5 6-2 6 2 6-2v16l-6 2-6-2-6 2V5Z" /><path d="M9 3v16M15 5v16" /></>,
    risk: <><path d="m12 3 10 18H2L12 3Z" /><path d="M12 9v5M12 17h.01" /></>,
    announcement: <><path d="m4 10 14-5v14L4 14v-4ZM18 9l3-1v8l-3-1M7 15l2 5h3l-2-4" /></>,
    report: <><rect x="5" y="3" width="14" height="18" rx="2" /><path d="M9 8h6M9 12h6M9 16h4" /></>
  };
  return <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{paths[kind]}</svg>;
}

export function PageHeroArt({ kind }: { kind: HeroKind }) {
  return <div className="page-hero__art" aria-hidden="true">
    <span className="page-hero__tile page-hero__tile--small"><HeroIcon kind={kind === "water" ? "map" : "water"} /></span>
    <span className="page-hero__tile page-hero__tile--large"><HeroIcon kind={kind} /></span>
    <span className="page-hero__tile page-hero__tile--small"><HeroIcon kind={kind === "report" ? "announcement" : "report"} /></span>
  </div>;
}
