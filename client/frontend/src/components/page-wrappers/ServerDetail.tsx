import AppShell from '@/components/AppShell';
import ServerDetailPage from '@/features/servers/[id]/page';

export default function ServerDetail({ id }: Readonly<{ id?: string }>) {
    const searchParams = typeof window !== 'undefined' ? new URLSearchParams(window.location.search) : null;
    const effectiveId = id || searchParams?.get('id') || '';
    return <AppShell currentPath={`/servers/${effectiveId}`}><ServerDetailPage params={{ id: effectiveId }} /></AppShell>;
}
