'use client';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Shield, Globe, Lock, ExternalLink, Copy, Check, Terminal } from 'lucide-react';
import { Label } from '@/components/ui/label';
import { useState } from 'react';
import { useAuth } from '@/contexts/providers';
import { useToast } from '@/hooks/use-toast';
import { useTranslations } from '@/contexts/translations-context';

type SFTPViewProps = {
    server: any;
};

export default function SFTPView({ server }: SFTPViewProps) {
    const { user } = useAuth();
    const { toast } = useToast();
    const { t } = useTranslations();
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

    const copyToClipboard = (text: string, field: string) => {
        navigator.clipboard.writeText(text);
        setCopiedField(field);
        setTimeout(() => setCopiedField(null), 2000);
        toast({
            description: "Copiado al portapapeles",
        });
    };

    const handleLaunchSFTP = () => {
        // Some clients might handle the # differently, but standard is sftp://user#pass@host:port/
        // However, PufferPanel uses # as a separator in the username field.
        const url = `sftp://${encodeURIComponent(sftpUser)}@${sftpHost}:${sftpPort}/`;
        window.location.href = url;
        toast({
            title: "Lanzando SFTP",
            description: "Se ha enviado la solicitud a tu cliente SFTP local.",
        });
    };

    return (
        <div className="mt-6 flex flex-col gap-6">
            <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
                <div>
                    <h2 className="text-2xl font-bold tracking-tight">Acceso SFTP</h2>
                    <p className="text-muted-foreground">Usa estas credenciales para gestionar tus archivos mediante un cliente externo (como FileZilla o WinSCP).</p>
                </div>
                <Button onClick={handleLaunchSFTP} className="shrink-0">
                    <ExternalLink className="mr-2 h-4 w-4" />
                    Conectar con cliente local
                </Button>
            </div>

            <div className="grid gap-6 md:grid-cols-2">
                <Card className="border-0 bg-card/50 backdrop-blur-sm shadow-xl border-l-4 border-l-primary">
                    <CardHeader>
                        <CardTitle className="text-lg flex items-center gap-2">
                            <Globe className="h-5 w-5 text-primary" />
                            Conexión
                        </CardTitle>
                        <CardDescription>Detalles del servidor remoto</CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                        <div className="space-y-1.5 p-3 rounded-md bg-background/50 border group relative">
                            <div className="flex items-center justify-between">
                                <Label className="text-xs text-muted-foreground uppercase">Host / IP</Label>
                                <Button variant="ghost" size="icon" className="h-5 w-5 opacity-0 group-hover:opacity-100 transition-opacity" onClick={() => copyToClipboard(sftpHost, 'host')}>
                                    {copiedField === 'host' ? <Check className="h-2.5 w-2.5" /> : <Copy className="h-2.5 w-2.5" />}
                                </Button>
                            </div>
                            <code className="text-sm font-mono block">{sftpHost}</code>
                        </div>

                        <div className="space-y-1.5 p-3 rounded-md bg-background/50 border group relative">
                            <div className="flex items-center justify-between">
                                <Label className="text-xs text-muted-foreground uppercase">Puerto</Label>
                                <Button variant="ghost" size="icon" className="h-5 w-5 opacity-0 group-hover:opacity-100 transition-opacity" onClick={() => copyToClipboard(sftpPort.toString(), 'port')}>
                                    {copiedField === 'port' ? <Check className="h-2.5 w-2.5" /> : <Copy className="h-2.5 w-2.5" />}
                                </Button>
                            </div>
                            <code className="text-sm font-mono block">{sftpPort}</code>
                        </div>
                    </CardContent>
                </Card>

                <Card className="border-0 bg-card/50 backdrop-blur-sm shadow-xl border-l-4 border-l-accent">
                    <CardHeader>
                        <CardTitle className="text-lg flex items-center gap-2">
                            <Shield className="h-5 w-5 text-accent" />
                            Credenciales
                        </CardTitle>
                        <CardDescription>Usa tu cuenta del panel</CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                        <div className="space-y-1.5 p-3 rounded-md bg-background/50 border group relative">
                            <div className="flex items-center justify-between">
                                <Label className="text-xs text-muted-foreground uppercase">Usuario</Label>
                                <Button variant="ghost" size="icon" className="h-5 w-5 opacity-0 group-hover:opacity-100 transition-opacity" onClick={() => copyToClipboard(sftpUser, 'user')}>
                                    {copiedField === 'user' ? <Check className="h-2.5 w-2.5" /> : <Copy className="h-2.5 w-2.5" />}
                                </Button>
                            </div>
                            <code className="text-sm font-mono block">{sftpUser}</code>
                        </div>

                        <div className="space-y-1.5 p-3 rounded-md bg-background/50 border group relative">
                            <div className="flex items-center justify-between">
                                <Label className="text-xs text-muted-foreground uppercase">Contraseña</Label>
                            </div>
                            <div className="flex items-center gap-2">
                                <Lock className="h-4 w-4 text-muted-foreground" />
                                <span className="text-sm">La misma contraseña de tu cuenta</span>
                            </div>
                        </div>
                    </CardContent>
                </Card>
            </div>

            <Card className="border-0 bg-primary/5 backdrop-blur-sm border-dashed border-2 border-primary/20">
                <CardContent className="pt-6 flex gap-4 items-start">
                    <Terminal className="h-5 w-5 text-primary shrink-0 mt-0.5" />
                    <div className="space-y-1">
                        <h4 className="text-sm font-semibold">Comando para terminal:</h4>
                        <div className="flex items-center gap-2 bg-background/80 p-2 rounded border font-mono text-xs overflow-x-auto">
                            <span>sftp -P {sftpPort} {sftpUser}@{sftpHost}</span>
                            <Button variant="ghost" size="icon" className="h-6 w-6 ml-auto" onClick={() => copyToClipboard(`sftp -P ${sftpPort} ${sftpUser}@${sftpHost}`, 'cmd')}>
                                {copiedField === 'cmd' ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}
                            </Button>
                        </div>
                    </div>
                </CardContent>
            </Card>
        </div>
    );
}
