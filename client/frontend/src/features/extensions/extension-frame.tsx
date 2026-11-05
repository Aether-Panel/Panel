'use client';

import { useEffect, useRef, useState } from 'react';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';

// The panel's address as set in Settings, the same one its extension events carry and sign,
// so a service can match the page to the events; the browser's origin when it is not set
let panelUrl: Promise<string> | null = null;
function signedPanelUrl(): Promise<string> {
    panelUrl ??= fetch('/.well-known/aether-panel.json')
        .then((r) => (r.ok ? r.json() : null))
        .then((identity) => (typeof identity?.url === 'string' && identity.url ? identity.url : globalThis.location.origin))
        .catch(() => globalThis.location.origin);
    return panelUrl;
}

/**
 * An extension page. The frame is sandboxed (and the panel sends its files with a sandbox
 * policy too), so its scripts run in an opaque origin: no panel cookies, storage or API.
 * Once loaded it gets {type: 'aether:context', serverId, panelUrl, language, theme} by postMessage, and
 * it may answer {type: 'aether:resize', height} to fit its content.
 */
export function ExtensionFrame({
    src,
    title,
    serverId,
    className,
}: Readonly<{ src: string; title: string; serverId?: string; className?: string }>) {
    const { language } = useTranslations();
    const frameRef = useRef<HTMLIFrameElement>(null);
    const [height, setHeight] = useState<number | null>(null);

    const sendContext = async () => {
        // panelUrl tells a service which panel the page is in: the page itself cannot see it
        const url = await signedPanelUrl();
        const target = frameRef.current?.contentWindow;
        if (!target) return;
        const theme = document.documentElement.classList.contains('dark') ? 'dark' : 'light';
        // the sandboxed page has an opaque origin, so "*" is the only target that reaches it
        target.postMessage({ type: 'aether:context', serverId: serverId ?? null, panelUrl: url, language, theme }, '*');
    };

    useEffect(() => {
        const onMessage = (event: MessageEvent) => {
            // only this frame's page, which is sandboxed and so always has the opaque origin "null"
            if (event.origin !== 'null' || event.source !== frameRef.current?.contentWindow) return;
            const data = event.data;
            if (data?.type === 'aether:resize' && typeof data.height === 'number' && Number.isFinite(data.height)) {
                setHeight(Math.min(Math.max(Math.round(data.height), 200), 4000));
            }
        };
        window.addEventListener('message', onMessage);
        return () => window.removeEventListener('message', onMessage);
    }, []);

    // a new server or language reaches a page that is already open
    useEffect(() => {
        void sendContext();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [serverId, language]);

    return (
        <iframe
            ref={frameRef}
            src={src}
            title={title}
            sandbox="allow-scripts allow-forms allow-popups allow-popups-to-escape-sandbox allow-modals allow-downloads"
            referrerPolicy="no-referrer"
            loading="lazy"
            onLoad={() => void sendContext()}
            style={height ? { height } : undefined}
            className={cn('block w-full rounded-xl border border-border/60 bg-card', !height && 'h-[70dvh] min-h-[24rem]', className)}
        />
    );
}
