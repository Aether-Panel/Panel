'use client';
import { useAuth } from '@/contexts/providers';
import { useEffect, useState } from 'react';
import type { Server } from '@/lib/data';
import { PageHeader } from '@/components/page-header';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { MoreHorizontal, PlusCircle, Server as ServerIcon, Globe, Activity } from 'lucide-react';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuTrigger, DropdownMenuSeparator } from '@/components/ui/dropdown-menu';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';

import { useTranslations } from '@/contexts/translations-context';
import { useServers } from '@/hooks/use-servers';
import { cn, formatBytes } from '@/lib/utils';
import { CreateServerStepper } from './create-server-stepper';

import { ServerCard } from '@/components/server-card';

export default function ServersPage() {
  const { role, hasScope } = useAuth();
  const { servers, loading: serversLoading, refresh } = useServers();
  const [isMounted, setIsMounted] = useState(false);
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const { t } = useTranslations();

  useEffect(() => {
    setIsMounted(true);

    // Check if we should automatically open the create dialog
    const searchParams = new URLSearchParams(window.location.search);
    if (searchParams.get('create') === 'true') {
      setIsCreateOpen(true);
    }
  }, []);

  useEffect(() => {
    if (isMounted && role && !hasScope('server.view')) {
      window.location.href = '/dashboard/';
    }
  }, [role, isMounted, hasScope]);

  if (!isMounted || !hasScope('server.view') || serversLoading) {
    return (
      <div className="flex h-full items-center justify-center min-h-[400px]">
        <div className="h-8 w-8 animate-spin rounded-full border-[3px] border-border border-t-primary border-l-accent" />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-8">
      <PageHeader title={t('servers.title')} description={t('servers.description')}>
        {hasScope('server.create') && (
          <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
            <DialogTrigger asChild>
              <Button>
                <PlusCircle className="mr-2" />
                {t('servers.addServer')}
              </Button>
            </DialogTrigger>
            <DialogContent className="sm:max-w-5xl max-h-[88vh] flex flex-col">
              <DialogHeader>
                <DialogTitle>{t('servers.addDialog.title')}</DialogTitle>
                <DialogDescription>{t('servers.addDialog.description')}</DialogDescription>
              </DialogHeader>
              <div className="flex-1 overflow-y-auto py-4 pr-1">
                <CreateServerStepper onComplete={() => {
                  setIsCreateOpen(false);
                  refresh();
                }} />
              </div>
            </DialogContent>
          </Dialog>
        )}
      </PageHeader>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2 2xl:grid-cols-3">
        {servers.map((server) => (
          <ServerCard key={server.id} server={server} t={t} />
        ))}
      </div>
    </div>
  );
}
