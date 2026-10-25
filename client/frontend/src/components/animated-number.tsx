'use client';

import { useEffect, useRef, useState } from 'react';

type AnimatedNumberProps = {
    value: number;
    /** Decimal places shown while counting and at rest. */
    decimals?: number;
    /** Count duration for the first appearance; later changes take half. */
    duration?: number;
    className?: string;
};

const easeOutExpo = (t: number) => (t >= 1 ? 1 : 1 - Math.pow(2, -10 * t));

/**
 * A figure that counts up to its value on first paint and glides to each new
 * value afterwards, so live readouts change without jumping. Tabular figures
 * keep the width steady while it counts. Shows the final value straight away
 * under `prefers-reduced-motion`.
 */
export function AnimatedNumber({ value, decimals = 0, duration = 1100, className }: Readonly<AnimatedNumberProps>) {
    const [display, setDisplay] = useState(0);
    const from = useRef(0);
    const first = useRef(true);

    useEffect(() => {
        const target = Number.isFinite(value) ? value : 0;
        if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
            from.current = target;
            setDisplay(target);
            return;
        }

        const start = from.current;
        const span = first.current ? duration : duration / 2;
        first.current = false;
        let frame = 0;
        const began = performance.now();

        const tick = (now: number) => {
            const progress = Math.min(1, (now - began) / span);
            const next = start + (target - start) * easeOutExpo(progress);
            from.current = next;
            setDisplay(next);
            if (progress < 1) frame = requestAnimationFrame(tick);
        };
        frame = requestAnimationFrame(tick);
        return () => cancelAnimationFrame(frame);
    }, [value, duration]);

    return <span className={className}>{display.toFixed(decimals)}</span>;
}
