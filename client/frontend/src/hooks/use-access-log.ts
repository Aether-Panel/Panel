import { useEffect, useState } from 'react';
import { ApiError, api } from '@/lib/api-client';

/** Event kinds the panel records today. */
export type AccessEventKind = 'login' | 'login_failed' | 'register';

export type AccessEvent = {
    id: number;
    event: AccessEventKind | string;
    username?: string;
    email?: string;
    ip?: string;
    userAgent?: string;
    createdAt: string;
};

type AccessLogState = {
    events: AccessEvent[];
    loading: boolean;
    /** The request was refused because the account lacks the scope. */
    denied: boolean;
    /** The panel answered without the activity feed (older build, or an error). */
    unavailable: boolean;
};

const IDLE: AccessLogState = { events: [], loading: false, denied: false, unavailable: false };

const REFRESH_MS = 60_000;
/**
 * Reads the access feed from `GET /api/activity`. The states are kept apart on
 * purpose: "no records", "not allowed" and "not there" are three different
 * answers and the panel must not pass one off as another. It refreshes once a
 * minute — without flipping back to the skeleton — so a sign-in that happens
 * while the dashboard is open shows up.
 */
export function useAccessLog(limit = 6, skip = false): AccessLogState {
    const [state, setState] = useState<AccessLogState>({ ...IDLE, loading: !skip });

    useEffect(() => {
        if (skip) {
            setState(IDLE);
            return;
        }

        let cancelled = false;

        const load = async (initial: boolean) => {
            if (initial) {
                setState((previous) => ({ ...previous, loading: true }));
            }

            try {
                const data = await api.get(`/api/activity?limit=${limit}`);
                if (cancelled) return;
                const events: AccessEvent[] = Array.isArray(data?.activity) ? data.activity : [];
                setState({ events, loading: false, denied: false, unavailable: false });
            } catch (error: unknown) {
                if (cancelled) return;
                const status = error instanceof ApiError ? error.status : 0;
                setState({ events: [], loading: false, denied: status === 403, unavailable: status !== 403 });
            }
        };

        void load(true);

        const interval = window.setInterval(() => {
            // A hidden tab does not poll: nothing is watching the feed.
            if (document.visibilityState === 'hidden') return;
            void load(false);
        }, REFRESH_MS);

        return () => {
            cancelled = true;
            window.clearInterval(interval);
        };
    }, [limit, skip]);

    return state;
}
