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
    // Backend contract (spec §4.5): a wrong old password and a policy
    // violation are both 400 VALIDATION_ERROR, distinguished by
    // errors[].field (old_password vs new_password). 401 on this endpoint is
    // SESSION_EXPIRED, handled globally by apiFetch.
    if (error instanceof ApiError) {
      if (error.code === 'VALIDATION_ERROR') {
        const field = error.errors?.[0]?.field;
        if (field === 'old_password') throw new ChangePasswordError('invalidInitialPassword');
        if (field === 'new_password') throw new ChangePasswordError('policyViolation');
      }
      throw new ChangePasswordError('unknown');
    }
    // Transport-level failure (connection, DNS, timeout): no HTTP status involved.
    throw new ChangePasswordError('network');
  }
}
