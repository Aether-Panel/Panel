import { useEffect, useState } from 'react';
import { api } from '@/lib/api-client';

export type ExtensionPage = {
    id: string;
    title: string;
    placement: 'server' | 'admin';
    path: string;
    url: string;
};

export type ActiveExtension = {
    id: string;
    name: string;
    pages: ExtensionPage[];
};

// one request per page load, shared by the menu and the server tabs
let request: Promise<ActiveExtension[]> | null = null;

function loadActive(): Promise<ActiveExtension[]> {
    request ??= api.get<ActiveExtension[]>('/api/extensions/active').catch(() => {
        request = null;
        return [];
    });
    return request;
}

/** The switched-on extensions and their pages; administration pages only for administrators. */
export function useActiveExtensions() {
    const [extensions, setExtensions] = useState<ActiveExtension[]>([]);
    const [ready, setReady] = useState(false);

    useEffect(() => {
        let cancelled = false;
        void loadActive().then((list) => {
            if (cancelled) return;
            setExtensions(list);
            setReady(true);
        });
        return () => {
            cancelled = true;
        };
    }, []);

    return { extensions, ready };
}

/** Every page of the given placement, with the extension it belongs to. */
export function pagesFor(extensions: ActiveExtension[], placement: ExtensionPage['placement']) {
    return extensions.flatMap((ext) =>
        ext.pages.filter((p) => p.placement === placement).map((page) => ({ extension: ext, page })),
    );
}
