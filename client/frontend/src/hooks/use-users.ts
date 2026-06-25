import { useState, useEffect } from 'react';
import { api } from '@/lib/api-client';

export type User = {
    id: number;
    username: string;
    email: string;
};

export function useUsers(skip = false) {
    const [users, setUsers] = useState<User[]>([]);
    const [loading, setLoading] = useState(!skip);
    const [error, setError] = useState<Error | null>(null);

    const fetchUsers = async () => {
        if (skip) return;
        try {
            setLoading(true);
            const data = await api.get('/api/users');
            setUsers(Array.isArray(data.users) ? data.users : []);
            setError(null);
        } catch (e: any) {
            setError(e);
            console.error('Failed to fetch users:', e);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        if (!skip) fetchUsers();
    }, [skip]);

    return { users, loading, error, refresh: fetchUsers };
}
