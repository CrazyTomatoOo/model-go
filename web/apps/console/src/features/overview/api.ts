import { useEffect, useState } from 'react';

/**
 * Overview snapshot client (#37). The backend has no overview endpoint yet,
 * so the client resolves null and every card renders its designed empty
 * state. The types below ARE the API contract: when the overview API ships,
 * implement fetchOverviewSnapshot against lib/request's apiFetch and the
 * cards pick up real data without any UI changes.
 */

export type GpuStatus = 'online' | 'offline';

/** One accelerator as the overview API will describe it. */
export type GpuNode = {
  id: string;
  name: string;
  status: GpuStatus;
  driverVersion: string;
};

export type ResourceStats = {
  models: number;
  datasets: number;
  trainingJobs: number;
};

export type OverviewSnapshot = {
  gpus: GpuNode[];
  resources: ResourceStats;
};

/**
 * No overview API exists yet — resolve null so the page renders its empty
 * states. Replace with a real apiFetch('/api/v1/…') call when the endpoint
 * lands; the UI already handles both branches.
 */
export async function fetchOverviewSnapshot(): Promise<OverviewSnapshot | null> {
  return null;
}

/**
 * Loads the snapshot once per mount. Absent data (initial state or a null
 * client result) is represented as null — the cards render their designed
 * empty states in both cases, so there is no loading flash.
 */
export function useOverviewSnapshot(): OverviewSnapshot | null {
  const [snapshot, setSnapshot] = useState<OverviewSnapshot | null>(null);

  useEffect(() => {
    let cancelled = false;
    void fetchOverviewSnapshot().then((data) => {
      if (!cancelled) {
        setSnapshot(data);
      }
    });
    return () => {
      cancelled = true;
    };
  }, []);

  return snapshot;
}
