import AppShell from '@/components/AppShell';
import DashboardPage from '@/features/dashboard/page';

export default function Dashboard() {
    return <AppShell currentPath="/dashboard/"><DashboardPage /></AppShell>;
}
