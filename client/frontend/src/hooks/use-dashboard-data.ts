import { useState, useEffect, useRef, useCallback } from 'react';
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
                } catch (e) {
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
        
        fetchNodes();
        const interval = setInterval(() => {
            fetchNodes();
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
        api.get('/api/users?pageSize=1')
            .then(data => {
                setCount(data.metadata?.total || (Array.isArray(data.users) ? data.users.length : 0));
            })
            .catch(() => setCount(0))
            .finally(() => setLoading(false));
    }, [skip]);

    return { count, loading };
}

/**
 * Polls all nodes every 5s and computes a differential KB/s network rate
 * from the cumulative counters in /api/nodes/:id/system.
 * Returns a time-series array (last 60 points) for the dashboard global chart.
 */
export function useGlobalNetworkMetrics() {
    const [metrics, setMetrics] = useState<NodeNetMetric[]>([]);
    const lastSnapshotsRef = useRef<Record<number, NodeNetSnapshot>>({});

    const poll = useCallback(async () => {
        try {
            const data = await api.get('/api/nodes');
            const nodesList: any[] = Array.isArray(data) ? data : [];
            if (nodesList.length === 0) return;

            const now = Date.now();
            let totalRxRate = 0;
            let totalTxRate = 0;

            await Promise.all(nodesList.map(async (node: any) => {
                try {
                    const sysInfo = await api.get(`/api/nodes/${node.id}/system`).catch(() => null);
                    if (!sysInfo) return;

                    const bytesRecv: number = sysInfo.networkBytesRecv ?? 0;
                    const bytesSent: number = sysInfo.networkBytesSent ?? 0;

                    const prev = lastSnapshotsRef.current[node.id];
                    if (prev && prev.time > 0) {
                        const elapsed = (now - prev.time) / 1000; // seconds
                        if (elapsed > 0) {
                            // Guard against counter resets (daemon restart, etc.)
                            const rxDiff = bytesRecv >= prev.bytesRecv ? bytesRecv - prev.bytesRecv : bytesRecv;
                            const txDiff = bytesSent >= prev.bytesSent ? bytesSent - prev.bytesSent : bytesSent;
                            totalRxRate += (rxDiff / elapsed) / 1024; // KB/s
                            totalTxRate += (txDiff / elapsed) / 1024; // KB/s
                        }
                    }

                    lastSnapshotsRef.current[node.id] = {
                        bytesRecv,
                        bytesSent,
                        time: now,
                    };
                } catch (_) {
                    // silently ignore per-node errors
                }
            }));

            const time = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });

            setMetrics(prev => {
                const next: NodeNetMetric[] = [
                    ...prev,
                    {
                        time,
                        networkIn: parseFloat(totalRxRate.toFixed(2)),
                        networkOut: parseFloat(totalTxRate.toFixed(2)),
                    },
                ];
                return next.slice(-60);
            });
        } catch (e) {
            console.error('Failed to poll global network metrics:', e);
        }
    }, []);

    useEffect(() => {
        // Kick off immediately, then every 5 seconds
        poll();
        const interval = setInterval(poll, 5000);
        return () => clearInterval(interval);
    }, [poll]);

    return { metrics };
}
