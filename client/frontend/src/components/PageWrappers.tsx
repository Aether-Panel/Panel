import React from 'react';
import AppShell from '@/components/AppShell';
import AuthShell from '@/components/AuthShell';

import DashboardPage from '@/features/dashboard/page';
import ServersPage from '@/features/servers/page';
import ServerDetailPage from '@/features/servers/[id]/page';
import NodesPage from '@/features/nodes/page';
import NodeDetailPage from '@/features/nodes/[id]/page';
import UsersPage from '@/features/users/page';
import RolesPage from '@/features/roles/page';
import DatabaseHostsPage from '@/features/database-hosts/page';
import TemplatesPage from '@/features/templates/page';
import SettingsPage from '@/features/settings/page';
import ProfileSettingsPage from '@/features/profile/settings/page';
import LoginPage from '@/features/auth/login/page';
import RegisterPage from '@/features/auth/register/page';

export function Dashboard() {
    return <AppShell currentPath="/dashboard/"><DashboardPage /></AppShell>;
}

export function Servers() {
    return <AppShell currentPath="/servers/"><ServersPage /></AppShell>;
}

export function ServerDetail({ id }: { id?: string }) {
    const searchParams = typeof window !== 'undefined' ? new URLSearchParams(window.location.search) : null;
    const effectiveId = id || searchParams?.get('id') || '';
    return <AppShell currentPath={`/servers/${effectiveId}`}><ServerDetailPage params={{ id: effectiveId }} /></AppShell>;
}

export function Nodes() {
    return <AppShell currentPath="/nodes/"><NodesPage /></AppShell>;
}

export function NodeDetail({ id }: { id?: string }) {
    const searchParams = typeof window !== 'undefined' ? new URLSearchParams(window.location.search) : null;
    const effectiveId = id || searchParams?.get('id') || '';
    return <AppShell currentPath={`/nodes/${effectiveId}`}><NodeDetailPage params={{ id: effectiveId }} /></AppShell>;
}

export function Users() {
    return <AppShell currentPath="/users/"><UsersPage /></AppShell>;
}

export function Roles() {
    return <AppShell currentPath="/roles/"><RolesPage /></AppShell>;
}

export function DatabaseHosts() {
    return <AppShell currentPath="/database-hosts/"><DatabaseHostsPage /></AppShell>;
}

export function Templates() {
    return <AppShell currentPath="/templates/"><TemplatesPage /></AppShell>;
}

export function Settings() {
    return <AppShell currentPath="/settings/"><SettingsPage /></AppShell>;
}

export function ProfileSettings() {
    return <AppShell currentPath="/profile/settings/"><ProfileSettingsPage /></AppShell>;
}

export function Login() {
    return <AuthShell><LoginPage /></AuthShell>;
}

export function Register() {
    return <AuthShell><RegisterPage /></AuthShell>;
}
