'use client';
import { useState } from 'react';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { useAuth } from '@/contexts/providers';
import { useTranslations } from '@/contexts/translations-context';
import { sileo } from "@/lib/toast";
import { Check, Copy, ExternalLink, Lock, ServerOff } from 'lucide-react';
import { EmptyState } from '@/components/empty-state';

type SFTPViewProps = {
    server: any;
};

function CopyButton({ label, copied, onCopy }: { label: string; copied: boolean; onCopy: () => void }) {
    return (
        <Button
            variant="ghost"
            size="icon"
            className="h-9 w-9 shrink-0 text-muted-foreground hover:text-foreground sm:h-7 sm:w-7"
            onClick={onCopy}
            aria-label={label}
            title={label}
        >
            {copied ? <Check className="text-success" aria-hidden="true" /> : <Copy aria-hidden="true" />}
        </Button>
    );
}

function SpecRow({
    label,
    value,
    copyLabel,
    copied,
    onCopy,
}: {
    label: string;
    value: string;
    copyLabel: string;
    copied: boolean;
    onCopy: () => void;
}) {
    return (
        <div className="flex items-center gap-3 px-4 py-2.5">
            <div className="flex min-w-0 flex-1 flex-col gap-0.5 sm:flex-row sm:items-center sm:justify-between sm:gap-4">
                <span className="shrink-0 text-sm text-muted-foreground">{label}</span>
                <code className="min-w-0 truncate font-mono text-sm text-foreground">{value}</code>
            </div>
            <CopyButton label={copyLabel} copied={copied} onCopy={onCopy} />
        </div>
    );
}

export default function SFTPView({ server }: SFTPViewProps) {
    const { user } = useAuth();
    const { t } = useTranslations();

    const [copiedField, setCopiedField] = useState<string | null>(null);

    if (!server || !server.node) {
        return (
            <Card className="mt-6 p-5 md:p-6">
                <EmptyState
                    icon={ServerOff}
                    title={t('servers.sftp.unavailableTitle')}
                    description={t('servers.sftp.unavailableDescription')}
                />
            </Card>
        );
    }

    const sftpHost = server.node.publicHost || window.location.hostname;
    const sftpPort = server.node.sftpPort || 5657;
    const sftpUser = `${user?.email || user?.username || 'user'}#${server.id}`;
    const uri = `sftp://${sftpUser}@${sftpHost}:${sftpPort}/`;
    const command = `sftp -P ${sftpPort} ${sftpUser}@${sftpHost}`;

    const copyToClipboard = (text: string, field: string) => {
        void navigator.clipboard.writeText(text);
        setCopiedField(field);
        setTimeout(() => setCopiedField(null), 2000);
        sileo.success({ title: t('common.copied') });
    };

    const handleLaunchSFTP = () => {
        const url = `sftp://${encodeURIComponent(sftpUser)}@${sftpHost}:${sftpPort}/`;
        window.location.href = url;
        sileo.success({
            title: t('servers.sftp.launchTitle'),
            description: t('servers.sftp.launchDescription'),
        });
    };

    return (
        <Card className="mt-6">
            <div className="flex flex-col gap-4 border-b border-border/50 p-5 sm:flex-row sm:items-center sm:justify-between md:p-6">
                <div className="min-w-0 space-y-1">
                    <h2 className="text-lg font-semibold tracking-[-0.02em]">{t('servers.sftp.title')}</h2>
                    <p className="text-sm text-muted-foreground">{t('servers.sftp.description')}</p>
                </div>
                <Button onClick={handleLaunchSFTP} className="shrink-0 self-start sm:self-auto">
                    <ExternalLink aria-hidden="true" />
                    {t('servers.sftp.launch')}
                </Button>
            </div>

            <div className="space-y-5 p-5 md:p-6">
                <div className="divide-y divide-border/50 rounded-xl border border-border/60 bg-background/30">
                    <div className="flex items-start gap-3 px-4 py-3">
                        <div className="min-w-0 flex-1 space-y-1">
                            <p className="text-xs text-muted-foreground">{t('servers.sftp.uriLabel')}</p>
                            <code className="block break-all font-mono text-sm leading-relaxed text-foreground">{uri}</code>
                        </div>
                        <CopyButton label={t('servers.sftp.copyUri')} copied={copiedField === 'uri'} onCopy={() => copyToClipboard(uri, 'uri')} />
                    </div>
                    <div className="flex items-start gap-3 px-4 py-3">
                        <div className="min-w-0 flex-1 space-y-1">
                            <p className="text-xs text-muted-foreground">{t('servers.sftp.commandLabel')}</p>
                            <code className="block break-all font-mono text-sm leading-relaxed text-foreground">{command}</code>
                        </div>
                        <CopyButton label={t('servers.sftp.copyCommand')} copied={copiedField === 'cmd'} onCopy={() => copyToClipboard(command, 'cmd')} />
                    </div>
                </div>

                <div className="grid grid-cols-1 gap-5 xl:grid-cols-2">
                    <section className="min-w-0 space-y-2">
                        <h3 className="text-sm font-semibold">{t('servers.sftp.connection')}</h3>
                        <div className="divide-y divide-border/50 rounded-xl border border-border/60 bg-background/30">
                            <SpecRow
                                label={t('servers.sftp.host')}
                                value={sftpHost}
                                copyLabel={t('servers.sftp.copyHost')}
                                copied={copiedField === 'host'}
                                onCopy={() => copyToClipboard(sftpHost, 'host')}
                            />
                            <SpecRow
                                label={t('servers.sftp.port')}
                                value={sftpPort.toString()}
                                copyLabel={t('servers.sftp.copyPort')}
                                copied={copiedField === 'port'}
                                onCopy={() => copyToClipboard(sftpPort.toString(), 'port')}
                            />
                        </div>
                    </section>

                    <section className="min-w-0 space-y-2">
                        <h3 className="text-sm font-semibold">{t('servers.sftp.credentials')}</h3>
                        <div className="divide-y divide-border/50 rounded-xl border border-border/60 bg-background/30">
                            <SpecRow
                                label={t('servers.sftp.username')}
                                value={sftpUser}
                                copyLabel={t('servers.sftp.copyUsername')}
                                copied={copiedField === 'user'}
                                onCopy={() => copyToClipboard(sftpUser, 'user')}
                            />
                            <div className="flex min-h-[3rem] flex-col justify-center gap-0.5 px-4 py-2.5 sm:flex-row sm:items-center sm:justify-between sm:gap-4">
                                <span className="shrink-0 text-sm text-muted-foreground">{t('servers.sftp.password')}</span>
                                <span className="flex min-w-0 items-center gap-2 text-sm text-foreground/80">
                                    <Lock className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
                                    <span className="truncate">{t('servers.sftp.passwordHint')}</span>
                                </span>
                            </div>
                        </div>
                    </section>
                </div>
            </div>
        </Card>
    );
}
