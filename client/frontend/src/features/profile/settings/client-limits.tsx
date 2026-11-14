import { useEffect, useState } from 'react';
import { Checkbox } from '@/components/ui/checkbox';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { fetchAllPages } from '@/lib/paged';

/** What a new client is limited to: one server (or all) and some permissions (or all). */
export type ClientLimits = { serverId: string; scopes: string[] | null };

export const NO_LIMITS: ClientLimits = { serverId: '', scopes: null };

/** Permission groups a client can be given on one server, by what an integration needs. */
const GROUPS: { key: string; scopes: string[] }[] = [
  { key: 'power', scopes: ['server.status', 'server.start', 'server.stop', 'server.kill'] },
  { key: 'stats', scopes: ['server.stats'] },
  { key: 'console', scopes: ['server.console', 'server.console.send'] },
  { key: 'files', scopes: ['server.files.view', 'server.files.edit'] },
  { key: 'data', scopes: ['server.data.view'] },
  { key: 'backups', scopes: ['server.backup.view', 'server.backup.create'] },
];

const ALL_SERVERS = '__all__';

/**
 * Lets a person limit a new OAuth2 client to one of their servers and to some permissions, so
 * an outside service (a game tool, a bot) gets a key for that server only, not for the account.
 */
export function ClientLimitsFields({ value, onChange, t }: Readonly<{
  value: ClientLimits;
  onChange: (next: ClientLimits) => void;
  t: (key: string, options?: Record<string, string | number>) => string;
}>) {
  const [servers, setServers] = useState<{ id: string; name: string }[] | null>(null);

  useEffect(() => {
    let alive = true;
    fetchAllPages('/api/servers', 'servers')
      .then(({ items }) => {
        if (alive) setServers(items.map((s: any) => ({ id: s.id, name: s.name || s.id })));
      })
      .catch(() => alive && setServers([]));
    return () => {
      alive = false;
    };
  }, []);

  const toggle = (group: string[], on: boolean) => {
    const current = new Set(value.scopes ?? []);
    for (const scope of group) {
      if (on) current.add(scope);
      else current.delete(scope);
    }
    onChange({ ...value, scopes: [...current] });
  };

  return (
    <div className="grid gap-3">
      <div className="grid gap-1.5">
        <Label htmlFor="client-server">{t('profileSettings.oauth.limits.server')}</Label>
        <Select
          value={value.serverId || ALL_SERVERS}
          onValueChange={(id) => onChange(id === ALL_SERVERS ? NO_LIMITS : { serverId: id, scopes: value.scopes })}
        >
          <SelectTrigger id="client-server">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL_SERVERS}>{t('profileSettings.oauth.limits.allServers')}</SelectItem>
            {(servers ?? []).map((server) => (
              <SelectItem key={server.id} value={server.id}>{server.name}</SelectItem>
            ))}
          </SelectContent>
        </Select>
        <p className="text-pretty text-xs text-muted-foreground">
          {value.serverId ? t('profileSettings.oauth.limits.oneServerHint') : t('profileSettings.oauth.limits.allServersHint')}
        </p>
      </div>

      {value.serverId && (
        <fieldset className="grid gap-2 rounded-lg border border-border/60 bg-background/30 p-3">
          <legend className="px-1 text-sm font-medium">{t('profileSettings.oauth.limits.permissions')}</legend>
          <label className="flex items-center gap-2 text-sm">
            <Checkbox checked={value.scopes === null} onCheckedChange={(on) => onChange({ ...value, scopes: on ? null : [] })} />
            {t('profileSettings.oauth.limits.everything')}
          </label>
          {value.scopes !== null && (
            <div className="grid gap-2 pl-1 sm:grid-cols-2">
              {GROUPS.map((group) => (
                <label key={group.key} className="flex items-center gap-2 text-sm">
                  <Checkbox
                    checked={group.scopes.every((s) => value.scopes?.includes(s))}
                    onCheckedChange={(on) => toggle(group.scopes, on === true)}
                  />
                  {t(`profileSettings.oauth.limits.groups.${group.key}`)}
                </label>
              ))}
            </div>
          )}
        </fieldset>
      )}
    </div>
  );
}
