'use client';

import { Toaster } from 'sileo';

export default function SileoToaster() {
    return (
        <Toaster
            position="bottom-right"
            offset={16}
            options={{
                fill: '#181b26',
                roundness: 10,
                duration: 4500,
                styles: {
                    title: 'normal-case! text-[13px] font-semibold leading-tight! text-white!',
                    description: 'text-[13px] leading-snug! text-white/75!',
                    button: 'bg-white/10! hover:bg-white/20! text-white!',
                },
            }}
        />
    );
}