import { getCsrfToken, setCsrfToken } from './auth';

export type FieldError = {
  field?: string;
  message?: string;
};

export class ApiError extends Error {
  readonly status: number;
  readonly code: string | null;
  /** Field-level details carried by 400 VALIDATION_ERROR responses (§4.5). */
  readonly errors: FieldError[] | null;

  constructor(
    status: number,
    message: string,
    code: string | null = null,
    errors: FieldError[] | null = null,
  ) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.errors = errors;
  }
}

export type ApiRequestOptions = {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  body?: Record<string, unknown>;
  csrfToken?: string | null;
};

type JsonObject = Record<string, unknown>;

async function readError(response: Response): Promise<ApiError> {
  const raw = await response.text();
  let message: string | null = null;
  let code: string | null = null;
  let errors: FieldError[] | null = null;
  if (raw) {
    try {
      const parsed: unknown = JSON.parse(raw);
      if (typeof parsed === 'object' && parsed !== null) {
        const obj = parsed as JsonObject;
        if (typeof obj.message === 'string') {
          message = obj.message;
        }
        if (typeof obj.code === 'string') {
          code = obj.code;
        }
        if (Array.isArray(obj.errors)) {
          errors = obj.errors.map((entry) => {
            if (typeof entry !== 'object' || entry === null) {
              return {};
            }
            const fe = entry as JsonObject;
            return {
              field: typeof fe.field === 'string' ? fe.field : undefined,
              message: typeof fe.message === 'string' ? fe.message : undefined,
            };
          });
        }
      }
    } catch {
      // non-JSON error body; fall back to the generic message below
    }
  }
  return new ApiError(
    response.status,
    message ?? `Request failed: ${response.status}`,
    code,
    errors,
  );
}

/**
 * Auth-level failures that invalidate the whole session are forwarded to a
 * single global handler (registered by AuthProvider) before the rejection is
 * thrown: SESSION_EXPIRED logs the user out, PASSWORD_CHANGE_REQUIRED sends
 * them to the forced password change page.
 */
export type GlobalErrorHandler = (error: ApiError, path: string) => void;

let onGlobalError: GlobalErrorHandler | null = null;

export function setOnGlobalError(handler: GlobalErrorHandler | null): void {
  onGlobalError = handler;
}

export async function apiFetch<T>(path: string, options: ApiRequestOptions = {}): Promise<T> {
  return apiFetchInner<T>(path, options, false);
}

async function apiFetchInner<T>(
  path: string,
  options: ApiRequestOptions,
  retried: boolean,
): Promise<T> {
  const { method = 'GET', body } = options;
  // Spec §4.1: X-CSRF-Token must match the session's token on state-changing requests.
  const csrfToken = options.csrfToken !== undefined ? options.csrfToken : getCsrfToken();

  const headers: Record<string, string> = {};
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json';
  }
  if (method !== 'GET' && csrfToken !== null) {
    headers['X-CSRF-Token'] = csrfToken;
  }

  const response = await fetch(path, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });

  if (!response.ok) {
    const error = await readError(response);
    if (!retried && error.status === 403 && error.code === 'CSRF_TOKEN_INVALID') {
      // Spec §4 全局约定: a stale CSRF token is refreshed via GET /auth/me and
      // the original request is retried exactly once.
      const refreshResponse = await fetch('/api/v1/auth/me');
      if (refreshResponse.ok) {
        const data = (await refreshResponse.json()) as { csrf_token?: unknown };
        if (typeof data.csrf_token === 'string') {
          setCsrfToken(data.csrf_token);
          return apiFetchInner<T>(path, { ...options, csrfToken: undefined }, true);
        }
      }
    }
    if (error.code === 'SESSION_EXPIRED' || error.code === 'PASSWORD_CHANGE_REQUIRED') {
      onGlobalError?.(error, path);
    }
    throw error;
  }

  // 204 (e.g. POST /auth/logout) carries no body.
  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
}
