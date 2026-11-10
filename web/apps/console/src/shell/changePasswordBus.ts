import { useEffect } from 'react';

/**
 * T3/T4 integration seam for the change-password modal (#36).
 *
 * T4 wires every chrome entry point (user dropdown, side menu, mobile drawer)
 * to `requestChangePassword()`. T3 builds the change-password modal in
 * parallel and completes the integration by mounting a listener:
 *
 * ```tsx
 * function ChangePasswordModalHost() {
 *   const [open, setOpen] = useState(false);
 *   useChangePasswordRequest(() => setOpen(true));
 *   return <ChangePasswordModal open={open} onClose={() => setOpen(false)} />;
 * }
 * ```
 *
 * Until a listener exists (T3 not merged yet), `requestChangePassword()`
 * returns false so callers can degrade gracefully — the shell falls back to
 * the standalone /change-password route in that case.
 *
 * This is a dependency-free module so either ticket can land first without a
 * merge conflict on shared state.
 */

type ChangePasswordListener = () => void;

const listeners = new Set<ChangePasswordListener>();

/** Notify all listeners. Returns true when a listener consumed the request. */
export function requestChangePassword(): boolean {
  if (listeners.size === 0) {
    return false;
  }
  listeners.forEach((listener) => listener());
  return true;
}

/** Subscribe to change-password requests; unsubscribes on unmount. */
export function useChangePasswordRequest(listener: ChangePasswordListener): void {
  useEffect(() => {
    listeners.add(listener);
    return () => {
      listeners.delete(listener);
    };
  }, [listener]);
}
