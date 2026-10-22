import AppShell from '@/components/AppShell';
import NodesPage from '@/features/nodes/page';

export default function Nodes() {
    return <AppShell currentPath="/nodes/"><NodesPage /></AppShell>;
}
