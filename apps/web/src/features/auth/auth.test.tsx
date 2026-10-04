import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AuthProvider, useAuth } from "./AuthProvider";
import { accountApi, apiClient, ApiClient, createAuthApi, createPasswordRecoveryApi } from "./api";
import { RequireAdmin, RequireAuth } from "./RequireAuth";

const account = {
  id: "account-1",
  email: "user@example.com",
  role: "USER" as const,
  createdAt: "2026-10-02T00:00:00Z"
};

function response(body: unknown, status = 200): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText: "Request failed",
    json: vi.fn().mockResolvedValue(body)
  } as unknown as Response;
}

function createTestAuthApi() {
  return createAuthApi(new ApiClient());
}

function AccountSummary() {
  const { account: currentAccount, logout, status } = useAuth();

  return (
    <>
      <p>{status}</p>
      {currentAccount ? <p>{currentAccount.email}</p> : null}
      <button type="button" onClick={() => void logout()}>
        Sign out
      </button>
    </>
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("AuthProvider", () => {
  it("does not restore a logged-out account after a pending username update", async () => {
    let resolveSave!: (value: typeof account) => void;
    const updateUsername = vi.fn(() => new Promise<typeof account>((resolve) => { resolveSave = resolve; }));
    const api = { refresh: vi.fn().mockResolvedValue(undefined), me: vi.fn().mockResolvedValue(account), login: vi.fn(), logout: vi.fn().mockResolvedValue(undefined), register: vi.fn() };
    function PendingAccount() {
      const current = useAuth();
      return <><p>{current.status}</p><button onClick={() => void current.updateUsername("new_user")}>Rename</button><button onClick={() => void current.logout()}>Logout</button></>;
    }
    render(<AuthProvider api={api} accountApi={{ updateUsername }}><PendingAccount /></AuthProvider>);
    await screen.findByText("authenticated");
    fireEvent.click(screen.getByRole("button", { name: "Rename" }));
    fireEvent.click(screen.getByRole("button", { name: "Logout" }));
    await screen.findByText("anonymous");
    await act(async () => { resolveSave({ ...account }); });
    expect(screen.getByText("anonymous")).toBeInTheDocument();
    expect(screen.queryByText("authenticated")).not.toBeInTheDocument();
  });

  it("bootstraps by refreshing the session before loading the account", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(response({ user: account, accessToken: "access-token", expiresIn: 900 }))
      .mockResolvedValueOnce(response({ user: account }));
    vi.stubGlobal("fetch", fetchMock);

    render(
      <AuthProvider api={createTestAuthApi()}>
        <AccountSummary />
      </AuthProvider>
    );

    expect(await screen.findByText(account.email)).toBeInTheDocument();
    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/v1/auth/refresh",
      expect.objectContaining({ credentials: "include", method: "POST" })
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/v1/auth/me",
      expect.objectContaining({ headers: expect.any(Headers) })
    );
    expect((fetchMock.mock.calls[1][1] as RequestInit).headers).toEqual(
      expect.objectContaining({ get: expect.any(Function) })
    );
    expect(((fetchMock.mock.calls[1][1] as RequestInit).headers as Headers).get("Authorization")).toBe(
      "Bearer access-token"
    );
  });

  it("clears the in-memory session after logout", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(response({ user: account, accessToken: "access-token", expiresIn: 900 }))
      .mockResolvedValueOnce(response({ user: account }))
      .mockResolvedValueOnce(response(undefined, 204));
    vi.stubGlobal("fetch", fetchMock);

    render(
      <AuthProvider api={createTestAuthApi()}>
        <AccountSummary />
      </AuthProvider>
    );

    await screen.findByText(account.email);
    fireEvent.click(screen.getByRole("button", { name: "Sign out" }));

    await waitFor(() => expect(screen.getByText("anonymous")).toBeInTheDocument());
    expect(fetchMock).toHaveBeenLastCalledWith(
      "/api/v1/auth/logout",
      expect.objectContaining({ credentials: "include", method: "POST" })
    );
  });
});

describe("ApiClient", () => {
  it("never refreshes and replays a username mutation after an unauthorized response", async () => {
    const fetchMock = vi.fn().mockResolvedValue(response({ error: { code: "UNAUTHENTICATED", message: "expired" } }, 401));
    vi.stubGlobal("fetch", fetchMock);
    const refresh = vi.fn().mockResolvedValue(true);
    apiClient.setUnauthorizedHandler(refresh);
    try {
      await expect(accountApi.updateUsername("new_user")).rejects.toMatchObject({ status: 401 });
      expect(refresh).not.toHaveBeenCalled();
      expect(fetchMock).toHaveBeenCalledOnce();
    } finally {
      createAuthApi(apiClient);
      apiClient.clearAccessToken();
    }
  });

  it("updates only the authenticated username using the shared API client", async () => {
    const updated = { ...account, username: "new_user" };
    const fetchMock = vi.fn().mockResolvedValue(response({ user: updated }));
    vi.stubGlobal("fetch", fetchMock);
    apiClient.setAccessToken("test-token");
    try {
      await expect(accountApi.updateUsername("new_user")).resolves.toEqual(updated);
      expect(fetchMock).toHaveBeenCalledExactlyOnceWith("/api/v1/auth/me", expect.objectContaining({ method: "PATCH", body: JSON.stringify({ username: "new_user" }) }));
      const headers = fetchMock.mock.calls[0][1].headers as Headers;
      expect(headers.get("Authorization")).toBe("Bearer test-token");
      expect(headers.get("Content-Type")).toBe("application/json");
    } finally {
      apiClient.clearAccessToken();
    }
  });

  it("uses the unauthenticated password recovery contract for all three steps", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(response({ challengeId: "challenge-1", expiresIn: 600, resendAfter: 60, message: "generic" }))
      .mockResolvedValueOnce(response({ resetToken: "reset-1", expiresIn: 300 }))
      .mockResolvedValueOnce(response(undefined, 204));
    vi.stubGlobal("fetch", fetchMock);
    const api = createPasswordRecoveryApi(new ApiClient());

    await expect(api.requestOtp("user@example.com")).resolves.toEqual(expect.objectContaining({ challengeId: "challenge-1" }));
    await expect(api.verifyOtp({ challengeId: "challenge-1", otp: "123456" })).resolves.toEqual({ resetToken: "reset-1", expiresIn: 300 });
    await expect(api.resetPassword({ resetToken: "reset-1", password: "password1234" })).resolves.toBeUndefined();

    expect(fetchMock).toHaveBeenNthCalledWith(1, "/api/v1/auth/password/request-otp", expect.objectContaining({ method: "POST", credentials: "same-origin" }));
    expect(fetchMock).toHaveBeenNthCalledWith(2, "/api/v1/auth/password/verify-otp", expect.objectContaining({ method: "POST", credentials: "same-origin" }));
    expect(fetchMock).toHaveBeenNthCalledWith(3, "/api/v1/auth/password/reset", expect.objectContaining({ method: "POST", credentials: "same-origin" }));
    expect(JSON.parse(String(fetchMock.mock.calls[1][1].body))).toEqual({ challengeId: "challenge-1", otp: "123456" });
  });

  it("does not refresh an unauthenticated recovery request after 401", async () => {
    const fetchMock = vi.fn().mockResolvedValue(response({ error: { code: "INVALID_REQUEST", message: "invalid" } }, 401));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient();
    const refresh = vi.fn().mockResolvedValue(true);
    client.setUnauthorizedHandler(refresh);

    await expect(createPasswordRecoveryApi(client).requestOtp("user@example.com")).rejects.toMatchObject({ code: "INVALID_REQUEST" });
    expect(refresh).not.toHaveBeenCalled();
  });

  it("shares one refresh and retries each unauthorized request once", async () => {
    const fetchMock = vi.fn((path: string, options: RequestInit) => {
      const authorized = (options.headers as Headers).get("Authorization") === "Bearer renewed-token";
      const requestedPath = path.replace("/api/v1", "");
      return Promise.resolve(authorized ? response({ path: requestedPath }) : response({ error: { code: "UNAUTHORIZED", message: "Expired" } }, 401));
    });
    vi.stubGlobal("fetch", fetchMock);

    const client = new ApiClient();
    let releaseRefresh: (() => void) | undefined;
    const refreshStarted = new Promise<void>((resolve) => {
      releaseRefresh = resolve;
    });
    const refresh = vi.fn(async () => {
      await refreshStarted;
      client.setAccessToken("renewed-token");
      return true;
    });
    client.setUnauthorizedHandler(refresh);

    const first = client.request<{ path: string }>("/protected/first");
    const second = client.request<{ path: string }>("/protected/second");

    await waitFor(() => expect(refresh).toHaveBeenCalledTimes(1));
    releaseRefresh?.();

    await expect(Promise.all([first, second])).resolves.toEqual([
      { path: "/protected/first" },
      { path: "/protected/second" }
    ]);
    expect(fetchMock).toHaveBeenCalledTimes(4);
  });
});

describe("auth route guards", () => {
  it("shows a loading state while session bootstrap is pending", () => {
    vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>(() => undefined)));

    render(
      <MemoryRouter>
        <AuthProvider api={createTestAuthApi()}>
          <RequireAuth>
            <p>Private content</p>
          </RequireAuth>
        </AuthProvider>
      </MemoryRouter>
    );

    expect(screen.getByRole("status")).toHaveTextContent("กำลังตรวจสอบเซสชัน");
  });

  it("shows anonymous and forbidden states before protected content", async () => {
    const fetchMock = vi.fn().mockResolvedValue(response({ error: { code: "UNAUTHORIZED", message: "Missing" } }, 401));
    vi.stubGlobal("fetch", fetchMock);

    const anonymousView = render(
      <MemoryRouter>
        <AuthProvider api={createTestAuthApi()}>
          <RequireAuth>
            <p>Private content</p>
          </RequireAuth>
        </AuthProvider>
      </MemoryRouter>
    );

    expect(await screen.findByRole("alert")).toHaveTextContent("กรุณาเข้าสู่ระบบเพื่อดำเนินการต่อ");
    anonymousView.unmount();

    fetchMock.mockReset()
      .mockResolvedValueOnce(response({ user: account, accessToken: "access-token", expiresIn: 900 }))
      .mockResolvedValueOnce(response({ user: account }));

    render(
      <MemoryRouter>
        <AuthProvider api={createTestAuthApi()}>
          <RequireAdmin>
            <p>Admin content</p>
          </RequireAdmin>
        </AuthProvider>
      </MemoryRouter>
    );

    expect(await screen.findByRole("alert")).toHaveTextContent("ไม่มีสิทธิ์ดูหน้านี้");
    expect(screen.queryByText("Admin content")).not.toBeInTheDocument();
  });
});
