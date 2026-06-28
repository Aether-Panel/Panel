import { useState, useEffect } from 'react';
import { api } from '@/lib/api-client';

export type DatabaseHost = {
    id: number;
    name: string;
    host: string;
    port: number;
    username: string;
    max_databases: number;
    node_id?: number | null;
};

export function useDatabaseHosts() {
    const [hosts, setHosts] = useState<DatabaseHost[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<Error | null>(null);

    const fetchHosts = async () => {
        try {
            setLoading(true);
            const data = await api.get('/api/databasehosts');
            setHosts(Array.isArray(data) ? data : []);
            setError(null);
        } catch (e: any) {
            setError(e);
            console.error('Failed to fetch database hosts:', e);
        } finally {
            setLoading(false);
        }
    };

    const createHost = async (hostData: Omit<DatabaseHost, 'id'> & { password?: string }) => {
        try {
            const data = await api.post('/api/databasehosts', hostData);
            setHosts(prev => [...prev, data]);
            return data;
        } catch (e: any) {
            console.error('Failed to create database host:', e);
            throw e;
        }
    };

    const updateHost = async (id: number, hostData: Partial<DatabaseHost> & { password?: string }) => {
        try {
            await api.put(`/api/databasehosts/${id}`, hostData);
            await fetchHosts();
        } catch (e: any) {
            console.error('Failed to update database host:', e);
            throw e;
        }
    };

    const deleteHost = async (id: number) => {
        try {
            await api.delete(`/api/databasehosts/${id}`);
            setHosts(prev => prev.filter(h => h.id !== id));
        } catch (e: any) {
            console.error('Failed to delete database host:', e);
            throw e;
        }
    };

    useEffect(() => {
        fetchHosts();
    }, []);

    return { hosts, loading, error, createHost, updateHost, deleteHost, refresh: fetchHosts };
}
