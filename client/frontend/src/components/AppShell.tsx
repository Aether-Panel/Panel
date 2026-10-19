'use client';
import { useAuth, Providers } from '@/contexts/providers';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { Badge } from '@/components/ui/badge';
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
    useSidebar,
} from '@/components/ui/sidebar';
import {
    ArrowUpRight,
    ChevronRight,
    ChevronsUpDown,
    Database,
    FileText,
    LayoutDashboard,
    LogOut,
    Menu,
    Network,
    Server,
    Settings,
    ShieldCheck,
    Sparkles,
    Users,
} from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { Logo } from '@/components/logo';
import { QuickSearch, type QuickSearchItem } from '@/components/quick-search';
import { useTranslations } from '@/contexts/translations-context';
import { useConfig } from '@/contexts/config-context';
import SileoToaster from '@/components/SileoToaster';
import { cn } from '@/lib/utils';
import { useEffect, useMemo } from 'react';
import type { ReactNode } from 'react';
import { sileo } from '@/lib/toast';
import { api } from '@/lib/api-client';

interface NavItem {
    href: string;
    label: string;
    icon: LucideIcon;
    requiredScopes: string[];
}

/** i18n key per URL segment; ids (`/servers/42`) fall back to the raw value. */
const SEGMENT_LABEL_KEYS: Record<string, string> = {
    dashboard: 'sidebar.dashboard',
    servers: 'sidebar.servers',
    users: 'sidebar.users',
    roles: 'sidebar.roles',
    nodes: 'sidebar.nodes',
    'database-hosts': 'sidebar.databaseHosts',
    templates: 'sidebar.templates',
    settings: 'sidebar.settings',
    profile: 'profileSettings.title',
};

/**
 * Integrated top bar: navigation breadcrumbs on the left and quick search
 * on the right. Replaces the old mobile-only bar, so desktop and mobile
 * share one header.
 */
function AppTopBar({ currentPath, searchItems }: { currentPath: string; searchItems: QuickSearchItem[] }) {
    const { t } = useTranslations();
    const { toggleSidebar } = useSidebar();

    const crumbs = useMemo(() => {
        const segments = currentPath.split('/').filter(Boolean);
        const trail = [{ key: 'root', label: t('header.root'), href: '/dashboard/' }];
        segments.forEach((segment, index) => {
            const labelKey = SEGMENT_LABEL_KEYS[segment];
            trail.push({
                key: `${segment}-${index}`,
                label: labelKey ? t(labelKey) : segment,
                href: `/${segments.slice(0, index + 1).join('/')}/`,
            });
        });
        return trail;
    }, [currentPath, t]);

    return (
        <header className="sticky top-0 z-40 flex h-16 shrink-0 items-center gap-2 border-b border-border/60 bg-surface-1/85 px-3 backdrop-blur-xl md:gap-3 md:px-6">
            <Button
                variant="ghost"
                size="icon"
                onClick={toggleSidebar}
                aria-label={t('sidebar.toggleMenu')}
                className="h-9 w-9 shrink-0 rounded-lg border border-border/60 text-muted-foreground hover:border-border hover:bg-accent/60 hover:text-foreground md:hidden"
            >
                <Menu aria-hidden="true" />
            </Button>

            <nav aria-label={t('header.breadcrumbs')} className="min-w-0 flex-1">
                <ol className="flex min-w-0 items-center gap-1 text-xs sm:gap-1.5 sm:text-sm">
                    {crumbs.map((crumb, index) => {
                        const isLast = index === crumbs.length - 1;
                        return (
                            <li
                                key={crumb.key}
                                className={cn('flex min-w-0 items-center gap-1 sm:gap-1.5', isLast && 'flex-1')}
                            >
                                {index > 0 && (
                                    <ChevronRight
                                        className="h-3.5 w-3.5 shrink-0 text-muted-foreground/70"
                                        aria-hidden="true"
                                    />
                                )}
                                {isLast ? (
                                    <span aria-current="page" title={crumb.label} className="truncate font-medium text-foreground">
                                        {crumb.label}
                                    </span>
                                ) : (
                                    <a
                                        href={crumb.href}
                                        className="truncate rounded px-1 py-0.5 text-muted-foreground transition-colors duration-200 ease-panel hover:bg-accent/60 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70"
                                    >
                                        {crumb.label}
                                    </a>
                                )}
                            </li>
                        );
                    })}
                </ol>
            </nav>

            <QuickSearch items={searchItems} />
        </header>
    );
}

function AppLayoutInner({ children, currentPath }: { children: ReactNode; currentPath: string }) {
    const { role, user, logout, hasScope } = useAuth();
    const { t } = useTranslations();
    const { config } = useConfig();
    const showHeaderDecorations = config?.headerDecorations !== false;

    useEffect(() => {
        const checkGlobalUpdate = async () => {
            if (!config?.version || !hasScope('settings.edit')) {
                return;
            }

            // Only check once per session to avoid spamming the GitHub API and user
            // (Temporarily disabled for testing)
            /*
            if (sessionStorage.getItem('hasCheckedForUpdates')) {
                return;
            }
            */

            try {
                const data = await api.get<{ current: string; latest: string; updateAvailable: boolean }>('/api/settings/update-check');

                if (data.updateAvailable) {
                    console.log("Update available! Firing toast...");

                    // Small delay to ensure toaster is mounted
                    setTimeout(() => {
                        sileo.info({
                            title: 'Update Available',
                            description: 'A new version of Aether Panel is available for installation in Settings.',
                        });
                    }, 500);
                } else {
                    console.log("System is up to date. No toast needed.");
                }
                sessionStorage.setItem('hasCheckedForUpdates', 'true');
            } catch (err) {
                console.error("Global update check failed:", err);
            }
        };

        void checkGlobalUpdate();
    }, [config?.version, hasScope]);

    const navItems: NavItem[] = [
        { href: '/dashboard/', label: t('sidebar.dashboard'), icon: LayoutDashboard, requiredScopes: [] },
        { href: '/servers/', label: t('sidebar.servers'), icon: Server, requiredScopes: ['server.view'] },
        { href: '/users/', label: t('sidebar.users'), icon: Users, requiredScopes: ['users.info.view'] },
        { href: '/roles/', label: t('sidebar.roles'), icon: ShieldCheck, requiredScopes: ['admin'] },
        { href: '/nodes/', label: t('sidebar.nodes'), icon: Network, requiredScopes: ['nodes.view'] },
        { href: '/database-hosts/', label: t('sidebar.databaseHosts'), icon: Database, requiredScopes: ['admin'] },
        { href: '/templates/', label: t('sidebar.templates'), icon: FileText, requiredScopes: ['templates.view'] },
        { href: '/settings/', label: t('sidebar.settings'), icon: Settings, requiredScopes: ['settings.edit'] },
    ];

    const hasPermission = (item: NavItem) => {
        if (role === 'admin') return true;
        if (item.requiredScopes.length === 0) return true;
        return item.requiredScopes.some(s => hasScope(s));
    };

    // Sections are only rendered once they hold at least one permitted item, so
    // a limited user never sees an orphaned group heading.
    const sections = [
        { id: 'overview', label: t('sidebar.groups.overview'), items: navItems.filter((item) => item.requiredScopes.length === 0) },
        { id: 'management', label: t('sidebar.groups.management'), items: navItems.filter((item) => item.requiredScopes.length > 0) },
    ]
        .map((section) => ({ ...section, items: section.items.filter(hasPermission) }))
        .filter((section) => section.items.length > 0);

    // Quick search reuses the sidebar navigation model, so it can only ever
    // suggest destinations the current user is allowed to open.
    const searchItems: QuickSearchItem[] = [
        ...sections.flatMap((section) =>
            section.items.map((item) => ({
                href: item.href,
                label: item.label,
                group: section.label,
                icon: item.icon,
            })),
        ),
        {
            href: '/profile/settings/',
            label: t('userMenu.settings'),
            group: t('sidebar.groups.account'),
            icon: Settings,
        },
    ];

    const isAdmin = role === 'admin';
    const roleLabel = isAdmin ? t('sidebar.roleBadge.admin') : t('sidebar.roleBadge.user');
    const displayName = user?.username || '';
    const initials = displayName.trim().slice(0, 2).toUpperCase() || '?';
    const avatarSrc = user?.avatar || (user?.email ? `https://avatar.vercel.sh/${user.email}` : undefined);

    return (
        <SidebarProvider>
            <Sidebar collapsible="offcanvas" mobileCloseLabel={t('sidebar.closeMenu')}>
                <SidebarHeader className="relative overflow-hidden border-b border-sidebar-border/60 p-0">
                    <img
                        src="/img/Fondos/minecraft-shaders-anime-hd-wallpaper-preview.jpg"
                        alt=""
                        aria-hidden="true"
                        className="pointer-events-none absolute inset-0 h-full w-full object-cover opacity-30"
                    />
                    <div className="pointer-events-none absolute inset-0 bg-gradient-to-b from-transparent via-sidebar/40 to-sidebar" />
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
                                className="pointer-events-none absolute bottom-1 right-1 h-8 w-8 rounded object-cover opacity-60"
                            />
                        </>
                    )}
                    <div className={cn("relative px-4 pt-5", showHeaderDecorations ? "pb-14" : "pb-4")}>
                        <Logo />
                    </div>
                </SidebarHeader>
                <SidebarContent className="custom-scrollbar">
                    {sections.map((section) => (
                        <SidebarGroup key={section.id}>
                            <SidebarGroupLabel>{section.label}</SidebarGroupLabel>
                            <SidebarGroupContent>
                                <SidebarMenu>
                                    {section.items.map((item) => (
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
                                    ))}
                                </SidebarMenu>
                            </SidebarGroupContent>
                        </SidebarGroup>
                    ))}
                    <a
                        href="https://www.aetherpanel.es/"
                        target="_blank"
                        rel="noopener noreferrer"
                        className="mt-auto px-2 pb-1 group-data-[collapsible=icon]:hidden"
                    >
                        <div className="flex items-center justify-between gap-2 rounded-lg border border-sidebar-border/70 bg-gradient-to-br from-primary/15 via-sidebar-accent/40 to-transparent px-3 py-2.5 transition-colors duration-200 ease-panel hover:border-primary/40 hover:bg-primary/10">
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
                        <DropdownMenuTrigger asChild>
                            <Button
                                variant="ghost"
                                className="group/user h-14 w-full justify-start gap-3 rounded-xl border border-sidebar-border/60 bg-sidebar-accent/40 px-2.5 text-left hover:border-sidebar-border hover:bg-sidebar-accent data-[state=open]:border-primary/40 data-[state=open]:bg-sidebar-accent"
                            >
                                <Avatar className="h-9 w-9 ring-1 ring-sidebar-border/70 transition-shadow duration-200 ease-panel group-hover/user:ring-primary/40">
                                    <AvatarImage src={avatarSrc} alt={displayName} />
                                    <AvatarFallback className="bg-gradient-to-br from-primary/25 via-sidebar-accent to-sidebar text-[13px] font-semibold text-sidebar-foreground">
                                        {initials}
                                    </AvatarFallback>
                                </Avatar>
                                <div className="flex min-w-0 flex-1 flex-col items-start gap-1">
                                    <div className="flex w-full min-w-0 items-center gap-1.5">
                                        <span className="truncate text-sm font-medium leading-none text-sidebar-foreground">{displayName}</span>
                                        <Badge
                                            variant={isAdmin ? 'default' : 'secondary'}
                                            className="rounded-full px-1.5 py-px text-[9px] uppercase tracking-[0.1em]"
                                        >
                                            {roleLabel}
                                        </Badge>
                                    </div>
                                    <span className="w-full truncate text-[11px] leading-none text-muted-foreground">{user?.email}</span>
                                </div>
                                <ChevronsUpDown className="shrink-0 text-sidebar-foreground/40 transition-colors duration-200 ease-panel group-hover/user:text-sidebar-foreground/70 group-data-[state=open]/user:text-sidebar-foreground/70" />
                            </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent side="right" align="end" sideOffset={8} className="w-64 shadow-elevation-3">
                            <DropdownMenuLabel className="font-normal">
                                <div className="flex items-center gap-3">
                                    <Avatar className="h-10 w-10 ring-1 ring-border/60">
                                        <AvatarImage src={avatarSrc} alt={displayName} />
                                        <AvatarFallback className="bg-gradient-to-br from-primary/25 via-sidebar-accent to-sidebar text-sm font-semibold">
                                            {initials}
                                        </AvatarFallback>
                                    </Avatar>
                                    <div className="flex min-w-0 flex-col gap-1">
                                        <span className="truncate text-sm font-medium leading-none">{displayName}</span>
                                        <span className="truncate text-xs leading-none text-muted-foreground">{user?.email}</span>
                                    </div>
                                </div>
                                <Badge
                                    variant={isAdmin ? 'default' : 'secondary'}
                                    className="mt-3 w-fit rounded-full px-2 py-px text-[9px] uppercase tracking-[0.1em]"
                                >
                                    {roleLabel}
                                </Badge>
                            </DropdownMenuLabel>
                            <DropdownMenuSeparator />
                            <DropdownMenuItem asChild className="cursor-pointer py-2">
                                <a href="/profile/settings/">
                                    <Settings />
                                    <span>{t('userMenu.settings')}</span>
                                </a>
                            </DropdownMenuItem>
                            <DropdownMenuSeparator />
                            <DropdownMenuItem
                                onClick={logout}
                                className="cursor-pointer py-2 text-destructive-bright focus:bg-destructive/10"
                            >
                                <LogOut />
                                <span>{t('userMenu.logout')}</span>
                            </DropdownMenuItem>
                        </DropdownMenuContent>
                    </DropdownMenu>
                </SidebarFooter>
            </Sidebar>

            <SidebarInset>
                <AppTopBar currentPath={currentPath} searchItems={searchItems} />

                <div className="p-4 md:p-6 lg:p-8">
                    <div className="animate-in fade-in duration-500 space-y-6">
                        {children}
                    </div>
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
