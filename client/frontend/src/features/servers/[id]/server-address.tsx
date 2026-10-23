'use client';
import { Copy } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { sileo } from "@/lib/toast";
import { useTranslations } from '@/contexts/translations-context';

export function ServerAddress({ ip, port }: { ip: string; port: number }) {
  
  const { t } = useTranslations();
  const address = `${ip}:${port}`;

  const handleCopy = () => {
    void navigator.clipboard.writeText(address);
    sileo.success({
      title: t('servers.address.copyToastTitle'),
      description: t('servers.address.copyToastDescription', { address }),
    });
  };

  return (
    <div className="flex items-center gap-1.5 rounded-lg border border-border/60 bg-muted/50 py-1 pl-3 pr-1 text-sm text-muted-foreground">
      <span className="font-mono tabular-nums">{address}</span>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        onClick={handleCopy}
        aria-label={t('servers.address.copyButtonTitle')}
        title={t('servers.address.copyButtonTitle')}
        className="h-6 w-6 text-muted-foreground hover:text-foreground"
      >
        <Copy aria-hidden="true" />
      </Button>
    </div>
  );
}
