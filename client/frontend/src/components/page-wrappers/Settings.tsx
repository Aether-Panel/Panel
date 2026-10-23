import AppShell from '@/components/AppShell';
import SettingsPage from '@/features/settings/page';

export default function Settings() {
    return <AppShell currentPath="/settings/"><SettingsPage /></AppShell>;
}
