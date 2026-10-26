import { api } from '@/lib/api-client';

// the API's largest page; a list asked for without a limit stops at 20
const PAGE_SIZE = 100;
// a safety stop, far above any real panel
const MAX_PAGES = 50;

type Paged = { paging?: { total?: number } } & Record<string, any>;

/**
 * Every item of a paginated list (`/api/servers`, `/api/users`): it asks for pages of 100
 * until it has `paging.total` items. `key` is where the items are in the answer.
 */
export async function fetchAllPages<T = any>(url: string, key: string): Promise<{ items: T[]; total: number }> {
    const items: T[] = [];
    const separator = url.includes('?') ? '&' : '?';
    let total = 0;
    for (let page = 1; page <= MAX_PAGES; page++) {
        const data = await api.get<Paged>(`${url}${separator}limit=${PAGE_SIZE}&page=${page}`);
        const batch: T[] = Array.isArray(data?.[key]) ? data[key] : [];
        items.push(...batch);
        total = data?.paging?.total ?? items.length;
        if (batch.length < PAGE_SIZE || items.length >= total) break;
    }
    return { items, total };
}
