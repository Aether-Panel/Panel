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

export function useNodes(skip = false) {
    const [nodes, setNodes] = useState<any[]>([]);
    const [loading, setLoading] = useState(!skip);
    const [error, setError] = useState<Error | null>(null);

    const fetchNodes = async () => {
        if (skip) return;
        try {
            const data = await api.get('/api/nodes');
            const nodesList = Array.isArray(data) ? data : [];

            // Try to fetch system info for each node
            const nodesWithStats = await Promise.all(nodesList.map(async (node: any) => {
                try {
                    const systemInfo = await api.get(`/api/nodes/${node.id}/system`).catch(() => null);
                    return { ...node, systemInfo };
                } catch {
                    // Stats are optional: a node whose /system call fails is still listed, just without them.
                    return node;
                }
            }));

            setNodes(nodesWithStats);
            setError(null);
        } catch (e: any) {
            setError(e);
            console.error('Failed to fetch nodes:', e);
        } finally {
            setLoading(false);
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

    useEffect(() => {
        if (nodes.length === 0) return;

        const now = Date.now();
        let totalRxRate = 0;
        let totalTxRate = 0;
        // a rate needs two readings: the first poll after opening the page only sets the baseline
        let measured = false;

        for (const node of nodes) {
            const sysInfo = node.systemInfo;
            if (!sysInfo) continue;

            const bytesRecv: number = sysInfo.networkBytesRecv ?? 0;
            const bytesSent: number = sysInfo.networkBytesSent ?? 0;

            const prev = lastSnapshotsRef.current[node.id];
            const elapsed = prev && prev.time > 0 ? (now - prev.time) / 1000 : 0; // seconds
            if (elapsed > 0) {
                totalRxRate += rateKBps(bytesRecv, prev.bytesRecv, elapsed);
                totalTxRate += rateKBps(bytesSent, prev.bytesSent, elapsed);
                measured = true;
            }

            lastSnapshotsRef.current[node.id] = { bytesRecv, bytesSent, time: now };
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
