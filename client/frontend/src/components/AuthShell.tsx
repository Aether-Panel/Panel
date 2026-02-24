'use client';
import { Providers } from '@/contexts/providers';
import { Toaster } from '@/components/ui/toaster';
import type { ReactNode } from 'react';

export default function AuthShell({ children }: { children: ReactNode }) {
    return (
        <Providers>
            <main className="animate-in fade-in duration-500">
                {children}
            </main>
            <Toaster />
        </Providers>
    );
}
