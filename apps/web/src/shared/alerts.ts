import { translate } from "../i18n";
import th from "../i18n/locales/th/common";
import en from "../i18n/locales/en/common";
const messages = { th, en };
const t = (key: keyof typeof th) => translate(messages, key);
import Swal from "sweetalert2";
import type { SweetAlertIcon, SweetAlertOptions } from "sweetalert2";
import "sweetalert2/dist/sweetalert2.min.css";
import "./alerts.css";

const appearance = (): SweetAlertOptions => ({
  buttonsStyling: false,
  confirmButtonText: t("ok"),
  cancelButtonText: t("cancel"),
  reverseButtons: true,
  returnFocus: true,
  heightAuto: false,
  customClass: {
    container: "fw-alert-container",
    popup: "fw-alert",
    icon: "fw-alert__icon",
    title: "fw-alert__title",
    htmlContainer: "fw-alert__message",
    actions: "fw-alert__actions",
    confirmButton: "fw-alert__button fw-alert__button--confirm",
    cancelButton: "fw-alert__button fw-alert__button--cancel",
    loader: "fw-alert__loader"
  }
});

let pending: Promise<void> = Promise.resolve();

function enqueue<Result>(task: () => Promise<Result>): Promise<Result> {
  const result = pending.then(task);
  pending = result.then(() => undefined, () => undefined);
  return result;
}

async function message(icon: SweetAlertIcon, title: string, text?: string): Promise<void> {
  await Swal.fire({ ...appearance(), icon, titleText: title, text });
}

export const notifySuccess = (title: string, text?: string) => enqueue(() => message("success", title, text));
export const notifyError = (title: string, text?: string) => enqueue(() => message("error", title, text));
export const notifyInfo = (title: string, text?: string) => enqueue(() => message("info", title, text));
export const notifyWarning = (title: string, text?: string) => enqueue(() => message("warning", title, text));

type ConfirmActionOptions = {
  title: string;
  text: string;
  confirmText: string;
  cancelText?: string;
  danger?: boolean;
  action: () => Promise<void>;
  successTitle: string;
  successText?: string;
  errorTitle?: string;
  errorText?: string | ((cause: unknown) => string);
};

export function confirmAction(options: ConfirmActionOptions): Promise<boolean> {
  return enqueue(async () => {
    let failure: unknown;
    let failed = false;
    let completed = false;
    const result = await Swal.fire({
      ...appearance(),
      icon: "warning",
      titleText: options.title,
      text: options.text,
      showCancelButton: true,
      focusCancel: true,
      focusConfirm: false,
      confirmButtonText: options.confirmText,
      cancelButtonText: options.cancelText ?? t("cancel"),
      customClass: {
        ...appearance().customClass,
        confirmButton: `fw-alert__button fw-alert__button--${options.danger ? "danger" : "confirm"}`
      },
      showLoaderOnConfirm: true,
      allowOutsideClick: () => !Swal.isLoading(),
      allowEscapeKey: () => !Swal.isLoading(),
      preConfirm: async () => {
        Swal.disableButtons();
        Swal.update({ titleText: t("working") });
        try {
          await options.action();
          completed = true;
        } catch (cause) {
          failed = true;
          failure = cause;
        }
        return { completed };
      }
    });
    if (failed) {
      const text = typeof options.errorText === "function" ? options.errorText(failure) : options.errorText;
      await message("error", options.errorTitle ?? t("couldNotCompleteThisAction"), text ?? t("pleaseTryAgain"));
      return false;
    }
    if (!result.isConfirmed || !completed) return false;
    await message("success", options.successTitle, options.successText);
    return true;
  });
}
