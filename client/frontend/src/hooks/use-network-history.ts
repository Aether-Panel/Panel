import { useCallback, useEffect, useState } from 'react';
import { api } from '@/lib/api-client';

export type NetworkRange = '1h' | '6h' | '24h';

export type NetworkSample = {
    /** Epoch ms of the start of the step. */
    t: number;
    /** Download rate added up over every node, in KB/s, averaged over the step. */
    in: number;
    out: number;
};

type NetworkHistoryResponse = {
    step: number;
    points: { t: number; rx: number; tx: number }[];
    nodes: number;
    unavailable: number;
};

export type NetworkHistory = {
    samples: NetworkSample[];
    /** Length of one point, in ms (1 min, 5 min or 15 min with the range). */
    stepMs: number;
    nodes: number;
    /** Nodes that did not answer (down, or too old to keep a history). */
    unavailable: number;
    loading: boolean;
    error: boolean;
};

/** The nodes record a point a minute: asking more often only redraws the same chart. */
const REFRESH_MS = 60_000;

/** The browser-side buffer the chart used before the nodes kept the history themselves. */
const LEGACY_STORAGE_KEY = 'aether.dashboard.network-history.v1';

/**
 * The network history of every node, added up. Each node keeps its own traffic for a day, a
 * point a minute, so the chart shows what happened while nobody had the dashboard open.
 */
export function useNetworkHistory(range: NetworkRange, enabled: boolean): NetworkHistory {
    const [state, setState] = useState<Omit<NetworkHistory, 'loading' | 'error'> & { range: NetworkRange | null }>({
        samples: [],
        stepMs: 60_000,
        nodes: 0,
        unavailable: 0,
        range: null,
    });
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState(false);

    useEffect(() => {
        try {
            window.localStorage.removeItem(LEGACY_STORAGE_KEY);
        } catch {
            // storage blocked: nothing to clean up
        }
    }, []);

    const load = useCallback(async () => {
        setLoading(true);
        try {
            const data = await api.get<NetworkHistoryResponse>(`/api/nodes/network-history?range=${range}`);
            setState({
                samples: data.points.map((point) => ({ t: point.t * 1000, in: point.rx, out: point.tx })),
                stepMs: data.step * 1000,
                nodes: data.nodes,
                unavailable: data.unavailable,
                range,
            });
            setError(false);
        } catch {
            // the last good history stays on screen
            setError(true);
        } finally {
            setLoading(false);
        }
    }, [range]);

    useEffect(() => {
        if (!enabled) return;
        void load();
        const interval = window.setInterval(() => {
            if (document.visibilityState !== 'hidden') void load();
        }, REFRESH_MS);
        return () => window.clearInterval(interval);
    }, [enabled, load]);

    // a switch of range shows nothing from the previous range while the new one loads
    const current = state.range === range;
    return {
        samples: current ? state.samples : [],
        stepMs: state.stepMs,
        nodes: state.nodes,
        unavailable: state.unavailable,
        loading: loading || !current,
        error,
    };
}
