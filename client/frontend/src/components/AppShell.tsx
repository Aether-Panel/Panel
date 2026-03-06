'use client';
import { useAuth, Providers } from '@/contexts/providers';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { Button } from '@/components/ui/button';
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
    Sidebar,
    SidebarContent,
    SidebarFooter,
    SidebarHeader,
    SidebarInset,
    SidebarMenu,
    SidebarMenuItem,
    SidebarMenuButton,
    SidebarProvider,
    SidebarTrigger,
} from '@/components/ui/sidebar';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { LayoutDashboard, LogOut, Server, Settings, Users, ShieldCheck, Database, Network, FileText } from 'lucide-react';
import { Logo } from '@/components/logo';
import { useTranslations } from '@/contexts/translations-context';
import { Toaster } from '@/components/ui/toaster';
import type { ReactNode } from 'react';

function AppLayoutInner({ children, currentPath }: { children: ReactNode; currentPath: string }) {
    const { role, user, logout, scopes } = useAuth();
    const { t } = useTranslations();

    const navItems = [
        { href: '/dashboard/', label: t('sidebar.dashboard'), icon: LayoutDashboard, requiredScopes: [] },
        { href: '/servers/', label: t('sidebar.servers'), icon: Server, requiredScopes: ['server.view'] },
        { href: '/users/', label: t('sidebar.users'), icon: Users, requiredScopes: ['users.info.view'] },
        { href: '/roles/', label: t('sidebar.roles'), icon: ShieldCheck, requiredScopes: ['admin'] },
        { href: '/nodes/', label: t('sidebar.nodes'), icon: Network, requiredScopes: ['nodes.view'] },
        { href: '/database-hosts/', label: t('sidebar.databaseHosts'), icon: Database, requiredScopes: ['admin'] },
        { href: '/templates/', label: t('sidebar.templates'), icon: FileText, requiredScopes: ['templates.view'] },
        { href: '/settings/', label: t('sidebar.settings'), icon: Settings, requiredScopes: ['settings.edit'] },
    ];

    const hasPermission = (item: typeof navItems[0]) => {
        if (role === 'admin') return true;
        if (item.requiredScopes.length === 0) return true;
        return item.requiredScopes.some(s => scopes.includes(s));
    };

    return (
        <SidebarProvider>
            <Sidebar>
                <SidebarHeader>
                    <Logo />
                </SidebarHeader>
                <SidebarContent>
                    <SidebarMenu>
                        {navItems.map((item) =>
                            hasPermission(item) ? (
                                <SidebarMenuItem key={item.href}>
                                    <SidebarMenuButton
                                        asChild
                                        isActive={currentPath.startsWith(item.href)}
                                        tooltip={item.label}
                                    >
                                        <a href={item.href}>
                                            <item.icon />
                                            <span>{item.label}</span>
                                        </a>
                                    </SidebarMenuButton>
                                </SidebarMenuItem>
                            ) : null
                        )}
                    </SidebarMenu>
                </SidebarContent>
                <SidebarFooter>
                    <DropdownMenu>
                        <Tooltip>
                            <TooltipTrigger asChild>
                                <DropdownMenuTrigger asChild>
                                    <Button variant="ghost" className="h-12 w-full justify-start gap-2 px-2">
                                        <Avatar className="h-8 w-8">
                                            <AvatarImage src={`https://avatar.vercel.sh/${user?.email}`} alt={user?.username} />
                                            <AvatarFallback>{user?.username?.charAt(0)}</AvatarFallback>
                                        </Avatar>
                                        <div className="flex-1 min-w-0 flex-col items-start overflow-hidden group-data-[state=collapsed]:hidden flex">
                                            <span className="w-full truncate font-medium">{user?.username}</span>
                                            <span className="w-full truncate text-xs text-muted-foreground">{user?.email}</span>
                                        </div>
                                    </Button>
                                </DropdownMenuTrigger>
                            </TooltipTrigger>
                            <TooltipContent side="right" align="center" className="hidden group-data-[state=collapsed]:block">
                                <div className="text-left">
                                    <p className="font-medium">{user?.username}</p>
                                    <p className="text-xs text-muted-foreground">{user?.email}</p>
                                </div>
                            </TooltipContent>
                        </Tooltip>
                        <DropdownMenuContent className="w-56" side="right" align="end">
                            <DropdownMenuLabel className="font-normal">
                                <div className="flex flex-col space-y-1">
                                    <p className="text-sm font-medium leading-none">{user?.username}</p>
                                    <p className="text-xs leading-none text-muted-foreground">{user?.email}</p>
                                </div>
                            </DropdownMenuLabel>
                            <DropdownMenuSeparator />
                            <a href="/profile/settings/">
                                <DropdownMenuItem className="cursor-pointer">
                                    <Settings className="mr-2" />
                                    <span>{t('userMenu.settings')}</span>
                                </DropdownMenuItem>
                            </a>
                            <DropdownMenuSeparator />
                            <DropdownMenuItem onClick={logout}>
                                <LogOut className="mr-2" />
                                <span>{t('userMenu.logout')}</span>
                            </DropdownMenuItem>
                        </DropdownMenuContent>
                    </DropdownMenu>
                </SidebarFooter>
            </Sidebar>

            <SidebarInset className="p-4 md:p-6 lg:p-8">
                <header className="mb-6 flex items-center justify-between">
                    <SidebarTrigger className="md:hidden" />
                    <div className="flex-grow" />
                </header>
                <main className="animate-in fade-in duration-500">
                    {children}
                </main>
            </SidebarInset>
        </SidebarProvider>
    );
}

export default function AppShell({ children, currentPath }: { children: ReactNode; currentPath: string }) {
    return (
        <Providers>
            <AppLayoutInner currentPath={currentPath}>
                {children}
            </AppLayoutInner>
            <Toaster />
        </Providers>
    );
}
