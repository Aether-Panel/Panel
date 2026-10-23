import AppShell from '@/components/AppShell';
import ServersPage from '@/features/servers/page';

export default function Servers() {
    return <AppShell currentPath="/servers/"><ServersPage /></AppShell>;
}
