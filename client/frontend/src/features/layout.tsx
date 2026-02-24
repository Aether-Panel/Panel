'use client';
import React from 'react';
import { useAuth } from '@/contexts/providers';
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

export default function AppLayout({ children, currentPath = '' }: { children: React.ReactNode, currentPath?: string }) {
  const { role, user, logout } = useAuth();
  const { t } = useTranslations();

  // Use currentPath prop or fallback to browser path if available
  const [pathname, setPathname] = React.useState(currentPath);
  React.useEffect(() => {
    if (!currentPath && typeof window !== 'undefined') {
      setPathname(window.location.pathname);
    }
  }, [currentPath]);

  const navItems = [
    { href: '/dashboard', label: t('sidebar.dashboard'), icon: LayoutDashboard, roles: ['admin', 'user'] },
    { href: '/servers', label: t('sidebar.servers'), icon: Server, roles: ['admin'] },
    { href: '/users', label: t('sidebar.users'), icon: Users, roles: ['admin'] },
    { href: '/roles', label: t('sidebar.roles'), icon: ShieldCheck, roles: ['admin'] },
    { href: '/nodes', label: t('sidebar.nodes'), icon: Network, roles: ['admin'] },
    { href: '/database-hosts', label: t('sidebar.databaseHosts'), icon: Database, roles: ['admin'] },
    { href: '/templates', label: t('sidebar.templates'), icon: FileText, roles: ['admin'] },
    { href: '/settings', label: t('sidebar.settings'), icon: Settings, roles: ['admin'] },
  ];

  return (
    <SidebarProvider>
      <Sidebar>
        <SidebarHeader>
          <Logo />
        </SidebarHeader>
        <SidebarContent>
          <SidebarMenu>
            {navItems.map((item) =>
              item.roles.includes(role!) ? (
                <SidebarMenuItem key={item.href}>
                  <SidebarMenuButton
                    asChild
                    isActive={pathname.startsWith(item.href)}
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
                      <AvatarImage src={user?.avatar} alt={user?.name} />
                      <AvatarFallback>{user?.name?.charAt(0)}</AvatarFallback>
                    </Avatar>
                    <div className="flex-1 min-w-0 flex-col items-start overflow-hidden group-data-[state=collapsed]:hidden flex">
                      <span className="w-full truncate font-medium">{user?.name}</span>
                      <span className="w-full truncate text-xs text-muted-foreground">{user?.email}</span>
                    </div>
                  </Button>
                </DropdownMenuTrigger>
              </TooltipTrigger>
              <TooltipContent side="right" align="center" className="hidden group-data-[state=collapsed]:block">
                <div className="text-left">
                  <p className="font-medium">{user?.name}</p>
                  <p className="text-xs text-muted-foreground">{user?.email}</p>
                </div>
              </TooltipContent>
            </Tooltip>
            <DropdownMenuContent className="w-56" side="right" align="end">
              <DropdownMenuLabel className="font-normal">
                <div className="flex flex-col space-y-1">
                  <p className="text-sm font-medium leading-none">{user?.name}</p>
                  <p className="text-xs leading-none text-muted-foreground">{user?.email}</p>
                </div>
              </DropdownMenuLabel>
              <DropdownMenuSeparator />
              <a href="/profile/settings">
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
          {/* Additional header content can go here */}
        </header>
        {children}
      </SidebarInset>
    </SidebarProvider>
  );
}
