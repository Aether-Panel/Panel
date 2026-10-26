import { useState, useEffect } from 'react';
import { fetchAllPages } from '@/lib/paged';

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
            // every page: the owner picker must offer every user, not the first 20
            const { items } = await fetchAllPages<User>('/api/users', 'users');
            setUsers(items);
            setError(null);
        } catch (e: any) {
            setError(e);
            console.error('Failed to fetch users:', e);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        if (!skip) void fetchUsers();
    }, [skip]);

    return { users, loading, error, refresh: fetchUsers };
}
