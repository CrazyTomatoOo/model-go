import { useState } from 'react';
import { useChangePasswordRequest } from '../../shell/changePasswordBus';
import ChangePasswordModal from './ChangePasswordModal';

/**
 * Mounts the change-password modal and subscribes it to the shell's
 * change-password bus (#36 seam): every chrome entry point (user dropdown,
 * side menu, mobile drawer) fires `requestChangePassword()`, which this host
 * turns into an open modal. Rendering the host is what makes the bus
 * "consumed" — without it the shell falls back to the /change-password route.
 */
export default function ChangePasswordModalHost() {
  const [open, setOpen] = useState(false);
  useChangePasswordRequest(() => setOpen(true));
  return <ChangePasswordModal open={open} onClose={() => setOpen(false)} />;
}
