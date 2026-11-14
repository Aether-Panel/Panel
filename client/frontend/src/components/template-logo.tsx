import { useState, type ReactNode } from 'react';
import { cn } from '@/lib/utils';
import type { TemplateLogo as Logo } from '@/lib/template-logos';

/**
 * A template's logo in a square of the size `className` gives it. Brand marks drawn on a
 * transparent background sit on a light plate so they read on both themes. Decorative: the name
 * shown next to it says what it is. Without a logo, or when the file fails to load, it shows
 * `fallback` (nothing by default), never a broken image.
 */
export function TemplateLogo({ logo, className, fallback = null }: Readonly<{
    logo: Logo | undefined;
    className?: string;
    fallback?: ReactNode;
}>) {
    const [failedSrc, setFailedSrc] = useState<string>();
    if (!logo || failedSrc === logo.src) return <>{fallback}</>;
    return (
        <span
            className={cn(
                'inline-grid shrink-0 place-items-center overflow-hidden rounded',
                // no padding on the box: a percentage padding is taken from the parent's width, not
                // the logo's, and grew the box while squeezing the image to nothing
                logo.plate && 'bg-white ring-1 ring-inset ring-black/10',
                className,
            )}
        >
            <img
                src={logo.src}
                alt=""
                loading="lazy"
                decoding="async"
                draggable={false}
                onError={() => setFailedSrc(logo.src)}
                className={cn('object-contain', logo.plate ? 'h-[76%] w-[76%]' : 'h-full w-full')}
            />
        </span>
    );
}
