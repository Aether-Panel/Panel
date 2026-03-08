'use client';

import React, { useState, useEffect } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Loader2, ChevronRight, ChevronLeft, Check, Server as ServerIcon, Code, Puzzle, Bot, Globe, Shield, X, ShieldAlert } from 'lucide-react';
import { useNodes } from '@/hooks/use-dashboard-data';
import { useUsers } from '@/hooks/use-users';
import { useTemplates, type TemplateRepo } from '@/hooks/use-templates';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { useToast } from '@/hooks/use-toast';

// Error Boundary component to prevent the whole app from crashing
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

// Helper to safely render values that might be objects
const SafeValue = ({ v, fallback = "" }: { v: any, fallback?: string }) => {
    if (v === null || v === undefined) return fallback;
    if (typeof v === 'object') return JSON.stringify(v);
    return String(v);
};

export function CreateServerStepper({ onComplete }: { onComplete: () => void }) {
    const { t } = useTranslations();
    const { toast } = useToast();
    const { nodes } = useNodes();
    const { users } = useUsers();
    const { repos, getTemplatesForRepo, getTemplateDetails } = useTemplates();

    const [currentStep, setCurrentStep] = useState<Step>(1);
    const [loading, setLoading] = useState(false);

    // Step 1: Environment
    const [name, setName] = useState('');
    const [selectedNode, setSelectedNode] = useState('');
    const [selectedEnvironment, setSelectedEnvironment] = useState('docker');
    const [selectedUsers, setSelectedUsers] = useState<number[]>([]);

    // Step 2: Templates
    const [selectedRepo, setSelectedRepo] = useState<number | null>(null);
    const [templateList, setTemplateList] = useState<any[]>([]);
    const [selectedTemplateName, setSelectedTemplateName] = useState('');
    const [loadingTemplates, setLoadingTemplates] = useState(false);

    // Step 3: Configuration
    const [templateDetails, setTemplateDetails] = useState<any>(null);
    const [templateError, setTemplateError] = useState<string | null>(null);
    const [configData, setConfigData] = useState<Record<string, any>>({});

    useEffect(() => {
        if (selectedRepo !== null) {
            setLoadingTemplates(true);
            setSelectedTemplateName('');
            setTemplateDetails(null);
            setTemplateError(null);
            getTemplatesForRepo(selectedRepo).then(list => {
                setTemplateList(list);
                setLoadingTemplates(false);
            });
        }
    }, [selectedRepo]);

    useEffect(() => {
        if (selectedTemplateName && selectedRepo !== null) {
            setTemplateError(null);
            getTemplateDetails(selectedRepo, selectedTemplateName).then(details => {
                console.log('Template Details Received:', details);
                if (!details) {
                    setTemplateError("No se pudo cargar la información de la plantilla.");
                    return;
                }
                setTemplateDetails(details);

                // Initialize config data with default values
                const initial: Record<string, any> = {};
                // PufferPanel templates use "data" or "variables" or "Variables"
                const rawVars = details.data || details.variables || details.Variables || {};

                // Ensure rawVars is an object we can iterate over
                const variables = (typeof rawVars === 'object' && rawVars !== null) ? rawVars : {};

                Object.entries(variables).forEach(([key, val]: [string, any]) => {
                    // Normalize the value to a string/number/boolean safe for inputs
                    if (val && typeof val === 'object') {
                        let rawValue = val.value !== undefined ? val.value : (val.default !== undefined ? val.default : "");

                        // Old frontend logic for booleans
                        if (val.type === 'boolean') {
                            rawValue = rawValue !== 'false' && rawValue !== false && !!rawValue;
                        }
                        // For integer
                        if (val.type === 'integer') {
                            rawValue = Number(rawValue) || 0;
                        }

                        initial[key] = (typeof rawValue === 'object' && rawValue !== null)
                            ? JSON.stringify(rawValue)
                            : (rawValue ?? "");
                    } else if (val !== undefined && val !== null) {
                        initial[key] = String(val);
                    }
                });
                console.log('Initialized Config Data:', initial);
                setConfigData(initial);
            }).catch(err => {
                setTemplateError(err.message || "Error al obtener detalles de la plantilla.");
                console.error("Details Fetch Error:", err);
            });
        }
    }, [selectedTemplateName, selectedRepo]);

    const handleNext = () => {
        if (currentStep === 1) {
            if (!name || !selectedNode) {
                toast({ title: 'Error', description: 'Por favor completa los campos obligatorios.', variant: 'destructive' });
                return;
            }
            setCurrentStep(2);
        } else if (currentStep === 2) {
            if (!selectedTemplateName) {
                toast({ title: 'Error', description: 'Por favor selecciona una plantilla.', variant: 'destructive' });
                return;
            }
            setCurrentStep(3);
        }
    };

    const handleCreate = async () => {
        setLoading(true);
        try {
            // Identifier is usually a slug of the name or generated
            const identifier = name.toLowerCase().replace(/\s+/g, '-').slice(0, 20);

            // Format config data as { key: { value: val } } for Go backend
            const vars: Record<string, any> = {};
            Object.entries(configData).forEach(([k, v]) => {
                vars[k] = { value: v };
            });

            const usernames = selectedUsers
                .map(id => users.find(u => u.id === id)?.username)
                .filter((uname): uname is string => !!uname);

            // Combine template definition with user overrides
            const serverPayload = {
                ...templateDetails,
                name: name,
                node: Number(selectedNode),
                type: templateDetails.type, // Server type (e.g., minecraft-java)
                environment: { type: selectedEnvironment }, // Environment type (host/docker)
                data: vars,
                users: usernames,
            };

            console.log('Final Server Payload:', serverPayload);

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
            <div className="space-y-6">
                {/* Stepper Indicator */}
                <div className="flex items-center justify-between mb-8 max-w-2xl mx-auto">
                    {[1, 2, 3].map((step) => (
                        <div key={step} className="flex items-center">
                            <div className={`flex h-10 w-10 items-center justify-center rounded-full border-2 transition-all duration-300 ${currentStep >= step ? 'bg-primary border-primary text-primary-foreground shadow-lg shadow-primary/20' : 'border-muted text-muted-foreground'
                                }`}>
                                {currentStep > step ? <Check className="h-6 w-6" /> : step}
                            </div>
                            <span className={`ml-3 text-sm font-medium ${currentStep >= step ? 'text-primary' : 'text-muted-foreground'}`}>
                                {step === 1 ? 'Entorno' : step === 2 ? 'Plantilla' : 'Configuración'}
                            </span>
                            {step < 3 && <div className={`mx-4 h-[2px] w-12 sm:w-24 ${currentStep > step ? 'bg-primary' : 'bg-muted'}`} />}
                        </div>
                    ))}
                </div>

                <Card className="border-0 bg-transparent shadow-none">
                    <CardContent className="p-0">
                        {currentStep === 1 && (
                            <div className="grid gap-6 animate-in fade-in slide-in-from-right-4 duration-300">
                                <div className="grid gap-2">
                                    <Label htmlFor="server-name">Nombre del Servidor *</Label>
                                    <Input id="server-name" placeholder="Ej: Mi Servidor Minecraft" value={name} onChange={(e) => setName(e.target.value)} />
                                </div>
                                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                    <div className="grid gap-2">
                                        <Label>Nodo *</Label>
                                        <Select value={selectedNode} onValueChange={setSelectedNode}>
                                            <SelectTrigger>
                                                <SelectValue placeholder="Seleccionar nodo" />
                                            </SelectTrigger>
                                            <SelectContent>
                                                {nodes.map(n => (
                                                    <SelectItem key={n.id} value={String(n.id)}>{n.name} ({n.publicHost})</SelectItem>
                                                ))}
                                            </SelectContent>
                                        </Select>
                                    </div>
                                    <div className="grid gap-2">
                                        <Label>Entorno</Label>
                                        <Select value={selectedEnvironment} onValueChange={setSelectedEnvironment}>
                                            <SelectTrigger>
                                                <SelectValue />
                                            </SelectTrigger>
                                            <SelectContent>
                                                <SelectItem value="docker">Docker</SelectItem>
                                                <SelectItem value="standard">Estándar (Hijo)</SelectItem>
                                                <SelectItem value="tty">TTY</SelectItem>
                                            </SelectContent>
                                        </Select>
                                    </div>
                                </div>
                                <div className="grid gap-2">
                                    <Label>Usuarios con acceso</Label>
                                    <div className="flex flex-col gap-3">
                                        <Select
                                            value=""
                                            onValueChange={(val) => {
                                                const id = Number(val);
                                                if (!selectedUsers.includes(id)) {
                                                    setSelectedUsers(prev => [...prev, id]);
                                                }
                                            }}
                                        >
                                            <SelectTrigger>
                                                <SelectValue placeholder="Seleccionar usuario para añadir..." />
                                            </SelectTrigger>
                                            <SelectContent>
                                                {users.filter(u => !selectedUsers.includes(u.id)).map(u => (
                                                    <SelectItem key={u.id} value={String(u.id)}>{u.username} ({u.email})</SelectItem>
                                                ))}
                                                {users.filter(u => !selectedUsers.includes(u.id)).length === 0 && (
                                                    <div className="p-2 text-xs text-muted-foreground text-center italic">No hay más usuarios disponibles.</div>
                                                )}
                                            </SelectContent>
                                        </Select>

                                        {selectedUsers.length > 0 && (
                                            <div className="flex flex-wrap gap-2 pt-1">
                                                {selectedUsers.map(id => {
                                                    const u = users.find(user => user.id === id);
                                                    return (
                                                        <Badge key={id} variant="secondary" className="pl-3 pr-1 py-1 flex items-center gap-2 bg-primary/10 text-primary border-primary/20">
                                                            {u?.username}
                                                            <Button
                                                                variant="ghost"
                                                                size="icon"
                                                                className="h-4 w-4 rounded-full hover:bg-red-500 hover:text-white transition-colors"
                                                                onClick={() => setSelectedUsers(prev => prev.filter(uid => uid !== id))}
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
                            </div>
                        )}

                        {currentStep === 2 && (
                            <div className="grid gap-6 animate-in fade-in slide-in-from-right-4 duration-300">
                                <div className="grid gap-2">
                                    <Label>Repositorio de Plantillas</Label>
                                    <div className="grid grid-cols-2 gap-4">
                                        {repos.map(r => (
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
                                    </div>
                                </div>

                                {selectedRepo !== null && (
                                    <div className="grid gap-2 mt-4">
                                        <Label>Seleccionar Plantilla</Label>
                                        {loadingTemplates ? (
                                            <div className="flex items-center justify-center p-8">
                                                <Loader2 className="h-8 w-8 animate-spin text-primary" />
                                            </div>
                                        ) : (
                                            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3">
                                                {templateList.map(t => (
                                                    <div
                                                        key={t.name}
                                                        onClick={() => setSelectedTemplateName(t.name)}
                                                        className={`p-3 rounded-lg border text-sm flex items-center gap-3 cursor-pointer transition-all ${selectedTemplateName === t.name ? 'bg-primary text-primary-foreground border-primary' : 'hover:bg-muted'
                                                            }`}
                                                    >
                                                        <Code className="h-4 w-4" />
                                                        <span className="truncate flex-1">{t.name}</span>
                                                    </div>
                                                ))}
                                            </div>
                                        )}
                                    </div>
                                )}
                            </div>
                        )}

                        {currentStep === 3 && (
                            <div className="grid gap-6 animate-in fade-in-0 duration-300">
                                {templateError ? (
                                    <div className="flex flex-col items-center justify-center p-12 text-center bg-red-500/5 rounded-xl border border-red-500/20">
                                        <ShieldAlert className="h-12 w-12 text-red-500 mb-4" />
                                        <p className="font-bold text-lg text-red-700">Error de Plantilla</p>
                                        <p className="text-sm text-red-600 mt-2 max-w-sm">{templateError}</p>
                                        <Button variant="outline" className="mt-6 border-red-200 hover:bg-red-50 text-red-700" onClick={() => setCurrentStep(2)}>
                                            <ChevronLeft className="mr-2 h-4 w-4" />
                                            Seleccionar otra plantilla
                                        </Button>
                                    </div>
                                ) : templateDetails ? (
                                    <div className="space-y-6">
                                        <div className="bg-primary/5 p-5 rounded-xl flex items-start gap-4 border border-primary/10 shadow-sm">
                                            <div className="bg-primary/20 p-2.5 rounded-lg shadow-inner">
                                                <Code className="h-6 w-6 text-primary" />
                                            </div>
                                            <div className="space-y-1">
                                                <h4 className="font-bold text-lg leading-none"><SafeValue v={templateDetails.display || templateDetails.name} /></h4>
                                                <p className="text-xs text-muted-foreground leading-relaxed">
                                                    <SafeValue v={templateDetails.readme} fallback="Configure los parámetros necesarios para esta plantilla. Los valores por defecto han sido precargados." />
                                                </p>
                                            </div>
                                        </div>

                                        <div className="grid grid-cols-1 md:grid-cols-2 gap-x-10 gap-y-8 pt-2">
                                            {(() => {
                                                try {
                                                    const variables = (templateDetails.data || templateDetails.variables || templateDetails.Variables || {}) as Record<string, any>;
                                                    const entries = Object.entries(variables).filter(([_, v]) => v && !v.internal);

                                                    if (entries.length === 0) {
                                                        return (
                                                            <div className="col-span-full py-10 text-center border-2 border-dashed rounded-xl border-muted-foreground/20">
                                                                <p className="text-muted-foreground italic">Esta plantilla no requiere configuración adicional.</p>
                                                            </div>
                                                        );
                                                    }

                                                    return entries.map(([key, variable]: [string, any], idx: number) => {
                                                        try {
                                                            if (!variable || typeof variable !== 'object') return null;

                                                            const currentVal = configData[key] !== undefined ? configData[key] : "";
                                                            const stringifiedVal = (typeof currentVal === 'object' && currentVal !== null)
                                                                ? JSON.stringify(currentVal)
                                                                : String(currentVal ?? "");

                                                            return (
                                                                <div key={key + idx} className="space-y-2.5 animate-in fade-in slide-in-from-bottom-2 duration-300" style={{ animationDelay: `${idx * 50}ms` }}>
                                                                    <div className="flex justify-between items-center group">
                                                                        <Label className="text-sm font-bold text-foreground/80 group-hover:text-primary transition-colors">
                                                                            <SafeValue v={variable.display || key} />
                                                                            {variable.required && <span className="text-red-500 ml-1 font-black">*</span>}
                                                                        </Label>
                                                                        <Badge variant="secondary" className="text-[10px] uppercase tracking-wider font-bold h-5 px-1.5 opacity-70 group-hover:opacity-100 transition-opacity">
                                                                            <SafeValue v={variable.type} fallback="string" />
                                                                        </Badge>
                                                                    </div>

                                                                    {Array.isArray(variable.options) && variable.options.length > 0 ? (
                                                                        <Select
                                                                            value={stringifiedVal === "" ? "_EMPTY_VALUE_" : stringifiedVal}
                                                                            onValueChange={(val) => setConfigData(prev => ({ ...prev, [key]: val === "_EMPTY_VALUE_" ? "" : val }))}
                                                                        >
                                                                            <SelectTrigger className="bg-background/50 border-muted-foreground/20 hover:border-primary/50 transition-all">
                                                                                <SelectValue />
                                                                            </SelectTrigger>
                                                                            <SelectContent>
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
                                                                        <div className="flex items-center gap-3 p-3 rounded-lg border border-muted-foreground/20 bg-background/30 hover:bg-background/50 transition-colors cursor-pointer" onClick={() => setConfigData(prev => ({ ...prev, [key]: !currentVal }))}>
                                                                            <div className={`h-5 w-5 rounded border transition-colors flex items-center justify-center ${currentVal ? 'bg-primary border-primary' : 'border-muted-foreground/30 bg-transparent'}`}>
                                                                                {currentVal && <Check className="h-3.5 w-3.5 text-primary-foreground stroke-[3px]" />}
                                                                            </div>
                                                                            <span className="text-sm font-medium select-none">Habilitar esta opción</span>
                                                                        </div>
                                                                    ) : (
                                                                        <Input
                                                                            type={variable.type === 'integer' ? 'number' : 'text'}
                                                                            value={stringifiedVal}
                                                                            onChange={(e) => setConfigData(prev => ({ ...prev, [key]: variable.type === 'integer' ? Number(e.target.value) : e.target.value }))}
                                                                            className="bg-background/50 border-muted-foreground/20 hover:border-primary/50 focus-visible:ring-primary/20 transition-all"
                                                                        />
                                                                    )}
                                                                    {variable.desc && (
                                                                        <p className="text-[11px] text-muted-foreground leading-snug px-1 italic">
                                                                            <SafeValue v={variable.desc} />
                                                                        </p>
                                                                    )}
                                                                </div>
                                                            );
                                                        } catch (err) {
                                                            console.error("Error rendering variable:", key, err);
                                                            return null;
                                                        }
                                                    });
                                                } catch (e) {
                                                    console.error("Critical rendering error in Step 3:", e);
                                                    return (
                                                        <div className="col-span-full py-8 text-center text-red-500 bg-red-50 rounded-xl border border-red-100 flex flex-col items-center">
                                                            <ShieldAlert className="h-8 w-8 mb-2" />
                                                            <p className="font-bold">Error al procesar los campos de la plantilla</p>
                                                            <p className="text-xs opacity-80 mt-1">La estructura de esta plantilla no es compatible con el panel actual.</p>
                                                        </div>
                                                    );
                                                }
                                            })()}
                                        </div>
                                    </div>
                                ) : (
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
                                )}
                            </div>
                        )}
                    </CardContent>
                    <CardFooter className="flex justify-between mt-8 p-0">
                        <Button
                            variant="ghost"
                            onClick={() => setCurrentStep(prev => (prev - 1) as Step)}
                            disabled={currentStep === 1 || loading}
                        >
                            <ChevronLeft className="mr-2 h-4 w-4" />
                            Atrás
                        </Button>
                        {currentStep < 3 ? (
                            <Button onClick={handleNext}>
                                Siguiente
                                <ChevronRight className="ml-2 h-4 w-4" />
                            </Button>
                        ) : (
                            <Button onClick={handleCreate} disabled={loading} className="bg-primary hover:bg-primary/90">
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
