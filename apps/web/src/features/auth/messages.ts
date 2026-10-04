import { translate } from "../../i18n";
import en from "../../i18n/locales/en/auth";
import th from "../../i18n/locales/th/auth";

export const authMessages = { th, en };
export type AuthMessageKey = keyof typeof th;
export type AuthMessageParams = Record<string, string | number>;
export type AuthMessage = {
  key: AuthMessageKey;
  params?: AuthMessageParams;
  cause?: AuthMessage;
};

export function authMessage(key: AuthMessageKey, params?: AuthMessageParams, cause?: AuthMessage): AuthMessage {
  return { key, params, cause };
}

export function formatAuthMessage(message: AuthMessage, t: (key: AuthMessageKey, params?: AuthMessageParams) => string): string {
  const params = { ...message.params };
  if (message.cause) params.message = formatAuthMessage(message.cause, t);
  return t(message.key, params);
}

export function translateAuthMessage(message: AuthMessage): string {
  return formatAuthMessage(message, (key, params) => translate(authMessages, key, params));
}
