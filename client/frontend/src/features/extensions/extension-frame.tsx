'use client';

import { useEffect, useRef, useState } from 'react';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';

/**
 * An extension page. The frame is sandboxed (and the panel sends its files with a sandbox
 * policy too), so its scripts run in an opaque origin: no panel cookies, storage or API.
 * Once loaded it gets {type: 'aether:context', serverId, language, theme} by postMessage, and
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

    const sendContext = () => {
        const target = frameRef.current?.contentWindow;
        if (!target) return;
        const theme = document.documentElement.classList.contains('dark') ? 'dark' : 'light';
        // the sandboxed page has an opaque origin, so "*" is the only target that reaches it
        target.postMessage({ type: 'aether:context', serverId: serverId ?? null, language, theme }, '*');
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
        sendContext();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [serverId, language]);

    return (
        <iframe
            ref={frameRef}
            src={src}
            title={title}
            sandbox="allow-scripts allow-forms allow-popups allow-modals allow-downloads"
            referrerPolicy="no-referrer"
            loading="lazy"
            onLoad={sendContext}
            style={height ? { height } : undefined}
            className={cn('block w-full rounded-xl border border-border/60 bg-card', !height && 'h-[70dvh] min-h-[24rem]', className)}
        />
    );
}
