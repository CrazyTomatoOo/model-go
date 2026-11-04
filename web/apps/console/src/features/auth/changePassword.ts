import { ApiError, apiFetch } from '../../lib/request';

export type ChangePasswordPayload = {
  readonly oldPassword: string;
  readonly newPassword: string;
};

export type ChangePasswordErrorCode =
  | 'invalidInitialPassword'
  | 'policyViolation'
  | 'network'
  | 'unknown';

export class ChangePasswordError extends Error {
  readonly name = 'ChangePasswordError';

  constructor(readonly code: ChangePasswordErrorCode) {
    super(code);
  }
}

export async function changePassword(payload: ChangePasswordPayload): Promise<void> {
  try {
    await apiFetch<unknown>('/api/v1/auth/password', {
      method: 'POST',
      body: { old_password: payload.oldPassword, new_password: payload.newPassword },
    });
  } catch (error) {
    // apiFetch (lib/request.ts) parses the response body unconditionally. The password
    // endpoint answers 204 with an empty body (login-design spec §4.1), which surfaces
    // here as a SyntaxError and means the change succeeded. The wrapper is frozen, so
    // the normalization lives in this module.
    if (error instanceof SyntaxError) return;
    if (error instanceof ApiError) {
      if (error.status === 401) throw new ChangePasswordError('invalidInitialPassword');
      if (error.status === 400) throw new ChangePasswordError('policyViolation');
      throw new ChangePasswordError('unknown');
    }
    // Transport-level failure (connection, DNS, timeout): no HTTP status involved.
    throw new ChangePasswordError('network');
  }
}
