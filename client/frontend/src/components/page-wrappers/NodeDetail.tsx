import AppShell from '@/components/AppShell';
import NodeDetailPage from '@/features/nodes/[id]/page';

export default function NodeDetail({ id }: { id?: string }) {
    const searchParams = typeof window !== 'undefined' ? new URLSearchParams(window.location.search) : null;
    const effectiveId = id || searchParams?.get('id') || '';
    return <AppShell currentPath={`/nodes/${effectiveId}`}><NodeDetailPage params={{ id: effectiveId }} /></AppShell>;
}
