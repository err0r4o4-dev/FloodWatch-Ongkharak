import { apiConfig } from "../../shared/config/api";
import type { Account, Credentials, RegisterInput } from "./types";

type ApiErrorPayload = {
  error?: {
    code?: string;
    message?: string;
  };
};

type RequestOptions = {
  authenticate?: boolean;
  retryUnauthorized?: boolean;
};

type AuthSession = {
  user: Account;
  accessToken: string;
  expiresIn: number;
};

type AuthApi = {
  register(input: RegisterInput): Promise<Account>;
  login(input: Credentials): Promise<AuthSession>;
  refresh(): Promise<AuthSession>;
  me(): Promise<Account>;
  logout(): Promise<void>;
};

export type AccountApi = {
  updateUsername(username: string): Promise<Account>;
};

type RequestOtpResponse = {
  challengeId: string;
  expiresIn: number;
  resendAfter: number;
  message: string;
};

type VerifyOtpResponse = {
  resetToken: string;
  expiresIn: number;
};

export type PasswordRecoveryApi = {
  requestOtp(email: string): Promise<RequestOtpResponse>;
  verifyOtp(input: { challengeId: string; otp: string }): Promise<VerifyOtpResponse>;
  resetPassword(input: { resetToken: string; password: string }): Promise<void>;
};

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly code?: string
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export class ApiClient {
  private accessToken: string | null = null;
  private refreshInFlight: Promise<boolean> | null = null;
  private unauthorizedHandler: (() => Promise<boolean>) | null = null;

  setAccessToken(accessToken: string | null) {
    this.accessToken = accessToken;
  }

  clearAccessToken() {
    this.accessToken = null;
  }

  setUnauthorizedHandler(handler: () => Promise<boolean>) {
    this.unauthorizedHandler = handler;
  }

  async request<ResponseBody>(
    path: string,
    init: RequestInit = {},
    options: RequestOptions = {}
  ): Promise<ResponseBody> {
    const response = await this.send(path, init, options.authenticate ?? true);

    if (response.status === 401 && options.retryUnauthorized !== false) {
      const refreshed = await this.refreshOnce();
      if (refreshed) {
        return this.request(path, init, {
          ...options,
          retryUnauthorized: false
        });
      }
    }

    if (!response.ok) {
      throw await ApiClient.toError(response);
    }

    if (response.status === 204) {
      return undefined as ResponseBody;
    }

    return (await response.json()) as ResponseBody;
  }

  async requestBlob(path: string, init: RequestInit = {}, options: RequestOptions = {}): Promise<Blob> {
    const response = await this.send(path, init, options.authenticate ?? true);

    if (response.status === 401 && options.retryUnauthorized !== false) {
      const refreshed = await this.refreshOnce();
      if (refreshed) {
        return this.requestBlob(path, init, { ...options, retryUnauthorized: false });
      }
    }

    if (!response.ok) {
      throw await ApiClient.toError(response);
    }
    return response.blob();
  }

  private async send(path: string, init: RequestInit, authenticate: boolean) {
    const headers = new Headers(init.headers);
    if (authenticate && this.accessToken) {
      headers.set("Authorization", `Bearer ${this.accessToken}`);
    }

    const url = path.startsWith("/api/") ? path : `${apiConfig.baseUrl}${path}`;
    return fetch(url, {
      ...init,
      credentials: init.credentials ?? "same-origin",
      headers
    });
  }

  private async refreshOnce() {
    if (!this.unauthorizedHandler) {
      return false;
    }

    if (!this.refreshInFlight) {
      this.refreshInFlight = this.unauthorizedHandler()
        .catch(() => false)
        .finally(() => {
          this.refreshInFlight = null;
        });
    }

    return this.refreshInFlight;
  }

  private static async toError(response: Response) {
    const payload = await response.json().catch(() => undefined) as ApiErrorPayload | undefined;
    const message = payload?.error?.message || response.statusText || "Request failed.";
    return new ApiError(message, response.status, payload?.error?.code);
  }
}

export function createAuthApi(client: ApiClient): AuthApi {
  let refreshInFlight: Promise<AuthSession> | null = null;

  const refresh = async () => {
    if (!refreshInFlight) {
      refreshInFlight = client.request<AuthSession>(
        "/auth/refresh",
        { credentials: "include", method: "POST" },
        { authenticate: false, retryUnauthorized: false }
      )
        .then((session) => {
          client.setAccessToken(session.accessToken);
          return session;
        })
        .catch((error: unknown) => {
          client.clearAccessToken();
          throw error;
        })
        .finally(() => {
          refreshInFlight = null;
        });
    }

    return refreshInFlight;
  };

  const authApi: AuthApi = {
    async register(input) {
      const response = await client.request<{ user: Account }>(
        "/auth/register",
        {
          body: JSON.stringify(input),
          headers: { "Content-Type": "application/json" },
          method: "POST"
        },
        { authenticate: false, retryUnauthorized: false }
      );
      return response.user;
    },

    async login(input) {
      const session = await client.request<AuthSession>(
        "/auth/login",
        {
          body: JSON.stringify(input),
          credentials: "include",
          headers: { "Content-Type": "application/json" },
          method: "POST"
        },
        { authenticate: false, retryUnauthorized: false }
      );
      client.setAccessToken(session.accessToken);
      return session;
    },

    refresh,

    async me() {
      const response = await client.request<{ user: Account }>("/auth/me");
      return response.user;
    },

    async logout() {
      try {
        await client.request<void>(
          "/auth/logout",
          { credentials: "include", method: "POST" },
          { retryUnauthorized: false }
        );
      } finally {
        client.clearAccessToken();
      }
    }
  };

  client.setUnauthorizedHandler(async () => {
    try {
      await refresh();
      return true;
    } catch {
      return false;
    }
  });

  return authApi;
}

export function createPasswordRecoveryApi(client: ApiClient): PasswordRecoveryApi {
  const request = <ResponseBody>(path: string, body: unknown) => client.request<ResponseBody>(
    path,
    {
      body: JSON.stringify(body),
      headers: { "Content-Type": "application/json" },
      method: "POST"
    },
    { authenticate: false, retryUnauthorized: false }
  );

  return {
    requestOtp(email) {
      return request<RequestOtpResponse>("/auth/password/request-otp", { email });
    },
    verifyOtp(input) {
      return request<VerifyOtpResponse>("/auth/password/verify-otp", input);
    },
    resetPassword(input) {
      return request<void>("/auth/password/reset", input);
    }
  };
}

export const apiClient = new ApiClient();
export const authApi = createAuthApi(apiClient);
export const accountApi: AccountApi = {
  async updateUsername(username) {
    const response = await apiClient.request<{ user: Account }>("/auth/me", {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username })
    }, { retryUnauthorized: false });
    return response.user;
  }
};
export const passwordRecoveryApi = createPasswordRecoveryApi(apiClient);

export type { AuthApi, AuthSession, RequestOtpResponse, VerifyOtpResponse };
