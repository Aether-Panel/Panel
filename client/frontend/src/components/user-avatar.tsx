'use client';

import { useEffect, useState } from 'react';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { cn } from '@/lib/utils';

/**
 * Kodama (https://www.kodama.sh) avatars, loaded from its HTTP API so none of
 * its AGPL code ships inside the panel. The seed is a one-way hash of the
 * account, never the e-mail or username itself, so no personal data leaves
 * the panel; the same account always gets the same face. Gradients follow the
 * panel's blues and violet (hex pairs are API parameters, not CSS).
 */
const KODAMA_API = 'https://api.kodama.sh';
const KODAMA_GRADIENTS = '3B82F6-8B5CF6,0EA5E9-6366F1,14B8A6-3B82F6,A78BFA-60A5FA';

/** FNV-1a (64-bit) as a fallback where SubtleCrypto is unavailable (plain-HTTP deployments). */
function fnv1a64(input: string): string {
    let hash = 0xcbf29ce484222325n;
    for (const char of new TextEncoder().encode(input)) {
        hash ^= BigInt(char);
        hash = (hash * 0x100000001b3n) & 0xffffffffffffffffn;
    }
    return hash.toString(16).padStart(16, '0');
}

async function seedFor(identity: string): Promise<string> {
    const normalized = identity.trim().toLowerCase();
    if (globalThis.crypto?.subtle) {
        const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(`aether:${normalized}`));
        return Array.from(new Uint8Array(digest).slice(0, 12), (b) => b.toString(16).padStart(2, '0')).join('');
    }
    return fnv1a64(`aether:${normalized}`);
}

/** The Kodama URL for an account, or `undefined` until the seed is ready. */
export function useKodamaAvatar(identity: string | undefined, size = 96): string | undefined {
    const [url, setUrl] = useState<string>();

    useEffect(() => {
        if (!identity) {
            setUrl(undefined);
            return;
        }
        let cancelled = false;
        void seedFor(identity).then((seed) => {
            if (cancelled) return;
            const params = new URLSearchParams({
                size: String(size),
                shape: 'circle',
                background: 'gradient',
                gradients: KODAMA_GRADIENTS,
                animations: 'blink',
            });
            setUrl(`${KODAMA_API}/${seed}?${params.toString()}`);
        });
        return () => {
            cancelled = true;
        };
    }, [identity, size]);

    return url;
}

type UserAvatarProps = {
    /** Stable identity for the generated face (e-mail preferred, else username). */
    identity?: string;
    /** A picture the user chose; wins over the generated one. */
    src?: string | null;
    /** Text for the fallback (initials). */
    fallback: string;
    className?: string;
    fallbackClassName?: string;
};

/**
 * The account picture used everywhere in the panel: the user's own picture if
 * set, otherwise their Kodama face, with initials while it loads or if the
 * service can't be reached.
 */
export function UserAvatar({ identity, src, fallback, className, fallbackClassName }: Readonly<UserAvatarProps>) {
    const generated = useKodamaAvatar(src ? undefined : identity);
    const image = src || generated;

    return (
        <Avatar className={cn('shrink-0', className)}>
            {image && <AvatarImage src={image} alt="" />}
            <AvatarFallback className={cn('bg-secondary text-sm font-semibold text-foreground', fallbackClassName)}>
                {fallback}
            </AvatarFallback>
        </Avatar>
    );
}
