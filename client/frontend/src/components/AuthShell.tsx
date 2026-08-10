'use client';
import { Providers } from '@/contexts/providers';
import SileoToaster from '@/components/SileoToaster';
import type { ReactNode } from 'react';

export default function AuthShell({ children }: { children: ReactNode }) {
    return (
        <Providers>
            <main className="animate-in fade-in duration-500">
                {children}
            </main>
            <SileoToaster />
        </Providers>
    );
}
