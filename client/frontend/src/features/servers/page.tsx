'use client';
import { useAuth } from '@/contexts/providers';
import { useEffect, useState } from 'react';
import { PageHeader } from '@/components/page-header';
import { Button } from '@/components/ui/button';
import { PlusCircle, Server as ServerIcon } from 'lucide-react';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';

import { useTranslations } from '@/contexts/translations-context';
import { useServers } from '@/hooks/use-servers';
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
    <div className="flex flex-col gap-6">
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

      {servers.length === 0 ? (
        <div className="rounded-xl border border-dashed border-border/60 bg-card/50 p-12 text-center">
          <ServerIcon className="mx-auto h-10 w-10 text-muted-foreground/40 mb-4" />
          <p className="text-sm font-medium text-foreground mb-1">{t('servers.empty.title')}</p>
          <p className="text-xs text-muted-foreground mb-4">{t('servers.empty.description')}</p>
          {hasScope('server.create') && (
            <Button onClick={() => setIsCreateOpen(true)}>
              <PlusCircle className="mr-2 h-4 w-4" />
              {t('servers.addServer')}
            </Button>
          )}
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {servers.map((server) => (
            <ServerCard key={server.id} server={server} t={t} />
          ))}
        </div>
      )}
    </div>
  );
}
