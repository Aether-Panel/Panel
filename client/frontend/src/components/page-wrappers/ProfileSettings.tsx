import AppShell from '@/components/AppShell';
import ProfileSettingsPage from '@/features/profile/settings/page';

export default function ProfileSettings() {
    return <AppShell currentPath="/profile/settings/"><ProfileSettingsPage /></AppShell>;
}
