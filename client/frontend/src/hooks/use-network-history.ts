import { useEffect, useRef, useState } from 'react';

export type NetworkSample = {
    /** Epoch ms of the sampling instant. */
    t: number;
    /** Aggregate download rate across the nodes, in KB/s. */
    in: number;
    out: number;
};

export type NetworkTotals = Omit<NetworkSample, 't'>;

/** Live cadence while the dashboard stays open. */
export const NETWORK_SAMPLE_INTERVAL_MS = 5_000;
/** The persisted copy keeps one point per minute so 6 h / 24 h ranges survive reloads. */
const PERSIST_INTERVAL_MS = 60_000;
const RETENTION_MS = 24 * 60 * 60 * 1000;
const STORAGE_KEY = 'aether.dashboard.network-history.v1';

const isSample = (value: unknown): value is NetworkSample => {
    if (!value || typeof value !== 'object') return false;
    const sample = value as Record<string, unknown>;
    return (
        typeof sample.t === 'number' &&
        Number.isFinite(sample.t) &&
        typeof sample.in === 'number' &&
        typeof sample.out === 'number'
    );
};

const rate = (value: number) => Math.max(0, Number(value.toFixed(2)));

/**
 * Collapses a series into time buckets, keeping the mean of every bucket.
 * Used both for storage (1 min) and for rendering (a bounded point count).
 */
export function downsampleNetwork(samples: NetworkSample[], bucketMs: number): NetworkSample[] {
    if (samples.length === 0) return [];
    const buckets = new Map<number, { t: number; count: number; in: number; out: number }>();

    for (const sample of samples) {
        const key = Math.floor(sample.t / bucketMs);
        const bucket = buckets.get(key);
        if (bucket) {
            bucket.t = Math.max(bucket.t, sample.t);
            bucket.count += 1;
            bucket.in += sample.in;
            bucket.out += sample.out;
        } else {
            buckets.set(key, { t: sample.t, count: 1, in: sample.in, out: sample.out });
        }
    }

    return [...buckets.values()]
        .sort((a, b) => a.t - b.t)
        .map((bucket) => ({
            t: bucket.t,
            in: rate(bucket.in / bucket.count),
            out: rate(bucket.out / bucket.count),
        }));
}

function readStored(): NetworkSample[] {
    try {
        const raw = window.localStorage.getItem(STORAGE_KEY);
        if (!raw) return [];
        const parsed: unknown = JSON.parse(raw);
        if (!Array.isArray(parsed)) return [];
        const cutoff = Date.now() - RETENTION_MS;
        return parsed
            .filter(isSample)
            .filter((sample) => sample.t >= cutoff)
            .sort((a, b) => a.t - b.t);
    } catch {
        // Corrupted payload or storage disabled: fall back to an empty session.
        return [];
    }
}

/**
 * Client-side 24 h history of the aggregate network rate. The panel has no
 * metrics-history endpoint, so the buffer lives here: 5 s points while the view
 * is open, persisted downsampled to 1 min. The chart states the cadence and
 * sample count so the data is never implied to cover more than it does.
 */
export function useNetworkHistory(totals: NetworkTotals | null) {
    const [samples, setSamples] = useState<NetworkSample[]>([]);
    const latest = useRef<NetworkTotals | null>(totals);
    // the reading last put on the chart: when the node polls stall, the same one is not
    // recorded again every 5 s as a flat line that was never measured
    const recorded = useRef<NetworkTotals | null>(null);
    const lastPersist = useRef(0);

    useEffect(() => {
        latest.current = totals;
    }, [totals]);

    // Hydrate the persisted series once on mount.
    useEffect(() => {
        const stored = readStored();
        if (stored.length === 0) return;
        setSamples((previous) => (previous.length === 0 ? stored : [...stored, ...previous]));
        lastPersist.current = Date.now();
    }, []);

    useEffect(() => {
        const collect = () => {
            const value = latest.current;
            if (!value || value === recorded.current) return;
            recorded.current = value;
            const now = Date.now();
            setSamples((previous) => {
                const last = previous[previous.length - 1];
                // StrictMode mounts effects twice; skip the duplicate instant.
                if (last && now - last.t < 1_000) return previous;
                const cutoff = now - RETENTION_MS;
                return [
                    ...previous.filter((sample) => sample.t >= cutoff),
                    { t: now, in: rate(value.in), out: rate(value.out) },
                ];
            });
        };

        const tick = () => {
            // A hidden tab does not sample: a flat line would be a lie.
            if (document.visibilityState === 'hidden') return;
            collect();
        };

        collect();
        const interval = window.setInterval(tick, NETWORK_SAMPLE_INTERVAL_MS);
        return () => window.clearInterval(interval);
    }, []);

    useEffect(() => {
        if (samples.length < 2) return;
        const now = Date.now();
        if (now - lastPersist.current < PERSIST_INTERVAL_MS) return;
        lastPersist.current = now;
        try {
            window.localStorage.setItem(STORAGE_KEY, JSON.stringify(downsampleNetwork(samples, PERSIST_INTERVAL_MS)));
        } catch {
            // Quota or private mode: keep the in-memory series for this session.
        }
    }, [samples]);

    return { samples, intervalMs: NETWORK_SAMPLE_INTERVAL_MS };
}
