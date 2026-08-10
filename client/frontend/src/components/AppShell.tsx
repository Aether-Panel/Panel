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
    SidebarGroup,
    SidebarGroupContent,
    SidebarGroupLabel,
    SidebarHeader,
    SidebarInset,
    SidebarMenu,
    SidebarMenuItem,
    SidebarMenuButton,
    SidebarProvider,
    SidebarRail,
    SidebarTrigger,
} from '@/components/ui/sidebar';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { ArrowUpRight, LayoutDashboard, LogOut, Server, Settings, Sparkles, Users, ShieldCheck, Database, Network, FileText } from 'lucide-react';
import { Logo } from '@/components/logo';
import { useTranslations } from '@/contexts/translations-context';
import { useConfig } from '@/contexts/config-context';
import SileoToaster from '@/components/SileoToaster';
import { cn } from '@/lib/utils';
import type { ReactNode } from 'react';

function AppLayoutInner({ children, currentPath }: { children: ReactNode; currentPath: string }) {
    const { role, user, logout, scopes } = useAuth();
    const { t } = useTranslations();
    const { config } = useConfig();
    const showHeaderDecorations = config?.headerDecorations !== false;

    const navItems = [
        { href: '/dashboard/', label: t('sidebar.dashboard'), icon: LayoutDashboard, requiredScopes: [] },
        { href: '/servers/', label: t('sidebar.servers'), icon: Server, requiredScopes: ['admin'] },
        { href: '/users/', label: t('sidebar.users'), icon: Users, requiredScopes: ['users.info.view'] },
        { href: '/roles/', label: t('sidebar.roles'), icon: ShieldCheck, requiredScopes: ['admin'] },
        { href: '/nodes/', label: t('sidebar.nodes'), icon: Network, requiredScopes: ['nodes.view'] },
        { href: '/database-hosts/', label: t('sidebar.databaseHosts'), icon: Database, requiredScopes: ['admin'] },
        { href: '/templates/', label: t('sidebar.templates'), icon: FileText, requiredScopes: ['templates.view'] },
        { href: '/settings/', label: t('sidebar.settings'), icon: Settings, requiredScopes: ['settings.edit'] },
    ];

    const overviewItems = navItems.filter((item) => item.requiredScopes.length === 0);
    const managementItems = navItems.filter((item) => item.requiredScopes.length > 0);

    const hasPermission = (item: typeof navItems[0]) => {
        if (role === 'admin') return true;
        if (item.requiredScopes.length === 0) return true;
        return item.requiredScopes.some(s => scopes.includes(s));
    };

    const renderItems = (items: typeof navItems) =>
        items.map((item) =>
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
        );

    return (
        <SidebarProvider>
            <Sidebar collapsible="offcanvas">
                <SidebarHeader className="relative overflow-hidden border-b border-sidebar-border/60 p-0">
                    <img
                        src="/img/Fondos/minecraft-shaders-anime-hd-wallpaper-preview.jpg"
                        alt=""
                        aria-hidden="true"
                        className="pointer-events-none absolute inset-0 h-full w-full object-cover opacity-30"
                    />
                    <div className="pointer-events-none absolute inset-0 bg-gradient-to-b from-transparent via-transparent to-sidebar" />
                    {showHeaderDecorations && (
                        <>
                            <img
                                src="/img/Gifs/animepixel1.gif"
                                alt=""
                                aria-hidden="true"
                                className="pointer-events-none absolute bottom-1 left-1 h-8 w-8 rounded object-cover opacity-60"
                            />
                            <video
                                src="/img/Gifs/b810142e63c0e12ac415d884e1e01ff5_t1.webm"
                                autoPlay
                                muted
                                loop
                                playsInline
                                aria-hidden="true"
                                className="pointer-events-none absolute bottom-1 right-1 h-8 w-8 rounded object-cover opacity-60"
                            />
                        </>
                    )}
                    <div className={cn("relative px-4 pt-5", showHeaderDecorations ? "pb-14" : "pb-4")}>
                        <Logo />
                    </div>
                </SidebarHeader>
                <SidebarContent>
                    <SidebarGroup>
                        <SidebarGroupLabel className="px-2 text-[11px] font-medium uppercase tracking-wider text-sidebar-foreground/40">
                            {t('sidebar.groups.overview')}
                        </SidebarGroupLabel>
                        <SidebarGroupContent>
                            <SidebarMenu>{renderItems(overviewItems)}</SidebarMenu>
                        </SidebarGroupContent>
                    </SidebarGroup>
                    <SidebarGroup>
                        <SidebarGroupLabel className="px-2 text-[11px] font-medium uppercase tracking-wider text-sidebar-foreground/40">
                            {t('sidebar.groups.management')}
                        </SidebarGroupLabel>
                        <SidebarGroupContent>
                            <SidebarMenu>{renderItems(managementItems)}</SidebarMenu>
                        </SidebarGroupContent>
                    </SidebarGroup>
                    <a
                        href="https://www.aetherpanel.es/"
                        target="_blank"
                        rel="noopener noreferrer"
                        className="mt-auto px-2 pb-1 group-data-[collapsible=icon]:hidden"
                    >
                        <div className="flex items-center justify-between gap-2 rounded-lg border border-sidebar-border/70 bg-gradient-to-br from-primary/15 via-sidebar-accent/40 to-transparent px-3 py-2.5 transition-colors hover:border-primary/40 hover:bg-primary/10">
                            <div className="flex items-center gap-2">
                                <Sparkles className="h-3.5 w-3.5 text-primary" />
                                <span className="text-[11px] font-medium text-sidebar-foreground/70">{t('sidebar.system.poweredBy')}</span>
                            </div>
                            <ArrowUpRight className="h-3.5 w-3.5 text-sidebar-foreground/40" />
                        </div>
                    </a>
                </SidebarContent>
                <SidebarFooter className="border-t border-sidebar-border/60 p-3">
                    <DropdownMenu>
                        <Tooltip>
                            <TooltipTrigger asChild>
                                <DropdownMenuTrigger asChild>
                                    <Button variant="ghost" className="h-12 w-full justify-start gap-2.5 rounded-lg px-2 hover:bg-sidebar-accent transition-all duration-200">
                                        <Avatar className="h-8 w-8 ring-1 ring-sidebar-border">
                                            <AvatarImage src={`https://avatar.vercel.sh/${user?.email}`} alt={user?.username} />
                                            <AvatarFallback>{user?.username?.charAt(0)}</AvatarFallback>
                                        </Avatar>
                                        <div className="flex-1 min-w-0 flex-col items-start overflow-hidden flex">
                                            <div className="flex w-full items-center gap-1.5">
                                                <span className="truncate font-medium text-sm">{user?.username}</span>
                                                <span className="shrink-0 rounded border border-sidebar-border px-1 py-px text-[9px] font-medium uppercase tracking-wider text-sidebar-foreground/50">
                                                    {role === 'admin' ? t('sidebar.roleBadge.admin') : t('sidebar.roleBadge.user')}
                                                </span>
                                            </div>
                                            <span className="w-full truncate text-xs text-muted-foreground">{user?.email}</span>
                                        </div>
                                    </Button>
                                </DropdownMenuTrigger>
                            </TooltipTrigger>
                            <TooltipContent side="right" align="center">
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
                <SidebarRail />
            </Sidebar>

            <SidebarInset>
                {/* Mobile top bar with menu button — desktop hidden */}
                <header className="flex h-12 items-center gap-3 border-b border-border/50 bg-background/95 backdrop-blur px-4 sticky top-0 z-40 md:hidden">
                    <SidebarTrigger className="h-8 w-8" />
                    <span className="text-sm font-medium text-muted-foreground">Menú</span>
                </header>

                <div className="p-4 md:p-6 lg:p-8">
                    <main className="animate-in fade-in duration-500 space-y-6">
                        {children}
                    </main>
                </div>
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
            <SileoToaster />
        </Providers>
    );
}
