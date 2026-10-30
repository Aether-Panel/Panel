'use client';

import { HardDrive } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { formatBytes } from '@/lib/utils';

type NodeWithSystem = {
  id: number | string;
  name?: string;
  systemInfo?: { serversDisk?: { low?: boolean; free?: number; total?: number } } | null;
};

/**
 * Warns the administrators about every node whose servers' disk is nearly full (the node sets
 * `serversDisk.low` under 10% or 5 GB free). A full disk stops every server on the node and
 * breaks installs halfway, so it shows before that happens. Renders nothing when all is fine.
 */
export function NodeDiskAlert({ nodes, linkToNode = true }: Readonly<{ nodes: NodeWithSystem[]; linkToNode?: boolean }>) {
  const { t } = useTranslations();
  const low = nodes.filter((node) => node.systemInfo?.serversDisk?.low);
  if (low.length === 0) return null;

  return (
    <output className="grid gap-2">
      {low.map((node) => {
        const disk = node.systemInfo?.serversDisk ?? {};
        const name = node.name || String(node.id);
        return (
          <div key={node.id} className="flex items-start gap-3 rounded-xl border border-warning/30 bg-warning/[0.06] px-4 py-3 text-sm">
            <HardDrive className="mt-0.5 h-4 w-4 shrink-0 text-warning" aria-hidden="true" />
            <div className="min-w-0 space-y-0.5">
              <p className="font-medium text-foreground">
                {linkToNode ? (
                  <a href={`/nodes/view/?id=${node.id}`} className="rounded-sm underline decoration-border underline-offset-2 hover:decoration-foreground focus-ring">
                    {t('nodes.diskLow.title', { name })}
                  </a>
                ) : (
                  t('nodes.diskLow.title', { name })
                )}
              </p>
              <p className="text-pretty text-muted-foreground">
                {t('nodes.diskLow.description', { free: formatBytes(disk.free ?? 0), total: formatBytes(disk.total ?? 0) })}
              </p>
            </div>
          </div>
        );
      })}
    </output>
  );
}
