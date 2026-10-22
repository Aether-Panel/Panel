'use client';
import { useAuth, Providers } from '@/contexts/providers';
import { UserAvatar } from '@/components/user-avatar';
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
    Users,
} from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { Logo } from '@/components/logo';
import { AmbientBackdrop } from '@/components/ambient-backdrop';
import { QuickSearch, type QuickSearchItem } from '@/components/quick-search';
import { useTranslations } from '@/contexts/translations-context';
import { useConfig } from '@/contexts/config-context';
import SileoToaster from '@/components/SileoToaster';
import { cn } from '@/lib/utils';
import { useEffect, useLayoutEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { sileo } from '@/lib/toast';
import { useIsMobile } from '@/hooks/use-mobile';
import { api } from '@/lib/api-client';

interface NavItem {
    href: string;
    label: string;
    icon: LucideIcon;
    requiredScopes: string[];
}

/**
 * i18n key per URL segment. Ids (`/servers/42`) show the entity name once the
 * detail page publishes it (see BREADCRUMB_EVENT), and the raw id until then.
 */
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

/** Detail pages dispatch this with `{ id, label }` once the entity name is known. */
const BREADCRUMB_EVENT = 'aether:breadcrumb';

/**
 * Integrated top bar: navigation breadcrumbs on the left and quick search
 * on the right. Replaces the old mobile-only bar, so desktop and mobile
 * share one header.
 */
function AppTopBar({ currentPath, searchItems }: { currentPath: string; searchItems: QuickSearchItem[] }) {
    const { t } = useTranslations();
    const { toggleSidebar } = useSidebar();
    const [entityLabels, setEntityLabels] = useState<Record<string, string>>({});

    // Layout effect: registered before any page effect can dispatch.
    useLayoutEffect(() => {
        const handleLabel = (event: Event) => {
            const detail = (event as CustomEvent<{ id?: string | number; label?: string }>).detail;
            if (detail?.id == null || !detail.label) return;
            const id = String(detail.id);
            const label = detail.label;
            setEntityLabels((prev) => (prev[id] === label ? prev : { ...prev, [id]: label }));
        };
        window.addEventListener(BREADCRUMB_EVENT, handleLabel);
        return () => window.removeEventListener(BREADCRUMB_EVENT, handleLabel);
    }, []);

    const crumbs = useMemo(() => {
        const segments = currentPath.split('/').filter(Boolean);
        const trail: { key: string; label: string; href: string; isId?: boolean }[] = [
            { key: 'root', label: t('header.root'), href: '/dashboard/' },
        ];
        // `/profile/` has no page of its own: the account settings are one crumb.
        if (segments[0] === 'profile') {
            trail.push({ key: 'profile', label: t('profileSettings.title'), href: '/profile/settings/' });
            return trail;
        }
        segments.forEach((segment, index) => {
            const labelKey = SEGMENT_LABEL_KEYS[segment];
            const entityLabel = labelKey ? undefined : entityLabels[segment];
            trail.push({
                key: `${segment}-${index}`,
                label: labelKey ? t(labelKey) : entityLabel ?? segment,
                href: `/${segments.slice(0, index + 1).join('/')}/`,
                isId: !labelKey && !entityLabel,
            });
        });
        return trail;
    }, [currentPath, t, entityLabels]);
    // On phones a deep trail drops the root crumb so the current page keeps its room.
    const collapseRoot = crumbs.length > 2;

    return (
        <header className="sticky top-0 z-40 flex h-16 shrink-0 items-center gap-2 border-b border-border/60 bg-background/80 px-3 backdrop-blur-xl md:gap-3 md:px-6">
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
                <ol className="flex min-w-0 items-center gap-1 text-sm sm:gap-1.5">
                    {crumbs.map((crumb, index) => {
                        const isLast = index === crumbs.length - 1;
                        return (
                            <li
                                key={crumb.key}
                                className={cn(
                                    'flex min-w-0 items-center gap-1 sm:gap-1.5',
                                    isLast ? 'flex-1' : 'max-w-[45%] shrink sm:max-w-none',
                                    index === 0 && collapseRoot && 'hidden sm:flex',
                                )}
                            >
                                {index > 0 && (
                                    <ChevronRight
                                        className={cn(
                                            'h-3.5 w-3.5 shrink-0 text-muted-foreground/70',
                                            index === 1 && collapseRoot && 'hidden sm:block',
                                        )}
                                        aria-hidden="true"
                                    />
                                )}
                                {isLast ? (
                                    <span
                                        aria-current="page"
                                        title={crumb.label}
                                        className={cn('truncate px-1 font-medium text-foreground', crumb.isId && 'font-mono text-[13px]')}
                                    >
                                        {crumb.label}
                                    </span>
                                ) : (
                                    <a
                                        href={crumb.href}
                                        title={crumb.label}
                                        className={cn(
                                            'truncate rounded-md px-1 py-0.5 text-muted-foreground transition-colors duration-200 ease-panel hover:bg-accent/60 hover:text-foreground focus-ring',
                                            crumb.isId && 'font-mono text-[13px]',
                                        )}
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
            if (sessionStorage.getItem('hasCheckedForUpdates')) {
                return;
            }

            try {
                const data = await api.get<{ current: string; latest: string; updateAvailable: boolean }>('/api/settings/update-check');

                if (data.updateAvailable) {
                    // Small delay to ensure toaster is mounted
                    setTimeout(() => {
                        sileo.info({
                            title: t('header.update.title'),
                            description: t('header.update.description'),
                        });
                    }, 500);
                }
                sessionStorage.setItem('hasCheckedForUpdates', 'true');
            } catch (err) {
                console.error("Global update check failed:", err);
            }
        };

        void checkGlobalUpdate();
        // `t` is read when the toast fires; re-running the check on a language switch is not wanted.
        // eslint-disable-next-line react-hooks/exhaustive-deps
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
            label: t('profileSettings.title'),
            group: t('sidebar.groups.account'),
            icon: Settings,
        },
    ];

    const isAdmin = role === 'admin';
    const roleLabel = isAdmin ? t('sidebar.roleBadge.admin') : t('sidebar.roleBadge.user');
    const displayName = user?.username || '';
    const initials = displayName.trim().slice(0, 2).toUpperCase() || '?';
    const avatarSrc = user?.avatar || undefined;
    const isMobile = useIsMobile();

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
                        className="group/powered mx-2 mb-1 mt-auto flex items-center justify-between gap-2 rounded-md px-2 py-1.5 text-xs text-sidebar-foreground/55 transition-colors duration-200 ease-panel hover:text-sidebar-foreground focus-ring group-data-[collapsible=icon]:hidden"
                    >
                        <span>{t('sidebar.system.poweredBy')}</span>
                        <ArrowUpRight aria-hidden="true" className="h-3.5 w-3.5 opacity-60 transition-opacity duration-200 ease-panel group-hover/powered:opacity-100" />
                    </a>
                </SidebarContent>
                <SidebarFooter className="border-t border-sidebar-border/60 p-3">
                    <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                            <Button
                                variant="ghost"
                                className="group/user h-14 w-full justify-start gap-3 rounded-xl border border-sidebar-border/60 bg-sidebar-accent/40 px-2.5 text-left hover:border-sidebar-border hover:bg-sidebar-accent data-[state=open]:border-primary/40 data-[state=open]:bg-sidebar-accent"
                            >
                                <UserAvatar
                                    identity={user?.email || displayName}
                                    src={avatarSrc}
                                    fallback={initials}
                                    className="h-9 w-9 ring-1 ring-sidebar-border/70 transition-shadow duration-200 ease-panel group-hover/user:ring-primary/40"
                                    fallbackClassName="bg-primary/15 text-[13px] text-primary-bright"
                                />
                                <div className="flex min-w-0 flex-1 flex-col items-start gap-1">
                                    <div className="flex w-full min-w-0 items-center gap-1.5">
                                        <span className="truncate text-sm font-medium leading-none text-sidebar-foreground">{displayName}</span>
                                        <Badge
                                            variant={isAdmin ? 'default' : 'secondary'}
                                            className="rounded-full px-1.5 py-px text-[10px]"
                                        >
                                            {roleLabel}
                                        </Badge>
                                    </div>
                                    <span className="w-full truncate text-[11px] leading-none text-muted-foreground">{user?.email}</span>
                                </div>
                                <ChevronsUpDown aria-hidden="true" className="shrink-0 text-sidebar-foreground/40 transition-colors duration-200 ease-panel group-hover/user:text-sidebar-foreground/70 group-data-[state=open]/user:text-sidebar-foreground/70" />
                            </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent
                            side={isMobile ? 'top' : 'right'}
                            align="end"
                            sideOffset={8}
                            className="w-[var(--radix-dropdown-menu-trigger-width)] min-w-60 rounded-xl p-1.5 shadow-elevation-3 md:w-64"
                        >
                            <DropdownMenuLabel className="px-2 py-2 font-normal">
                                <div className="flex items-center gap-3">
                                    <UserAvatar
                                        identity={user?.email || displayName}
                                        src={avatarSrc}
                                        fallback={initials}
                                        className="h-9 w-9 ring-1 ring-border/60"
                                        fallbackClassName="bg-primary/15 text-[13px] text-primary-bright"
                                    />
                                    <div className="flex min-w-0 flex-1 flex-col gap-1">
                                        <div className="flex min-w-0 items-center gap-1.5">
                                            <span className="truncate text-sm font-medium leading-none text-foreground">{displayName}</span>
                                            <Badge
                                                variant={isAdmin ? 'default' : 'secondary'}
                                                className="shrink-0 rounded-full px-1.5 py-px text-[10px]"
                                            >
                                                {roleLabel}
                                            </Badge>
                                        </div>
                                        <span className="truncate text-xs leading-none text-muted-foreground">{user?.email}</span>
                                    </div>
                                </div>
                            </DropdownMenuLabel>
                            <DropdownMenuSeparator className="my-1.5" />
                            <DropdownMenuItem asChild className="cursor-pointer gap-2.5 rounded-lg px-2 py-2">
                                <a href="/profile/settings/">
                                    <Settings aria-hidden="true" className="text-muted-foreground" />
                                    <span>{t('profileSettings.title')}</span>
                                </a>
                            </DropdownMenuItem>
                            <DropdownMenuItem
                                onClick={logout}
                                className="cursor-pointer gap-2.5 rounded-lg px-2 py-2 text-destructive-bright focus:bg-destructive/10 focus:text-destructive-bright"
                            >
                                <LogOut aria-hidden="true" />
                                <span>{t('userMenu.logout')}</span>
                            </DropdownMenuItem>
                        </DropdownMenuContent>
                    </DropdownMenu>
                </SidebarFooter>
            </Sidebar>

            <SidebarInset>
                <AmbientBackdrop />
                <AppTopBar currentPath={currentPath} searchItems={searchItems} />

                <div className="relative z-[1] p-4 md:p-6 lg:p-8">
                    <div className="page-enter space-y-6">
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
