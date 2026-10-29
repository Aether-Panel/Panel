'use client';

import { useMemo } from 'react';
import { Blocks } from '@/components/icons';
import { PageHeader } from '@/components/page-header';
import { EmptyState } from '@/components/empty-state';
import { Skeleton } from '@/components/ui/skeleton';
import { useTranslations } from '@/contexts/translations-context';
import { useActiveExtensions } from '@/hooks/use-extensions';
import { ExtensionFrame } from './extension-frame';

/** An extension's administration page, opened from the menu: /extension/?id=<id>&page=<page>. */
export default function ExtensionPage() {
    const { t } = useTranslations();
    const { extensions, ready } = useActiveExtensions();

    const found = useMemo(() => {
        const params = new URLSearchParams(window.location.search);
        const extension = extensions.find((e) => e.id === params.get('id'));
        const page = extension?.pages.find((p) => p.id === params.get('page') && p.placement === 'admin');
        return extension && page ? { extension, page } : null;
    }, [extensions]);

    if (!ready) {
        return (
            <div className="flex flex-col gap-6" aria-busy="true">
                <span className="sr-only">{t('common.loading')}</span>
                <Skeleton className="h-10 w-64" />
                <Skeleton className="h-[60dvh] w-full rounded-xl" />
            </div>
        );
    }

    if (!found) {
        return (
            <EmptyState
                icon={Blocks}
                title={t('extensions.page.notFoundTitle')}
                description={t('extensions.page.notFoundDescription')}
            />
        );
    }

    return (
        <div className="stagger flex flex-col gap-6 pb-12">
            <PageHeader title={found.page.title} description={t('extensions.page.from', { name: found.extension.name })} />
            <ExtensionFrame src={found.page.url} title={`${found.page.title} · ${found.extension.name}`} />
        </div>
    );
}
