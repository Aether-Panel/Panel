import AppShell from '@/components/AppShell';
import UsersPage from '@/features/users/page';

export default function Users() {
    return <AppShell currentPath="/users/"><UsersPage /></AppShell>;
}
