import { useState, useEffect } from 'react';
import { api } from '@/lib/api-client';

export function useNodes() {
    const [nodes, setNodes] = useState<any[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<Error | null>(null);

    const fetchNodes = async () => {
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
        fetchNodes();
    }, []);

    return { nodes, loading, error, refresh: fetchNodes };
}

export function useUsersCount() {
    const [count, setCount] = useState(0);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        api.get('/api/users?pageSize=1')
            .then(data => {
                setCount(data.metadata?.total || (Array.isArray(data.users) ? data.users.length : 0));
            })
            .catch(() => setCount(0))
            .finally(() => setLoading(false));
    }, []);

    return { count, loading };
}
