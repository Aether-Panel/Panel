import { useState, useEffect } from 'react';
import { api } from '@/lib/api-client';

export type Node = {
    id: number;
    name: string;
    publicHost: string;
    publicPort: number;
    sftpPort: number;
};

export function useNodes() {
    const [nodes, setNodes] = useState<Node[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<Error | null>(null);

    const fetchNodes = async () => {
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
        fetchNodes();
    }, []);

    return { nodes, loading, error, refresh: fetchNodes };
}
