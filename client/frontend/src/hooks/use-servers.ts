import { useState, useEffect, useRef } from 'react';
import { api } from '@/lib/api-client';
import type { Server } from '@/lib/data';

export function useServers() {
    const [servers, setServers] = useState<Server[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<Error | null>(null);
    const pollerRef = useRef<NodeJS.Timeout | null>(null);

    const fetchServers = async () => {
        try {
            const [serversData, uptimeData] = await Promise.all([
                api.get('/api/servers'),
                api.get('/api/uptime').catch(() => ({}))
            ]);

            const mappedServers: Server[] = (serversData.servers || []).map((s: any) => {
                const uptime = uptimeData[s.id] || {};
                return {
                    id: s.id,
                    name: s.name,
                    nodeId: s.nodeId,
                    ipAddress: s.ip || '0.0.0.0',
                    port: s.port || 0,
                    status: uptime.currentStatus ? 'online' : 'offline',
                    cpuUsage: 0,
                    memoryUsage: 0,
                    storageUsage: 0,
                    metrics: [],
                    alerts: [],
                };
            });

            setServers(mappedServers);
            setError(null);

            // Start fetching stats for each server
            updateAllStats(mappedServers);
        } catch (e: any) {
            setError(e);
            console.error('Failed to fetch servers:', e);
        } finally {
            setLoading(false);
        }
    };

    const updateAllStats = async (currentServers: Server[]) => {
        // Only fetch stats for the first 10 servers to avoid overloading
        const serversToFetch = currentServers.slice(0, 10);

        const statsPromises = serversToFetch.map(async (server) => {
            try {
                const stats = await api.get(`/api/servers/${server.id}/stats`).catch(() => null);
                if (stats) {
                    return {
                        id: server.id,
                        cpuUsage: Math.round(stats.cpu || 0),
                        memoryUsage: Math.round(stats.memory ? (stats.memory / (stats.maxMemory || stats.memory || 1024)) * 100 : 0),
                        storageUsage: 0, // Not always in stats
                        status: 'online',
                        metrics: {
                            time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
                            cpu: stats.cpu || 0,
                            memory: stats.memory || 0,
                            networkIn: stats.network?.in || 0,
                            networkOut: stats.network?.out || 0,
                        }
                    };
                }
            } catch (e) {
                return { id: server.id, status: 'offline' };
            }
            return null;
        });

        const results = await Promise.all(statsPromises);

        setServers(prev => prev.map(s => {
            const res = results.find(r => r && r.id === s.id);
            if (res) {
                // Prepend new metrics to history (keep last 20)
                const newMetrics = [...(s.metrics || []), res.metrics].filter(Boolean).slice(-20);
                return {
                    ...s,
                    status: res.status as any,
                    cpuUsage: res.cpuUsage ?? s.cpuUsage,
                    memoryUsage: res.memoryUsage ?? s.memoryUsage,
                    metrics: newMetrics as any
                };
            }
            return s;
        }));
    };

    useEffect(() => {
        fetchServers();
    }, []);

    useEffect(() => {
        if (servers.length === 0) return;

        const interval = setInterval(() => {
            updateAllStats(servers);
        }, 15000);

        // Initial update
        updateAllStats(servers);

        return () => clearInterval(interval);
    }, [servers.length]); // Only reset interval if server count changes

    return { servers, loading, error, refresh: fetchServers };
}
