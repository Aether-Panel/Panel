import { useState, useEffect, useRef } from 'react';
import { api, ApiError } from '@/lib/api-client';
import type { Server } from '@/lib/data';
import { parsePlayerQuery } from '@/lib/players';
import { fetchAllPages } from '@/lib/paged';
const globalFailedStatsSet = new Set<string>();

// Stats are polled for every listed server; cap parallel requests so large lists do not flood the panel
const STATS_CONCURRENCY = 6;

async function mapWithConcurrency<T, R>(items: T[], limit: number, fn: (item: T) => Promise<R>): Promise<R[]> {
    const results = new Array<R>(items.length);
    let next = 0;
    const workers = Array.from({ length: Math.min(limit, items.length) }, async () => {
        while (next < items.length) {
            const i = next++;
            results[i] = await fn(items[i]);
        }
    });
    await Promise.all(workers);
    return results;
}

/** CPU as a share of what the server may use: the node sends its allowance (the "cpu" limit,
 *  or every core when unlimited); 200% of one core on a two-core server is 100%, the same number
 *  every chart and card shows. */
function cpuShare(stats: any, server: Server) {
    const raw = stats.cpu || 0;
    const allowance = stats.maxCpu > 0 ? stats.maxCpu : (server.totalCpu ?? 0);
    return {
        cpu: Math.min(100, Math.round(allowance > 0 ? (raw / allowance) * 100 : raw)),
        raw: Math.round(raw),
        allowance: allowance > 0 ? allowance : undefined,
    };
}

/** Memory as a share of the limit; an older node without a limit may already send a percentage. */
function memoryShare(stats: any): number {
    if (stats.maxMemory > 0) return Math.round((stats.memory / stats.maxMemory) * 100);
    return stats.memory <= 100 ? Math.round(stats.memory) : 0;
}

/** One server's figures and live sample from its /stats and player query. */
function statsResult(server: Server, stats: any, players: ReturnType<typeof parsePlayerQuery>) {
    const { cpu, raw, allowance } = cpuShare(stats, server);
    const memory = memoryShare(stats);
    const storageUsed = stats.storage || 0;
    const storageMax = stats.maxStorage || 0;
    return {
        id: server.id,
        cpuUsage: cpu,
        cpuRaw: raw,
        maxCpu: allowance,
        memoryUsage: memory,
        memoryUsed: stats.memory || 0,
        storageUsage: storageMax > 0 ? Math.round((storageUsed / storageMax) * 100) : 0,
        storageUsed,
        storageMax,
        status: stats.running ? 'online' : 'offline',
        metrics: {
            // the charts format it in the panel's language
            ts: Date.now(),
            cpu,
            memory: Math.min(100, memory),
            networkIn: Number.parseFloat((stats.networkRx || 0).toFixed(2)),
            networkOut: Number.parseFloat((stats.networkTx || 0).toFixed(2)),
        },
        playersOnline: players?.online,
        maxPlayers: players?.max,
        hasPlayerQuery: players !== null,
        playerNames: players?.names,
        playerInfo: players ?? undefined,
    };
}

/**
 * Lists the user's servers and polls live stats for them.
 * `statsFor` limits the polling to the servers a page shows (the detail page needs one);
 * without it every listed server is polled.
 */
export function useServers(statsFor?: (server: Server) => boolean) {
    const [servers, setServers] = useState<Server[]>([]);
    const statsForRef = useRef(statsFor);
    statsForRef.current = statsFor;
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<Error | null>(null);
    const statsInFlightRef = useRef(false);

    const fetchServers = async () => {
        try {
            const [serversData, uptimeData] = await Promise.all([
                // every page: without a limit the API answers only the first 20
                fetchAllPages('/api/servers', 'servers'),
                api.get('/api/uptime').catch(() => ({}))
            ]);

            const mappedServers: Server[] = serversData.items.map((s: any) => {
                const uptime = uptimeData[s.id] || {};
                return {
                    id: s.id,
                    name: s.name,
                    nodeId: s.nodeId ?? (s.node?.id !== undefined ? s.node.id : 0),
                    ipAddress: s.ip && s.ip !== '0.0.0.0' ? s.ip : (s.node?.publicHost || '0.0.0.0'),
                    port: s.port || 0,
                    status: uptime.currentStatus ? 'online' : 'offline',
                    cpuUsage: 0,
                    memoryUsage: 0,
                    storageUsage: 0,
                    storageUsed: 0,
                    storageMax: 0,
                    memoryUsed: 0,
                    metrics: [],
                    alerts: [],
                    isGhost: s.isGhost,
                    parentServerId: s.parent_server_id,
                    totalCpu: s.total_cpu,
                    totalMemory: s.total_memory,
                    totalDisk: s.total_disk,
                    uptimePercent: typeof uptime.uptimePercent === 'number' ? uptime.uptimePercent : undefined,
                };
            });

            // a refresh (after start, stop…) keeps the live stats, chart history and players
            // already shown; the next stats round brings them up to date
            setServers(prev => mappedServers.map(fresh => {
                const shown = prev.find(p => p.id === fresh.id);
                if (!shown) return fresh;
                return {
                    ...shown,
                    name: fresh.name,
                    nodeId: fresh.nodeId,
                    ipAddress: fresh.ipAddress,
                    port: fresh.port,
                    isGhost: fresh.isGhost,
                    parentServerId: fresh.parentServerId,
                    totalCpu: fresh.totalCpu,
                    totalMemory: fresh.totalMemory,
                    totalDisk: fresh.totalDisk,
                    uptimePercent: fresh.uptimePercent,
                };
            }));
            setError(null);

            // Start fetching stats for each server
            void updateAllStats(mappedServers);
        } catch (e: any) {
            console.error('Failed to fetch servers:', e);
            setError(e);
        } finally {
            setLoading(false);
        }
    };

    // a server whose stats cannot be read shows nothing in use, not its last numbers as if live
    const offlineResult = (id: string) => ({ id, status: 'offline', cpuUsage: 0, cpuRaw: 0, memoryUsage: 0, memoryUsed: 0, metrics: undefined });

    const updateAllStats = async (currentServers: Server[]) => {
        const statsFor = statsForRef.current;
        const wanted = statsFor ? currentServers.filter((server) => statsFor(server)) : currentServers;
        const results = await mapWithConcurrency(wanted, STATS_CONCURRENCY, async (server) => {
            try {
                if (globalFailedStatsSet.has(server.id) || server.isGhost) {
                    return offlineResult(server.id);
                }

                const [stats, queryData] = await Promise.all([
                    // a failed stats call must reach the catch below and mark the server offline,
                    // or it would keep its last status and numbers forever
                    api.get(`/api/servers/${server.id}/stats`),
                    api.get(`/api/servers/${server.id}/query`).catch(() => null)
                ]);

                return stats ? statsResult(server, stats, parsePlayerQuery(queryData)) : null;
            } catch (e: any) {
                if (e instanceof ApiError && e.status === 404) {
                    globalFailedStatsSet.add(server.id);
                }
                return offlineResult(server.id);
            }
        });

        setServers(prev => prev.map(s => {
            const res = results.find(r => r && r.id === s.id);
            if (res) {
                // Keep last 60 points (~5 mins if polling every 5s)
                const newMetrics = [...(s.metrics || []), res.metrics].filter(Boolean).slice(-60);
                return {
                    ...s,
                    status: res.status as any,
                    cpuUsage: res.cpuUsage ?? s.cpuUsage,
                    cpuRaw: (res as any).cpuRaw ?? s.cpuRaw,
                    maxCpu: (res as any).maxCpu ?? s.maxCpu,
                    memoryUsage: res.memoryUsage ?? s.memoryUsage,
                    storageUsage: (res as any).storageUsage ?? s.storageUsage,
                    storageUsed: (res as any).storageUsed ?? s.storageUsed,
                    storageMax: (res as any).storageMax ?? s.storageMax,
                    memoryUsed: (res as any).memoryUsed ?? s.memoryUsed,
                    playersOnline: (res as any).playersOnline,
                    maxPlayers: (res as any).maxPlayers,
                    hasPlayerQuery: (res as any).hasPlayerQuery,
                    playerNames: (res as any).playerNames,
                    playerInfo: (res as any).playerInfo,
                    metrics: newMetrics as any
                };
            }
            return s;
        }));
    };

    useEffect(() => {
        void fetchServers();
    }, []);

    useEffect(() => {
        if (servers.length === 0) return;

        // fetchServers already loads stats right after listing, so only poll from here.
        // Skip a tick while the previous round is still running so slow nodes do not pile up requests.
        const interval = setInterval(() => {
            if (document.visibilityState === 'hidden' || statsInFlightRef.current) return;
            statsInFlightRef.current = true;
            void updateAllStats(servers).finally(() => {
                statsInFlightRef.current = false;
            });
        }, 5000);

        return () => clearInterval(interval);
    }, [servers.length]); // Only reset interval if server count changes


    return { servers, loading, error, refresh: fetchServers };
}
