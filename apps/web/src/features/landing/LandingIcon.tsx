export function LandingIcon({ kind }: { kind: "map" | "building" | "people" | "ai" | "bell" | "cloud" | "expand" }) {
  return <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    {kind === "map" && <><path d="m3 5 6-2 6 2 6-2v16l-6 2-6-2-6 2z" /><path d="M9 3v16M15 5v16" /></>}
    {kind === "building" && <><path d="M4 21V3h12v18M16 9h4v12M2 21h20M8 7h1m3 0h1M8 11h1m3 0h1M8 15h1m3 0h1M9 21v-3h3v3" /></>}
    {kind === "people" && <><circle cx="12" cy="7" r="3" /><circle cx="4" cy="8" r="2" /><circle cx="20" cy="8" r="2" /><path d="M7 21v-4a5 5 0 0 1 10 0v4M1 20v-4a3 3 0 0 1 4-3M23 20v-4a3 3 0 0 0-4-3M11 21v-4m2 4v-4" /></>}
    {kind === "ai" && <><rect x="6" y="6" width="12" height="12" rx="2" /><path d="M9 2v4m6-4v4M9 18v4m6-4v4M2 9h4m-4 6h4M18 9h4m-4 6h4" /><rect x="10" y="10" width="4" height="4" rx=".5" /></>}
    {kind === "bell" && <><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9M10 21h4M12 1v1M3 4 1 6m20-2 2 2" /></>}
    {kind === "cloud" && <><path d="M6 15a4 4 0 0 1-1-8 6 6 0 0 1 11-1 4.5 4.5 0 1 1 1 9Z" /><path d="m7 18-1 3m6-3-1 3m6-3-1 3" /></>}
    {kind === "expand" && <path d="M9 3H3v6M15 3h6v6M21 15v6h-6M9 21H3v-6" />}
  </svg>;
}
