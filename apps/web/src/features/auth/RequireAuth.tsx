import type { ReactNode } from "react";
import { useAuth } from "./AuthProvider";

type RequireAuthProps = {
  children: ReactNode;
};

export function RequireAuth({ children }: RequireAuthProps) {
  const { status } = useAuth();

  if (status === "loading") {
    return <p role="status">Checking your session...</p>;
  }

  if (status === "anonymous") {
    return <p role="alert">Sign in is required to continue.</p>;
  }

  return <>{children}</>;
}

export function RequireAdmin({ children }: RequireAuthProps) {
  const { account, status } = useAuth();

  if (status === "loading") {
    return <p role="status">Checking your session...</p>;
  }

  if (status === "anonymous") {
    return <p role="alert">Sign in is required to continue.</p>;
  }

  if (account?.role !== "ADMIN") {
    return <p role="alert">You do not have permission to view this page.</p>;
  }

  return <>{children}</>;
}
