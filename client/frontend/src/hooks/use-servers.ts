import { useState, useEffect, useRef } from 'react';
import { api, ApiError } from '@/lib/api-client';
import type { Server } from '@/lib/data';

const globalFailedStatsSet = new Set<string>();

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
                    storageUsage: uptime.storageUsage || 0,
                    metrics: [],
                    alerts: [],
                    isGhost: s.isGhost,
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
                if (globalFailedStatsSet.has(server.id) || server.isGhost) {
                    return { id: server.id, status: 'offline' };
                }

                const stats = await api.get(`/api/servers/${server.id}/stats`);
                if (stats) {
                    const cpu = Math.round(stats.cpu || 0);
                    // If maxMemory is missing or 0, we can't calculate percentage reliably. 
                    // We'll fallback to showing 0 or trying to use the value if it looks like a % already.
                    let memory = 0;
                    if (stats.maxMemory && stats.maxMemory > 0) {
                        memory = Math.round((stats.memory / stats.maxMemory) * 100);
                    } else if (stats.memory <= 100) {
                        // Maybe it's already a percentage?
                        memory = Math.round(stats.memory);
                    }

                    return {
                        id: server.id,
                        cpuUsage: cpu,
                        memoryUsage: memory,
                        storageUsage: Math.round(stats.storage || 0),
                        status: stats.running ? 'online' : 'offline',
                        metrics: {
                            time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }),
                            cpu: Math.min(100, cpu),
                            memory: Math.min(100, memory),
                            networkIn: stats.network?.in || 0,
                            networkOut: stats.network?.out || 0,
                        }
                    };
                }
            } catch (e: any) {
                if (e instanceof ApiError && e.status === 404) {
                    globalFailedStatsSet.add(server.id);
                }
                return { id: server.id, status: 'offline' };
            }
            return null;
        });

        const results = await Promise.all(statsPromises);

        setServers(prev => prev.map(s => {
            const res = results.find(r => r && r.id === s.id);
            if (res) {
                // Keep last 60 points (~5 mins if polling every 5s)
                const newMetrics = [...(s.metrics || []), res.metrics].filter(Boolean).slice(-60);
                return {
                    ...s,
                    status: res.status as any,
                    cpuUsage: res.cpuUsage ?? s.cpuUsage,
                    memoryUsage: res.memoryUsage ?? s.memoryUsage,
                    storageUsage: (res as any).storageUsage ?? s.storageUsage,
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
        }, 5000);

        // Initial update
        updateAllStats(servers);

        return () => clearInterval(interval);
    }, [servers.length]); // Only reset interval if server count changes

    return { servers, loading, error, refresh: fetchServers };
}
