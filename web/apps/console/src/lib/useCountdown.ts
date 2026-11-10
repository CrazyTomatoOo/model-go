import { useEffect, useState } from 'react';

/**
 * One-second ticker for a rate-limit style cooldown. `windowSeconds` is the
 * countdown window once it becomes known (null while inactive); the returned
 * value is the seconds left, 0 once expired. The value is derived during
 * render, so the first render after arming already shows the full window.
 * Call sites must route the window back through null between countdowns (the
 * alert state clears itself at zero) so re-arming resets the elapsed count.
 */
export default function useCountdown(windowSeconds: number | null): number {
  const [elapsed, setElapsed] = useState(0);

  useEffect(() => {
    setElapsed(0);
  }, [windowSeconds]);

  const active = windowSeconds !== null && elapsed < windowSeconds;
  useEffect(() => {
    if (!active) {
      return;
    }
    const id = window.setInterval(() => {
      setElapsed((s) => s + 1);
    }, 1000);
    return () => window.clearInterval(id);
  }, [active]);

  return windowSeconds === null ? 0 : Math.max(0, windowSeconds - elapsed);
}
