import { useState, useEffect } from 'react';
import { api } from '@/lib/api-client';

export type Node = {
    id: number;
    name: string;
    publicHost: string;
    publicPort: number;
    sftpPort: number;
};

export function useNodes(skip = false) {
    const [nodes, setNodes] = useState<Node[]>([]);
    const [loading, setLoading] = useState(!skip);
    const [error, setError] = useState<Error | null>(null);

    const fetchNodes = async () => {
        if (skip) return;
        try {
            setLoading(true);
            const data = await api.get('/api/nodes');
            setNodes(Array.isArray(data) ? data : []);
            setError(null);
        } catch (e: any) {
            setError(e);
            console.error('Failed to fetch nodes:', e);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        if (!skip) fetchNodes();
    }, [skip]);

    return { nodes, loading, error, refresh: fetchNodes };
}
