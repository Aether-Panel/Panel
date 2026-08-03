'use client';
import { useState } from 'react';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { useAuth } from '@/contexts/providers';
import { useToast } from '@/hooks/use-toast';
import { Check, Copy, ExternalLink, Lock } from 'lucide-react';

type SFTPViewProps = {
    server: any;
};

function SpecRow({
    label,
    value,
    copyable,
    copied,
    onCopy,
}: {
    label: string;
    value: string;
    copyable?: boolean;
    copied?: boolean;
    onCopy?: () => void;
}) {
    return (
        <div className="flex items-center justify-between gap-4 px-4 py-3">
            <span className="shrink-0 text-sm text-muted-foreground">{label}</span>
            <span className="flex min-w-0 items-center gap-2">
                <code className="truncate font-mono text-sm text-foreground">{value}</code>
                {copyable && (
                    <Button variant="ghost" size="icon" className="h-6 w-6 shrink-0 text-muted-foreground hover:text-foreground" onClick={onCopy} aria-label={`Copiar ${label}`}>
                        {copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
                    </Button>
                )}
            </span>
        </div>
    );
}

export default function SFTPView({ server }: SFTPViewProps) {
    const { user } = useAuth();
    const { toast } = useToast();
    const [copiedField, setCopiedField] = useState<string | null>(null);

    if (!server || !server.node) {
        return (
            <div className="mt-6">
                <Card className="border-0 bg-card/50 backdrop-blur-sm">
                    <CardContent className="py-12 text-center text-muted-foreground">
                        No se pudo cargar la información del nodo para SFTP.
                    </CardContent>
                </Card>
            </div>
        );
    }

    const sftpHost = server.node.publicHost || window.location.hostname;
    const sftpPort = server.node.sftpPort || 5657;
    const sftpUser = `${user?.email || user?.username || 'user'}#${server.id}`;
    const uri = `sftp://${sftpUser}@${sftpHost}:${sftpPort}/`;
    const command = `sftp -P ${sftpPort} ${sftpUser}@${sftpHost}`;

    const copyToClipboard = (text: string, field: string) => {
        navigator.clipboard.writeText(text);
        setCopiedField(field);
        setTimeout(() => setCopiedField(null), 2000);
        toast({
            description: "Copiado al portapapeles",
        });
    };

    const handleLaunchSFTP = () => {
        const url = `sftp://${encodeURIComponent(sftpUser)}@${sftpHost}:${sftpPort}/`;
        window.location.href = url;
        toast({
            title: "Lanzando SFTP",
            description: "Se ha enviado la solicitud a tu cliente SFTP local.",
        });
    };

    return (
        <div className="mt-6 flex flex-col gap-6 animate-in fade-in slide-in-from-bottom-4 duration-500">
            <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
                <div className="space-y-1">
                    <h2 className="font-headline text-2xl font-bold tracking-tight">Acceso SFTP</h2>
                    <p className="max-w-xl text-sm text-muted-foreground">
                        Gestiona los archivos de tu servidor desde un cliente externo como FileZilla o WinSCP.
                    </p>
                </div>
                <Button onClick={handleLaunchSFTP} className="shrink-0">
                    <ExternalLink className="mr-2 h-4 w-4" />
                    Conectar con cliente local
                </Button>
            </div>

            <div className="overflow-hidden rounded-xl border border-border/80 bg-[#070A12] shadow-[0_24px_70px_rgb(0_0_0/0.35)]">
                <div className="relative flex items-center justify-between border-b border-border/70 px-4 py-2.5">
                    <div className="flex items-center gap-1.5">
                        <span className="h-3 w-3 rounded-full bg-[#FF5F57]" />
                        <span className="h-3 w-3 rounded-full bg-[#FEBC2E]" />
                        <span className="h-3 w-3 rounded-full bg-[#28C840]" />
                    </div>
                    <span className="absolute left-1/2 -translate-x-1/2 font-mono text-[11px] text-muted-foreground">acceso-sftp comandos</span>
                </div>
                <div className="space-y-3 px-5 py-5">
                    <div className="flex items-start gap-3">
                        <span className="select-none font-mono text-sm font-bold text-primary">$</span>
                        <code className="min-w-0 flex-1 break-all font-mono text-sm leading-relaxed text-foreground">
                            <span className="text-primary">sftp://</span>
                            {sftpUser}@{sftpHost}:<span className="text-muted-foreground">{sftpPort}</span>/
                        </code>
                        <Button variant="ghost" size="icon" className="h-6 w-6 shrink-0 text-muted-foreground hover:text-foreground" onClick={() => copyToClipboard(uri, 'uri')} aria-label="Copiar URI SFTP">
                            {copiedField === 'uri' ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
                        </Button>
                    </div>
                    <div className="flex items-start gap-3">
                        <span className="select-none font-mono text-sm font-bold text-primary">$</span>
                        <code className="min-w-0 flex-1 break-all font-mono text-sm leading-relaxed text-foreground">{command}</code>
                        <Button variant="ghost" size="icon" className="h-6 w-6 shrink-0 text-muted-foreground hover:text-foreground" onClick={() => copyToClipboard(command, 'cmd')} aria-label="Copiar comando">
                            {copiedField === 'cmd' ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
                        </Button>
                    </div>
                </div>
            </div>

            <div className="grid gap-6 md:grid-cols-2">
                <Card className="overflow-hidden border border-border/80 bg-card">
                    <div className="flex items-center gap-2 border-b border-border/60 px-4 py-3">
                        <span className="h-1.5 w-1.5 rounded-full bg-primary" />
                        <span className="text-xs font-semibold uppercase tracking-[0.14em] text-muted-foreground">Conexión</span>
                    </div>
                    <CardContent className="divide-y divide-border/60 p-0">
                        <SpecRow
                            label="Host / IP"
                            value={sftpHost}
                            copyable
                            copied={copiedField === 'host'}
                            onCopy={() => copyToClipboard(sftpHost, 'host')}
                        />
                        <SpecRow
                            label="Puerto"
                            value={sftpPort.toString()}
                            copyable
                            copied={copiedField === 'port'}
                            onCopy={() => copyToClipboard(sftpPort.toString(), 'port')}
                        />
                    </CardContent>
                </Card>

                <Card className="overflow-hidden border border-border/80 bg-card">
                    <div className="flex items-center gap-2 border-b border-border/60 px-4 py-3">
                        <span className="h-1.5 w-1.5 rounded-full bg-primary" />
                        <span className="text-xs font-semibold uppercase tracking-[0.14em] text-muted-foreground">Credenciales</span>
                    </div>
                    <CardContent className="divide-y divide-border/60 p-0">
                        <SpecRow
                            label="Usuario"
                            value={sftpUser}
                            copyable
                            copied={copiedField === 'user'}
                            onCopy={() => copyToClipboard(sftpUser, 'user')}
                        />
                        <div className="flex items-center justify-between gap-4 px-4 py-3">
                            <span className="shrink-0 text-sm text-muted-foreground">Contraseña</span>
                            <span className="flex min-w-0 items-center gap-2 text-sm text-muted-foreground">
                                <Lock className="h-3.5 w-3.5 shrink-0" />
                                <span className="truncate">La misma contraseña de tu cuenta</span>
                            </span>
                        </div>
                    </CardContent>
                </Card>
            </div>
        </div>
    );
}
