import { useState, useEffect, useRef } from 'react';
import { api } from '@/lib/api-client';

type NodeNetSnapshot = {
    bytesSent: number;
    bytesRecv: number;
    time: number;
};

export type NodeNetMetric = {
    time: string;
    networkIn: number;  // KB/s
    networkOut: number; // KB/s
};

/** Gives a node still pending its first metrics (or null when it did not answer). */
function fillPending(id: number, info: unknown) {
    return (nodes: any[]) => nodes.map(n => (n.id === id && n.systemInfo === undefined ? { ...n, systemInfo: info } : n));
}

export function useNodes(skip = false) {
    const [nodes, setNodes] = useState<any[]>([]);
    const [loading, setLoading] = useState(!skip);
    const [error, setError] = useState<Error | null>(null);
    // a poll that is still waiting on a slow node is not stacked with another one
    const inFlight = useRef(false);

    const fetchNodes = async () => {
        if (skip || inFlight.current) return;
        inFlight.current = true;
        try {
            const data = await api.get('/api/nodes');
            const nodesList: any[] = Array.isArray(data) ? data : [];

            // the list shows at once, with the metrics it had; it does not wait for the slowest node.
            // A node not asked yet has undefined metrics (pending), one that did not answer null.
            setNodes(prev => {
                const known = new Map(prev.map(n => [n.id, n.systemInfo]));
                return nodesList.map(node => ({ ...node, systemInfo: known.get(node.id) }));
            });
            setError(null);
            setLoading(false);

            // a node whose /system call fails is still listed, without metrics (offline). A node
            // still pending shows its first answer at once instead of waiting for the slowest one;
            // later polls land together, so the network chart sums one reading of every node.
            const infos = await Promise.all(nodesList.map(node =>
                api.get(`/api/nodes/${node.id}/system`).catch(() => null).then(info => {
                    setNodes(fillPending(node.id, info));
                    return info;
                })));
            setNodes(nodesList.map((node, i) => ({ ...node, systemInfo: infos[i] })));
        } catch (e: any) {
            setError(e);
            console.error('Failed to fetch nodes:', e);
        } finally {
            setLoading(false);
            inFlight.current = false;
        }
    };

    useEffect(() => {
        if (skip) return;

        void fetchNodes();
        const interval = setInterval(() => {
            // a hidden tab does not poll: each /system call keeps the node busy for a second
            if (document.visibilityState === 'hidden') return;
            void fetchNodes();
        }, 5000);

        return () => clearInterval(interval);
    }, [skip]);

    return { nodes, loading, error, refresh: fetchNodes };
}

export function useUsersCount(skip = false) {
    const [count, setCount] = useState(0);
    const [loading, setLoading] = useState(!skip);

    useEffect(() => {
        if (skip) return;
        // one row is enough: the total comes in paging.total
        api.get('/api/users?limit=1')
            .then(data => {
                setCount(data.paging?.total ?? (Array.isArray(data.users) ? data.users.length : 0));
            })
            .catch(() => setCount(0))
            .finally(() => setLoading(false));
    }, [skip]);

    return { count, loading };
}

/** Converts an elapsed counter pair into KB/s, treating a lower reading as a counter reset (daemon restart, etc.). */
function rateKBps(current: number, previous: number, elapsedSeconds: number): number {
    // a counter that went backwards (the node restarted) starts over: no rate for that reading
    const diff = current >= previous ? current - previous : 0;
    return (diff / elapsedSeconds) / 1024;
}

/**
 * Computes a differential KB/s network rate from the cumulative counters in each node's
 * system info. It reuses the nodes useNodes already polls, instead of fetching
 * /api/nodes and every node's /system a second time.
 * Returns a time-series array (last 60 points) for the dashboard global chart.
 */
export function useGlobalNetworkMetrics(nodes: any[]) {
    const [metrics, setMetrics] = useState<NodeNetMetric[]>([]);
    const lastSnapshotsRef = useRef<Record<number, NodeNetSnapshot>>({});
    // the reading each node's rate was last taken from: the same reading shown again is no new sample
    const lastInfoRef = useRef<Record<number, unknown>>({});

    useEffect(() => {
        if (nodes.length === 0) return;

        const now = Date.now();
        let totalRxRate = 0;
        let totalTxRate = 0;
        // a rate needs two readings: the first poll after opening the page only sets the baseline
        let measured = false;

        for (const node of nodes) {
            const sysInfo = node.systemInfo;
            if (!sysInfo || lastInfoRef.current[node.id] === sysInfo) continue;
            lastInfoRef.current[node.id] = sysInfo;

            const bytesRecv: number = sysInfo.networkBytesRecv ?? 0;
            const bytesSent: number = sysInfo.networkBytesSent ?? 0;
            // when the node read its counters; an older node without it falls back to this poll
            const sampledAt: number = typeof sysInfo.sampledAt === 'number' && sysInfo.sampledAt > 0 ? sysInfo.sampledAt : now;

            const prev = lastSnapshotsRef.current[node.id];
            const elapsed = prev && prev.time > 0 ? (sampledAt - prev.time) / 1000 : 0; // seconds
            if (elapsed > 0) {
                totalRxRate += rateKBps(bytesRecv, prev.bytesRecv, elapsed);
                totalTxRate += rateKBps(bytesSent, prev.bytesSent, elapsed);
                measured = true;
            }

            lastSnapshotsRef.current[node.id] = { bytesRecv, bytesSent, time: sampledAt };
        }

        // no fake "0 B/s" on every page load
        if (!measured) return;
        const time = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });

        setMetrics(prev => [
            ...prev,
            {
                time,
                networkIn: Number.parseFloat(totalRxRate.toFixed(2)),
                networkOut: Number.parseFloat(totalTxRate.toFixed(2)),
            },
        ].slice(-60));
    }, [nodes]);

    return { metrics };
}
