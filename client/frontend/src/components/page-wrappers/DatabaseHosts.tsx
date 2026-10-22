import AppShell from '@/components/AppShell';
import DatabaseHostsPage from '@/features/database-hosts/page';

export default function DatabaseHosts() {
    return <AppShell currentPath="/database-hosts/"><DatabaseHostsPage /></AppShell>;
}
