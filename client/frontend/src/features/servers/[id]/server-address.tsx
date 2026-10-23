'use client';
import { useEffect, useState } from 'react';
import { Copy } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { sileo } from "@/lib/toast";
import { api } from '@/lib/api-client';
import { useTranslations } from '@/contexts/translations-context';

export type SubdomainRecord = { type: string; name: string; content: string };

/** A server's subdomain as GET /api/servers/:id/subdomain returns it. */
export type SubdomainInfo = {
  enabled: boolean;
  domain: string;
  subdomain: string;
  address: string;
  srv: boolean;
  port: number;
  records: SubdomainRecord[];
};

/** Sent on window when the subdomain card changes the address, so the header follows. */
export const SUBDOMAIN_EVENT = 'aether:subdomain-changed';
export type SubdomainEventDetail = { serverId: string; info: SubdomainInfo | null };

const pending = new Map<string, Promise<SubdomainInfo>>();

/** Loads a server's subdomain; the header chip and the settings card share one request. */
export function fetchSubdomain(serverId: string): Promise<SubdomainInfo> {
  const inFlight = pending.get(serverId);
  if (inFlight) return inFlight;
  const request = api.get<SubdomainInfo>(`/api/servers/${serverId}/subdomain`).finally(() => pending.delete(serverId));
  pending.set(serverId, request);
  return request;
}

export function announceSubdomain(serverId: string, info: SubdomainInfo | null) {
  window.dispatchEvent(new CustomEvent<SubdomainEventDetail>(SUBDOMAIN_EVENT, { detail: { serverId, info } }));
}

/** The address players type: Minecraft Java finds the port through the SRV record. */
export function playerAddress(info: SubdomainInfo) {
  return info.srv || !info.port ? info.address : `${info.address}:${info.port}`;
}

/** The server page's own id, from /servers/view?id=… */
function serverIdFromUrl() {
  if (typeof window === 'undefined') return '';
  return new URLSearchParams(window.location.search).get('id') ?? '';
}

export function ServerAddress({ ip, port, serverId }: { ip: string; port: number; serverId?: string }) {
  const { t } = useTranslations();
  const id = serverId || serverIdFromUrl();
  const [subdomain, setSubdomain] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    let alive = true;
    fetchSubdomain(id)
      .then((info) => {
        if (alive) setSubdomain(info.enabled && info.address ? playerAddress(info) : null);
      })
      // no permission or an older panel: the IP stays
      .catch(() => undefined);
    const onChange = (event: Event) => {
      const { detail } = event as CustomEvent<SubdomainEventDetail>;
      if (detail.serverId !== id) return;
      setSubdomain(detail.info && detail.info.address ? playerAddress(detail.info) : null);
    };
    window.addEventListener(SUBDOMAIN_EVENT, onChange);
    return () => {
      alive = false;
      window.removeEventListener(SUBDOMAIN_EVENT, onChange);
    };
  }, [id]);

  const ipAddress = `${ip}:${port}`;
  const address = subdomain ?? ipAddress;

  const handleCopy = () => {
    void navigator.clipboard.writeText(address);
    sileo.success({
      title: t('servers.address.copyToastTitle'),
      description: t('servers.address.copyToastDescription', { address }),
    });
  };

  return (
    <div className="flex min-w-0 max-w-full items-center gap-1.5 rounded-lg border border-border/60 bg-muted/50 py-1 pl-3 pr-1 text-sm text-muted-foreground">
      <span
        key={address}
        className="min-w-0 truncate font-mono tabular-nums animate-in fade-in-0 duration-300"
        title={subdomain ? ipAddress : undefined}
      >
        {address}
      </span>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        onClick={handleCopy}
        aria-label={t('servers.address.copyButtonTitle')}
        title={t('servers.address.copyButtonTitle')}
        className="h-6 w-6 shrink-0 text-muted-foreground hover:text-foreground"
      >
        <Copy aria-hidden="true" />
      </Button>
    </div>
  );
}
