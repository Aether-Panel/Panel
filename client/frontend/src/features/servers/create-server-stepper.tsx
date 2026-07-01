'use client';

import React, { useState, useEffect } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Card, CardContent, CardFooter } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Loader2, ChevronRight, ChevronLeft, Check, Code, Globe, Shield, X, ShieldAlert } from 'lucide-react';
import { useNodes } from '@/hooks/use-dashboard-data';
import { useUsers } from '@/hooks/use-users';
import { useTemplates } from '@/hooks/use-templates';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { useToast } from '@/hooks/use-toast';

class ErrorBoundary extends React.Component<
    { children: React.ReactNode },
    { hasError: boolean; errorMsg: string; errorStack: string }
> {
    constructor(props: any) {
        super(props);
        this.state = { hasError: false, errorMsg: '', errorStack: '' };
    }
    static getDerivedStateFromError(error: any) {
        return { hasError: true, errorMsg: String(error?.message || error || 'Unknown error'), errorStack: String(error?.stack || '') };
    }
    componentDidCatch(error: any, errorInfo: any) {
        console.error("[Stepper Error Boundary] Caught:", error);
        console.error("[Stepper Error Boundary] Component stack:", errorInfo?.componentStack);
    }
    render() {
        if (this.state.hasError) {
            return (
                <div className="p-6 border-2 border-dashed border-red-500/50 rounded-xl bg-red-50">
                    <ShieldAlert className="h-10 w-10 text-red-600 mx-auto mb-3" />
                    <h3 className="text-base font-bold text-red-700 text-center mb-2">Error en el renderizado</h3>
                    <pre className="text-xs bg-red-100 rounded p-3 overflow-auto max-h-40 text-red-800 whitespace-pre-wrap break-all">{this.state.errorMsg}{"\n\n"}{this.state.errorStack.slice(0, 500)}</pre>
                    <div className="flex gap-2 mt-4 justify-center">
                        <Button variant="outline" size="sm" onClick={() => this.setState({ hasError: false, errorMsg: '', errorStack: '' })}>Reintentar</Button>
                        <Button variant="outline" size="sm" onClick={() => window.location.reload()}>Recargar</Button>
                    </div>
                </div>
            );
        }
        return this.props.children;
    }
}

type Step = 1 | 2 | 3;

const SafeValue = ({ v, fallback = "" }: { v: any, fallback?: string }) => {
    if (v === null || v === undefined) return fallback;
    if (typeof v === 'object') return JSON.stringify(v);
    return String(v);
};

function Step1Environment({
    name, setName, selectedNode, setSelectedNode, selectedEnvironment, setSelectedEnvironment,
    selectedUsers, setSelectedUsers, forcedParentId, nodes, users
}: any) {
    return (
        <div className="grid gap-6 animate-in fade-in slide-in-from-right-4 duration-300 w-full">
            <div className="grid gap-2 w-full">
                <Label htmlFor="server-name" className="text-sm font-medium">Nombre del Servidor *</Label>
                <Input id="server-name" placeholder="Ej: Mi Servidor Minecraft" value={name} onChange={(e) => setName(e.target.value)} className="h-10" />
            </div>
            {forcedParentId ? (
                <div className="bg-accent/10 border border-accent/20 rounded-lg p-4 mb-4">
                    <div className="flex items-center gap-2 text-primary font-medium mb-1">
                        <Shield className="w-5 h-5" />
                        <span>Entorno Heredado</span>
                    </div>
                    <p className="text-sm text-muted-foreground">
                        El nodo, entorno y los usuarios de este subservidor se asignarán automáticamente desde el servidor padre.
                    </p>
                </div>
            ) : (
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div className="grid gap-2">
                        <Label className="text-sm font-medium">Nodo *</Label>
                        <Select value={selectedNode} onValueChange={setSelectedNode}>
                            <SelectTrigger className="h-10">
                                <SelectValue placeholder="Seleccionar nodo" />
                            </SelectTrigger>
                            <SelectContent className="max-h-80 overflow-y-auto z-[100]">
                                {nodes.map((n: any) => (
                                    <SelectItem key={n.id} value={String(n.id)}>{n.name} ({n.publicHost})</SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>
                    <div className="grid gap-2">
                        <Label className="text-sm font-medium">Entorno</Label>
                        <Select value={selectedEnvironment} onValueChange={setSelectedEnvironment}>
                            <SelectTrigger className="h-10">
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent className="max-h-80 overflow-y-auto z-[100]">
                                <SelectItem value="docker">Docker</SelectItem>
                                <SelectItem value="standard">Estándar (Hijo)</SelectItem>
                                <SelectItem value="tty">TTY</SelectItem>
                            </SelectContent>
                        </Select>
                    </div>
                </div>
            )}
            {!forcedParentId && (
                <div className="grid gap-2">
                    <Label className="text-sm font-medium">Usuarios con acceso</Label>
                    <div className="flex flex-col gap-3">
                        <Select
                            value=""
                            onValueChange={(val) => {
                                const id = Number(val);
                                if (!selectedUsers.includes(id)) {
                                    setSelectedUsers((prev: any) => [...prev, id]);
                                }
                            }}
                        >
                            <SelectTrigger className="h-10">
                                <SelectValue placeholder="Seleccionar usuario para añadir..." />
                            </SelectTrigger>
                            <SelectContent className="max-h-80 overflow-y-auto z-[100]">
                                {users.filter((u: any) => !selectedUsers.includes(u.id)).map((u: any) => (
                                    <SelectItem key={u.id} value={String(u.id)}>{u.username} ({u.email})</SelectItem>
                                ))}
                                {users.filter((u: any) => !selectedUsers.includes(u.id)).length === 0 && (
                                    <div className="p-2 text-xs text-muted-foreground text-center italic">No hay más usuarios disponibles.</div>
                                )}
                            </SelectContent>
                        </Select>

                        {selectedUsers.length > 0 && (
                            <div className="flex flex-wrap gap-2 pt-1">
                                {selectedUsers.map((id: number) => {
                                    const u = users.find((user: any) => user.id === id);
                                    return (
                                        <Badge key={id} variant="secondary" className="pl-3 pr-1 py-1 flex items-center gap-2 bg-primary/10 text-primary border-primary/20">
                                            {u?.username}
                                            <Button
                                                variant="ghost"
                                                size="icon"
                                                className="h-4 w-4 rounded-full hover:bg-red-500 hover:text-white transition-colors"
                                                onClick={() => setSelectedUsers((prev: any) => prev.filter((uid: number) => uid !== id))}
                                            >
                                                <X className="h-3 w-3" />
                                            </Button>
                                        </Badge>
                                    );
                                })}
                            </div>
                        )}
                    </div>
                    <p className="text-xs text-muted-foreground italic">Los usuarios seleccionados tendrán acceso de administrador al servidor.</p>
                </div>
            )}
        </div>
    );
}

function Step2Template({
    repos, selectedRepo, setSelectedRepo,
    templateList, selectedTemplateName, setSelectedTemplateName,
    loadingTemplates, templateDetails, templateError
}: any) {
    return (
        <div className="grid gap-6 animate-in fade-in slide-in-from-right-4 duration-300 w-full">
            <div className="grid gap-2 w-full">
                <Label>Repositorio de Plantillas</Label>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 w-full">
                    {repos.map((r: any) => (
                        <div
                            key={r.id}
                            onClick={() => setSelectedRepo(r.id)}
                            className={`flex items-center gap-3 p-4 rounded-xl border-2 cursor-pointer transition-all ${selectedRepo === r.id ? 'border-primary bg-primary/5 ring-4 ring-primary/10' : 'border-muted hover:border-primary/50'
                                }`}
                        >
                            {r.isLocal ? <Shield className="h-6 w-6 text-primary" /> : <Globe className="h-6 w-6 text-blue-400" />}
                            <div className="text-left">
                                <p className="font-semibold">{r.name}</p>
                                <p className="text-xs text-muted-foreground">{r.isLocal ? 'Local' : 'Comunidad'}</p>
                            </div>
                        </div>
                    ))}
                    {repos.length === 0 && (
                        <div className="col-span-2 flex items-center justify-center p-8 border-2 border-dashed rounded-xl text-muted-foreground text-sm">
                            <Loader2 className="h-5 w-5 animate-spin mr-2" />
                            Cargando repositorios...
                        </div>
                    )}
                </div>
            </div>

            {selectedRepo !== null && (
                <div className="grid gap-2 mt-4 w-full">
                    <Label>Seleccionar Plantilla</Label>
                    {loadingTemplates ? (
                        <div className="flex items-center justify-center p-8">
                            <Loader2 className="h-8 w-8 animate-spin text-primary" />
                        </div>
                    ) : templateList.length === 0 ? (
                        <div className="flex items-center justify-center p-8 border-2 border-dashed rounded-xl text-muted-foreground text-sm">
                            No hay plantillas disponibles en este repositorio.
                        </div>
                    ) : (
                        <div className="w-full min-w-0 max-h-[280px] overflow-y-auto pr-2">
                            <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-2 w-full">
                                {templateList.map((t: any) => (
                                    <div
                                        key={t.name}
                                        onClick={() => setSelectedTemplateName(t.name)}
                                        className={`p-2 rounded-lg border text-xs flex items-center gap-2 cursor-pointer transition-all min-w-0 ${selectedTemplateName === t.name ? 'bg-primary text-primary-foreground border-primary' : 'hover:bg-muted'
                                            }`}
                                    >
                                        <Code className="h-3 w-3 shrink-0" />
                                        <span className="truncate min-w-0">{t.name}</span>
                                    </div>
                                ))}
                            </div>
                        </div>
                    )}
                    {selectedTemplateName && !templateDetails && !templateError && (
                        <div className="flex items-center gap-2 text-xs text-muted-foreground mt-2 p-2 bg-primary/5 rounded-lg border border-primary/10">
                            <Loader2 className="h-3 w-3 animate-spin text-primary shrink-0" />
                            Cargando detalles de la plantilla <strong>{selectedTemplateName}</strong>...
                        </div>
                    )}
                    {selectedTemplateName && templateError && (
                        <div className="flex items-center gap-2 text-xs text-red-600 mt-2 p-2 bg-red-50 rounded-lg border border-red-200">
                            <ShieldAlert className="h-3 w-3 shrink-0" />
                            {templateError}
                        </div>
                    )}
                </div>
            )}
        </div>
    );
}

function Step3Configuration({
    templateDetails, templateError, configData, setConfigData,
    cpuLimit, setCpuLimit, memoryLimit, setMemoryLimit, diskLimit, setDiskLimit,
    setCurrentStep
}: any) {
    if (templateError) {
        return (
            <div className="flex flex-col items-center justify-center p-8 text-center bg-red-500/5 rounded-xl border border-red-500/20">
                <ShieldAlert className="h-10 w-10 text-red-500 mb-3" />
                <p className="font-bold text-red-700">Error de Plantilla</p>
                <p className="text-sm text-red-600 mt-1">{templateError}</p>
                <Button variant="outline" size="sm" className="mt-4" onClick={() => setCurrentStep(2)}>
                    <ChevronLeft className="mr-2 h-4 w-4" />
                    Volver
                </Button>
            </div>
        );
    }
    
    if (!templateDetails) {
        return (
            <div className="flex flex-col items-center justify-center p-20 text-center space-y-4">
                <div className="relative">
                    <div className="absolute -inset-4 bg-primary/10 rounded-full blur-xl animate-pulse" />
                    <Loader2 className="h-12 w-12 animate-spin text-primary relative" />
                </div>
                <div className="space-y-1">
                    <p className="font-bold text-lg text-foreground/80">Analizando Plantilla</p>
                    <p className="text-sm text-muted-foreground">Obteniendo parámetros de configuración...</p>
                </div>
            </div>
        );
    }

    const variables = (templateDetails.data || templateDetails.variables || templateDetails.Variables || {}) as Record<string, any>;
    const entries = Object.entries(variables).filter(([key, v]) => v && !v.internal && !['cpu', 'memory', 'disk'].includes(key));

    return (
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3 w-full">
            <div className="flex flex-col gap-1">
                <Label className="text-xs font-semibold">CPU <span className="font-normal text-muted-foreground">(%)</span></Label>
                <Input type="number" value={cpuLimit} onChange={(e) => setCpuLimit(e.target.value === '' ? '' : Number(e.target.value))} className="h-8 bg-background/50 text-sm" />
            </div>
            <div className="flex flex-col gap-1">
                <Label className="text-xs font-semibold">RAM <span className="font-normal text-muted-foreground">(MB)</span></Label>
                <Input type="number" value={memoryLimit} onChange={(e) => setMemoryLimit(e.target.value === '' ? '' : Number(e.target.value))} className="h-8 bg-background/50 text-sm" />
            </div>
            <div className="flex flex-col gap-1">
                <Label className="text-xs font-semibold">Disco <span className="font-normal text-muted-foreground">(MB)</span></Label>
                <Input type="number" value={diskLimit} onChange={(e) => setDiskLimit(e.target.value === '' ? '' : Number(e.target.value))} className="h-8 bg-background/50 text-sm" />
            </div>

            <div className="col-span-full border-t border-border/50 my-1" />

            {entries.map(([key, variable]: [string, any], idx: number) => {
                if (!variable || typeof variable !== 'object') return null;
                const currentVal = configData[key] !== undefined ? configData[key] : "";
                const stringifiedVal = (typeof currentVal === 'object' && currentVal !== null)
                    ? JSON.stringify(currentVal)
                    : String(currentVal ?? "");

                const label = variable.display || key;
                const isWide = (label.toLowerCase().includes('motd') || label.toLowerCase().includes('arguments') || label.toLowerCase().includes('command'));

                return (
                    <div key={key + idx} className={`flex flex-col gap-1 ${isWide ? 'col-span-2' : ''}`}>
                        <Label className="text-xs font-semibold truncate" title={label}>
                            <SafeValue v={label} />
                            {variable.required && <span className="text-red-500 ml-0.5">*</span>}
                        </Label>
                        {Array.isArray(variable.options) && variable.options.length > 0 ? (
                            <Select
                                value={stringifiedVal === "" ? "_EMPTY_VALUE_" : stringifiedVal}
                                onValueChange={(val) => setConfigData((prev: any) => ({ ...prev, [key]: val === "_EMPTY_VALUE_" ? "" : val }))}
                            >
                                <SelectTrigger className="h-8 bg-background/50 text-sm">
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent className="max-h-60 z-[100]">
                                    {variable.options.map((opt: any, oIdx: number) => {
                                        if (!opt) return null;
                                        const rawOptVal = opt.value !== undefined ? opt.value : (typeof opt === 'string' ? opt : "");
                                        const optVal = rawOptVal === "" ? "_EMPTY_VALUE_" : String(rawOptVal);
                                        return (
                                            <SelectItem key={oIdx} value={optVal}>
                                                <SafeValue v={opt.display || (rawOptVal === "" ? "Ninguno" : rawOptVal)} />
                                            </SelectItem>
                                        );
                                    })}
                                </SelectContent>
                            </Select>
                        ) : variable.type === 'boolean' ? (
                            <div
                                className="flex items-center gap-2 px-3 rounded-lg border bg-background/30 cursor-pointer h-8"
                                onClick={() => setConfigData((prev: any) => ({ ...prev, [key]: !currentVal }))}
                            >
                                <div className={`h-3.5 w-3.5 rounded border shrink-0 flex items-center justify-center ${currentVal ? 'bg-primary border-primary' : 'border-muted-foreground/40'}`}>
                                    {currentVal && <Check className="h-2.5 w-2.5 text-primary-foreground stroke-[3px]" />}
                                </div>
                                <span className="text-xs select-none">Habilitar</span>
                            </div>
                        ) : (
                            <Input
                                type={variable.type === 'integer' ? 'number' : 'text'}
                                value={stringifiedVal}
                                onChange={(e) => setConfigData((prev: any) => ({ ...prev, [key]: variable.type === 'integer' ? Number(e.target.value) : e.target.value }))}
                                className="h-8 bg-background/50 text-sm"
                            />
                        )}
                    </div>
                );
            })}
        </div>
    );
}

export function CreateServerStepper({ onComplete, forcedParentId, forcedNodeId }: { onComplete: () => void, forcedParentId?: string, forcedNodeId?: string }) {
    const { toast } = useToast();
    const isSplitter = !!forcedParentId;
    const { nodes } = useNodes(isSplitter);
    const { users } = useUsers(isSplitter);
    const { repos, getTemplatesForRepo, getTemplateDetails } = useTemplates();

    const [currentStep, setCurrentStep] = useState<Step>(1);
    const [loading, setLoading] = useState(false);

    const [name, setName] = useState('');
    const [selectedNode, setSelectedNode] = useState(forcedNodeId || '');
    const [selectedEnvironment, setSelectedEnvironment] = useState('docker');
    const [selectedUsers, setSelectedUsers] = useState<number[]>([]);

    useEffect(() => {
        if (forcedNodeId && forcedNodeId !== '' && selectedNode === '') {
            setSelectedNode(forcedNodeId);
        }
    }, [forcedNodeId]);

    const [selectedRepo, setSelectedRepo] = useState<number | null>(null);
    const [templateList, setTemplateList] = useState<any[]>([]);
    const [selectedTemplateName, setSelectedTemplateName] = useState('');
    const [loadingTemplates, setLoadingTemplates] = useState(false);

    const [templateDetails, setTemplateDetails] = useState<any>(null);
    const [templateError, setTemplateError] = useState<string | null>(null);
    const [configData, setConfigData] = useState<Record<string, any>>({});
    
    const [cpuLimit, setCpuLimit] = useState<number | ''>(100);
    const [memoryLimit, setMemoryLimit] = useState<number | ''>(1024);
    const [diskLimit, setDiskLimit] = useState<number | ''>(10240);

    useEffect(() => {
        if (repos.length === 1 && selectedRepo === null) {
            setSelectedRepo(repos[0].id);
        }
    }, [repos]);

    useEffect(() => {
        if (selectedRepo !== null) {
            setLoadingTemplates(true);
            setSelectedTemplateName('');
            setTemplateDetails(null);
            setTemplateError(null);
            getTemplatesForRepo(selectedRepo).then(list => {
                setTemplateList(list);
                setLoadingTemplates(false);
            }).catch(() => {
                setTemplateList([]);
                setLoadingTemplates(false);
            });
        }
    }, [selectedRepo]);

    const parseTemplateVariables = (details: any) => {
        const initial: Record<string, any> = {};
        const rawVars = details.data || details.variables || details.Variables || {};
        const variables = (typeof rawVars === 'object' && rawVars !== null) ? rawVars : {};

        Object.entries(variables).forEach(([key, val]: [string, any]) => {
            if (val && typeof val === 'object') {
                let rawValue = val.value !== undefined ? val.value : (val.default !== undefined ? val.default : "");

                if (val.type === 'boolean') {
                    rawValue = rawValue !== 'false' && rawValue !== false && !!rawValue;
                }
                if (val.type === 'integer') {
                    rawValue = Number(rawValue) || 0;
                }

                if (key === 'cpu') {
                    setCpuLimit(rawValue ? Number(rawValue) : 100);
                } else if (key === 'memory') {
                    setMemoryLimit(rawValue ? Number(rawValue) : 1024);
                } else if (key === 'disk') {
                    setDiskLimit(rawValue ? Number(rawValue) : 10240);
                } else {
                    initial[key] = (typeof rawValue === 'object' && rawValue !== null)
                        ? JSON.stringify(rawValue)
                        : (rawValue ?? "");
                }
            } else if (val !== undefined && val !== null) {
                if (key === 'cpu') setCpuLimit(Number(val));
                else if (key === 'memory') setMemoryLimit(Number(val));
                else if (key === 'disk') setDiskLimit(Number(val));
                else initial[key] = String(val);
            }
        });
        setConfigData(initial);
    };

    useEffect(() => {
        if (selectedTemplateName && selectedRepo !== null) {
            setTemplateError(null);
            setTemplateDetails(null);
            getTemplateDetails(selectedRepo, selectedTemplateName).then(details => {
                if (!details) {
                    setTemplateError("No se pudo cargar la información de la plantilla. Comprueba que el repositorio de plantillas esté disponible.");
                    return;
                }
                setTemplateDetails(details);
                parseTemplateVariables(details);
            }).catch(err => {
                setTemplateError(err.message || "Error al obtener detalles de la plantilla. Verifica tu conexión o los permisos.");
            });
        }
    }, [selectedTemplateName, selectedRepo]);

    const handleNext = () => {
        if (currentStep === 1) {
            if (!name || (!selectedNode && !forcedParentId)) {
                toast({ title: 'Error', description: 'Por favor completa los campos obligatorios.', variant: 'destructive' });
                return;
            }
            setCurrentStep(2);
        } else if (currentStep === 2) {
            if (!selectedTemplateName) {
                toast({ title: 'Error', description: 'Por favor selecciona una plantilla.', variant: 'destructive' });
                return;
            }
            if (!templateDetails && !templateError) {
                toast({ title: 'Cargando plantilla...', description: 'Espera un momento mientras se cargan los detalles de la plantilla.' });
                return;
            }
            setCurrentStep(3);
        }
    };

    const handleCreate = async () => {
        setLoading(true);
        try {
            const identifier = name.toLowerCase().replace(/\s+/g, '-').slice(0, 20);

            const vars: Record<string, any> = {};
            Object.entries(configData).forEach(([k, v]) => {
                vars[k] = { value: v };
            });
            
            vars['cpu'] = { value: cpuLimit === '' ? 100 : cpuLimit };
            vars['memory'] = { value: memoryLimit === '' ? 1024 : memoryLimit };
            vars['disk'] = { value: diskLimit === '' ? 10240 : diskLimit };

            const usernames = selectedUsers
                .map(id => users.find((u: any) => u.id === id)?.username)
                .filter((uname): uname is string => !!uname);

            let environmentConfig: Record<string, any> = { type: selectedEnvironment };

            if (selectedEnvironment === 'docker' && templateDetails?.supportedEnvironments) {
                const dockerEnvTemplate = templateDetails.supportedEnvironments.find(
                    (e: any) => e.type === 'docker'
                );
                if (dockerEnvTemplate) {
                    environmentConfig = { ...dockerEnvTemplate };
                } else {
                    environmentConfig = { type: 'docker', image: 'ubuntu:22.04' };
                }
            }

            const resolvedNode = selectedNode !== '' ? Number(selectedNode) : (forcedNodeId ? Number(forcedNodeId) : 0);

            const serverPayload = {
                ...templateDetails,
                name: name,
                node: resolvedNode,
                type: templateDetails.type,
                environment: environmentConfig,
                data: vars,
                users: usernames,
                parent_server_id: forcedParentId || undefined,
            };

            await api.put(`/api/servers/${identifier}`, serverPayload);

            toast({ title: 'Éxito', description: `Servidor "${name}" creado correctamente.` });
            onComplete();
        } catch (e: any) {
            toast({ title: 'Error al crear servidor', description: e?.message || 'Error desconocido.', variant: 'destructive' });
        } finally {
            setLoading(false);
        }
    };

    return (
        <ErrorBoundary>
            <div className="space-y-6 w-full max-w-full overflow-x-hidden">
                <div className="flex items-center justify-between mb-6 w-full min-w-0">
                    {[1, 2, 3].map((step) => (
                        <div key={step} className="flex items-center">
                            <div className={`flex h-8 w-8 sm:h-10 sm:w-10 items-center justify-center rounded-full border-2 transition-all duration-300 ${currentStep >= step ? 'bg-primary border-primary text-primary-foreground shadow-lg shadow-primary/20' : 'border-muted text-muted-foreground'
                                }`}>
                                {currentStep > step ? <Check className="h-4 w-4 sm:h-6 sm:w-6" /> : step}
                            </div>
                            <span className={`ml-2 text-xs sm:text-sm font-medium ${currentStep >= step ? 'text-primary' : 'text-muted-foreground'}`}>
                                {step === 1 ? 'Entorno' : step === 2 ? 'Plantilla' : 'Config.'}
                            </span>
                            {step < 3 && <div className={`mx-2 sm:mx-4 h-[2px] w-8 sm:w-24 ${currentStep > step ? 'bg-primary' : 'bg-muted'}`} />}
                        </div>
                    ))}
                </div>

                <Card className="border-0 bg-transparent shadow-none">
                    <CardContent className="p-0">
                        {currentStep === 1 && (
                            <Step1Environment
                                name={name} setName={setName}
                                selectedNode={selectedNode} setSelectedNode={setSelectedNode}
                                selectedEnvironment={selectedEnvironment} setSelectedEnvironment={setSelectedEnvironment}
                                selectedUsers={selectedUsers} setSelectedUsers={setSelectedUsers}
                                forcedParentId={forcedParentId} nodes={nodes} users={users}
                            />
                        )}
                        {currentStep === 2 && (
                            <Step2Template
                                repos={repos} selectedRepo={selectedRepo} setSelectedRepo={setSelectedRepo}
                                templateList={templateList} selectedTemplateName={selectedTemplateName} setSelectedTemplateName={setSelectedTemplateName}
                                loadingTemplates={loadingTemplates} templateDetails={templateDetails} templateError={templateError}
                            />
                        )}
                        {currentStep === 3 && (
                            <Step3Configuration
                                templateDetails={templateDetails} templateError={templateError}
                                configData={configData} setConfigData={setConfigData}
                                cpuLimit={cpuLimit} setCpuLimit={setCpuLimit}
                                memoryLimit={memoryLimit} setMemoryLimit={setMemoryLimit}
                                diskLimit={diskLimit} setDiskLimit={setDiskLimit}
                                setCurrentStep={setCurrentStep}
                            />
                        )}
                    </CardContent>
                    <CardFooter className="flex justify-between items-center mt-8 p-0 w-full">
                        <Button
                            variant="ghost"
                            onClick={() => setCurrentStep((prev) => (prev - 1) as Step)}
                            disabled={currentStep === 1 || loading}
                            className="shrink-0"
                        >
                            <ChevronLeft className="mr-2 h-4 w-4" />
                            Atrás
                        </Button>
                        {currentStep < 3 ? (
                            <Button
                                onClick={handleNext}
                                disabled={currentStep === 2 && !!selectedTemplateName && !templateDetails && !templateError}
                                className="shrink-0"
                            >
                                {currentStep === 2 && !!selectedTemplateName && !templateDetails && !templateError
                                    ? <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                    : null
                                }
                                Siguiente
                                <ChevronRight className="ml-2 h-4 w-4" />
                            </Button>
                        ) : (
                            <Button onClick={handleCreate} disabled={loading} className="bg-primary hover:bg-primary/90 shrink-0">
                                {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                                <Check className="mr-2 h-4 w-4" />
                                Crear Servidor
                            </Button>
                        )}
                    </CardFooter>
                </Card>
            </div>
        </ErrorBoundary>
    );
}