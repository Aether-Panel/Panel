'use client';

import React, { useState, useEffect, useMemo, useRef } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Checkbox } from '@/components/ui/checkbox';
import { Loader2, ChevronRight, ChevronLeft, Check, Code, Globe, Shield, X, ShieldAlert, Plus } from '@/components/icons';
import { useNodes } from '@/hooks/use-dashboard-data';
import { useUsers } from '@/hooks/use-users';
import { useTemplates } from '@/hooks/use-templates';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { sileo } from "@/lib/toast";
import { cn } from '@/lib/utils';
import { gameLogo } from '@/lib/game-logos';
import { keyedByContent } from '@/lib/keyed';

type TFn = (key: string, options?: { [key: string]: string | number }) => string;

class ErrorBoundary extends React.Component<
    { children: React.ReactNode; t: TFn },
    { hasError: boolean; errorMsg: string; errorStack: string }
> {
    constructor(props: any) {
        super(props);
        this.state = { hasError: false, errorMsg: '', errorStack: '' };
    }
    static getDerivedStateFromError(error: any) {
        return { hasError: true, errorMsg: String(error?.message || error || ''), errorStack: String(error?.stack || '') };
    }
    componentDidCatch(error: any, errorInfo: any) {
        console.error("[Stepper Error Boundary] Caught:", error);
        console.error("[Stepper Error Boundary] Component stack:", errorInfo?.componentStack);
    }
    render() {
        const { t } = this.props;
        if (this.state.hasError) {
            return (
                <div className="rounded-xl border border-dashed border-destructive/40 px-6 py-10 text-center">
                    <ShieldAlert className="mx-auto mb-3 h-6 w-6 text-destructive-bright" aria-hidden="true" />
                    <h3 className="mb-2 text-base font-semibold">{t('servers.create.renderError')}</h3>
                    <pre className="max-h-40 overflow-auto whitespace-pre-wrap break-all rounded-lg border border-border/60 bg-background/40 p-3 text-left font-mono text-xs text-muted-foreground">{this.state.errorMsg || t('servers.create.unknownError')}{"\n\n"}{this.state.errorStack.slice(0, 500)}</pre>
                    <div className="mt-4 flex justify-center gap-2">
                        <Button variant="outline" size="sm" onClick={() => this.setState({ hasError: false, errorMsg: '', errorStack: '' })}>{t('common.retry')}</Button>
                        <Button variant="outline" size="sm" onClick={() => window.location.reload()}>{t('servers.create.reload')}</Button>
                    </div>
                </div>
            );
        }
        return this.props.children;
    }
}

type Step = 1 | 2 | 3;

/** Template variables that fill the resource inputs, with the value used when the template leaves them empty. */
const RESOURCE_DEFAULTS: Record<string, number> = { cpu: 100, memory: 1024, disk: 10240, io: 500 };
const isResourceKey = (key: string) => Object.hasOwn(RESOURCE_DEFAULTS, key);

/**
 * The server id the API accepts (letters, digits, '-' and '_', up to 20, not starting with
 * '-' or '_'), from the name plus a random suffix so two servers named alike never collide.
 * The name itself stays the display name, with any characters it likes.
 */
function serverIdFromName(name: string) {
    const suffix = crypto.getRandomValues(new Uint32Array(1))[0].toString(36).padStart(6, '0').slice(0, 6);
    const base = name
        .normalize('NFD')
        .replaceAll(/[\u0300-\u036f]/g, '')
        .toLowerCase()
        .replaceAll(/[^a-z0-9_-]+/g, '-')
        .replaceAll(/^[-_]+|[-_]+$/g, '')
        .slice(0, 13)
        .replaceAll(/[-_]+$/g, '');
    return base ? `${base}-${suffix}` : `srv-${suffix}`;
}

/** A template variable's starting value: its value, else its default, coerced to the variable's type. */
function templateVariableValue(val: any) {
    let rawValue: any = "";
    if (val.value !== undefined) rawValue = val.value;
    else if (val.default !== undefined) rawValue = val.default;

    if (val.type === 'boolean') {
        rawValue = rawValue !== 'false' && rawValue !== false && !!rawValue;
    }
    if (val.type === 'integer') {
        rawValue = Number(rawValue) || 0;
    }
    return rawValue;
}

/** Template descriptions may carry a little HTML (links, <strong>); the form shows them as plain text. */
function plainDescription(desc: unknown): string {
    if (typeof desc !== 'string') return '';
    // Convert anchor tags to "text (url)" before stripping so links are preserved.
    const withLinks = desc.replace(/<a\s[^>]*href=['"]([^'"]+)['"][^>]*>(.*?)<\/a>/gi, '$2 ($1)');
    // Use the browser's HTML parser for safe, complete tag removal – no regex
    // backtracking and immune to nested-tag tricks like "<scr<b>ipt>".
    const doc = new DOMParser().parseFromString(withLinks, 'text/html');
    return (doc.body.textContent ?? '')
        .replace(/\s+/g, ' ')
        .trim();
}

type VersionTarget = { platform: string; variable: string };

/**
 * The install step that downloads the game and the variable that holds its version, found the
 * way the panel's version switch finds it: vanilla through mojangdl, PaperMC projects through
 * paperdl. Its variable gets a list of real releases instead of a free text field.
 */
function templateVersionTarget(details: any, values: Record<string, any>): VersionTarget | null {
    const ops: any[] = Array.isArray(details?.install) ? details.install : [];
    const vars = details?.data || {};
    const tokenOf = (arg: unknown) => (typeof arg === 'string' ? /^\$\{(\w+)\}$/.exec(arg.trim())?.[1] : undefined);
    for (const op of ops) {
        let variable: string | undefined;
        let platform = '';
        if (op?.type === 'mojangdl') {
            variable = tokenOf(op.version);
            platform = 'vanilla';
        } else if (op?.type === 'paperdl') {
            variable = tokenOf(op.minecraftVersion);
            const project = String(op.project ?? '');
            const projectVar = tokenOf(project);
            platform = (projectVar ? String(values[projectVar] ?? '') : project).trim().toLowerCase() || 'paper';
        }
        if (variable && vars[variable]) return { platform, variable };
    }
    return null;
}

/** The Java releases that have an eclipse-temurin image, newest first. */
const TEMURIN_JAVA_VERSIONS = ['25', '21', '17', '11', '8'];

/**
 * The Java a Minecraft release needs: 26.1 and newer 25, 1.20.5 to 1.21.x 21, 1.17 to 1.20.4 17,
 * older ones 8. Null for what is not a release number (a snapshot, an empty field).
 */
function minecraftJavaFor(version: string): string | null {
    const year = /^(\d{2,})\.\d+/.exec(version.trim());
    if (year && Number(year[1]) >= 26) return '25';
    const release = /^1\.(\d+)(?:\.(\d+))?/.exec(version.trim());
    if (!release) return null;
    const minor = Number(release[1]);
    const patch = Number(release[2] ?? 0);
    if (minor >= 21 || (minor === 20 && patch >= 5)) return '21';
    if (minor >= 17) return '17';
    return '8';
}

/**
 * The variable that picks the Java image, when the template runs eclipse-temurin:${variable}.
 * Typed freely, a value like 200 names an image that does not exist and the server cannot
 * start, so the form offers the releases that do.
 */
function javaImageVariable(details: any): string | null {
    const environments: any[] = [details?.environment, ...(Array.isArray(details?.supportedEnvironments) ? details.supportedEnvironments : [])];
    for (const env of environments) {
        const match = typeof env?.image === 'string' ? /^eclipse-temurin:\$\{(\w+)\}$/.exec(env.image.trim()) : null;
        if (match && details?.data?.[match[1]]) return match[1];
    }
    return null;
}

type FieldError = { code: 'required' | 'notNumber' | 'min' | 'max' | 'eula'; limit?: number };

/**
 * Checks the values before the server is created: a required field cannot be empty, a number
 * must be a whole number inside the template's min/max (a required number starts at 1 unless
 * the template says otherwise, so an ID left at 0 counts as missing).
 */
function validateTemplateValues(variables: Record<string, any>, values: Record<string, any>): Record<string, FieldError> {
    const errors: Record<string, FieldError> = {};
    for (const [key, v] of Object.entries(variables)) {
        if (!v || typeof v !== 'object' || v.internal || isResourceKey(key)) continue;
        const value = values[key];
        // a required checkbox only has to have a value (PvP off is fine); Minecraft's EULA is the
        // exception, as the server stops at once until it is accepted
        if (v.type === 'boolean') {
            if (key.toLowerCase() === 'eula' && value !== true && value !== 'true') errors[key] = { code: 'eula' };
            continue;
        }
        const empty = value === undefined || value === null || String(value).trim() === '';
        if (v.type !== 'integer') {
            if (v.required && empty) errors[key] = { code: 'required' };
            continue;
        }
        if (empty) {
            if (v.required) errors[key] = { code: 'required' };
            continue;
        }
        const n = Number(value);
        if (!Number.isInteger(n)) {
            errors[key] = { code: 'notNumber' };
            continue;
        }
        const hasMin = typeof v.min === 'number';
        const min = hasMin ? v.min : v.required ? 1 : 0;
        if (n < min) {
            errors[key] = !hasMin && v.required && n === 0 ? { code: 'required' } : { code: 'min', limit: min };
        } else if (typeof v.max === 'number' && n > v.max) {
            errors[key] = { code: 'max', limit: v.max };
        }
    }
    return errors;
}

/**
 * The releases of a game platform for the version select. Empty while loading or when Mojang
 * and PaperMC cannot be reached: the field is then plain text, as before.
 */
function useGameVersions(platform: string | undefined): string[] {
    const [versions, setVersions] = useState<string[]>([]);
    useEffect(() => {
        setVersions([]);
        if (!platform) return;
        let cancelled = false;
        api.get(`/api/game-versions/${encodeURIComponent(platform)}`)
            .then((list: unknown) => {
                if (!cancelled && Array.isArray(list)) setVersions(list.map(String));
            })
            .catch(() => undefined);
        return () => {
            cancelled = true;
        };
    }, [platform]);
    return versions;
}

/** The id of a variable's field, so an error can move the focus to it. */
const variableFieldId = (key: string) => `tpl-var-${key.replace(/[^\w-]/g, '_')}`;

/** What the config form holds for a variable: objects as JSON, anything else as is. */
const toConfigValue = (rawValue: any) =>
    (typeof rawValue === 'object' && rawValue !== null) ? JSON.stringify(rawValue) : (rawValue ?? "");

const SafeValue = ({ v, fallback = "" }: { v: any, fallback?: string }) => {
    if (v === null || v === undefined) return fallback;
    if (typeof v === 'object') return JSON.stringify(v);
    return String(v);
};

/** Small section heading used inside the steps (title + optional hint). */
function StepSection({ title, hint, children }: Readonly<{ title: string; hint?: React.ReactNode; children: React.ReactNode }>) {
    return (
        <section className="grid gap-3">
            <div className="space-y-0.5">
                <h3 className="text-sm font-semibold">{title}</h3>
                {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
            </div>
            {children}
        </section>
    );
}

const RequiredMark = () => <span className="ml-0.5 text-destructive-bright" aria-hidden="true">*</span>;

function Step1Environment({
    name, setName, selectedNode, setSelectedNode, selectedEnvironment, setSelectedEnvironment,
    selectedUsers, setSelectedUsers, forcedParentId, nodes, users, t
}: any) {
    const availableUsers = users.filter((u: any) => !selectedUsers.includes(u.id));
    return (
        <div className="grid w-full gap-5">
            <div className="grid gap-2">
                <Label htmlFor="server-name">{t('servers.create.nameLabel')}<RequiredMark /></Label>
                <Input id="server-name" placeholder={t('servers.create.namePlaceholder')} value={name} onChange={(e) => setName(e.target.value)} required autoComplete="off" />
            </div>
            {forcedParentId ? (
                <div className="flex gap-3 rounded-xl border border-border/60 bg-background/30 p-4">
                    <Shield className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
                    <div className="space-y-1">
                        <p className="text-sm font-medium">{t('servers.create.inheritedTitle')}</p>
                        <p className="text-sm text-muted-foreground">{t('servers.create.inheritedDescription')}</p>
                    </div>
                </div>
            ) : (
                <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 sm:gap-4">
                    <div className="grid gap-2">
                        <Label htmlFor="server-node">{t('servers.create.nodeLabel')}<RequiredMark /></Label>
                        <Select value={selectedNode} onValueChange={setSelectedNode}>
                            <SelectTrigger id="server-node">
                                <SelectValue placeholder={t('servers.create.nodePlaceholder')} />
                            </SelectTrigger>
                            <SelectContent className="z-[100] max-h-80 overflow-y-auto">
                                {nodes.map((n: any) => (
                                    <SelectItem key={n.id} value={String(n.id)}>
                                        {n.name} <span className="font-mono text-xs text-muted-foreground">{n.publicHost}</span>
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>
                    <div className="grid gap-2">
                        <Label htmlFor="server-environment">{t('servers.create.environmentLabel')}</Label>
                        <Select value={selectedEnvironment} onValueChange={setSelectedEnvironment}>
                            <SelectTrigger id="server-environment">
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent className="z-[100] max-h-80 overflow-y-auto">
                                <SelectItem value="docker">{t('servers.create.environment.docker')}</SelectItem>
                                <SelectItem value="standard">{t('servers.create.environment.standard')}</SelectItem>
                                <SelectItem value="tty">{t('servers.create.environment.tty')}</SelectItem>
                            </SelectContent>
                        </Select>
                    </div>
                </div>
            )}
            {!forcedParentId && (
                <div className="grid gap-2">
                    <Label htmlFor="server-users">{t('servers.create.usersLabel')}</Label>
                    <Select
                        value=""
                        onValueChange={(val) => {
                            const id = Number(val);
                            if (!selectedUsers.includes(id)) {
                                setSelectedUsers((prev: any) => [...prev, id]);
                            }
                        }}
                    >
                        <SelectTrigger id="server-users" aria-describedby="server-users-hint">
                            <SelectValue placeholder={t('servers.create.usersPlaceholder')} />
                        </SelectTrigger>
                        <SelectContent className="z-[100] max-h-80 overflow-y-auto">
                            {availableUsers.map((u: any) => (
                                <SelectItem key={u.id} value={String(u.id)}>
                                    {u.username} <span className="text-xs text-muted-foreground">{u.email}</span>
                                </SelectItem>
                            ))}
                            {availableUsers.length === 0 && (
                                <div className="p-2 text-center text-xs text-muted-foreground">{t('servers.create.usersEmpty')}</div>
                            )}
                        </SelectContent>
                    </Select>

                    {selectedUsers.length > 0 && (
                        <ul className="flex flex-wrap gap-2 pt-1">
                            {selectedUsers.map((id: number) => {
                                const u = users.find((user: any) => user.id === id);
                                return (
                                    <li key={id} className="inline-flex items-center gap-1 rounded-md border border-border/60 bg-secondary py-0.5 pl-2.5 pr-0.5 text-xs font-medium">
                                        {u?.username}
                                        <Button
                                            type="button"
                                            variant="ghost"
                                            size="icon"
                                            className="h-6 w-6 rounded text-muted-foreground hover:bg-destructive/10 hover:text-destructive-bright"
                                            onClick={() => setSelectedUsers((prev: any) => prev.filter((uid: number) => uid !== id))}
                                            aria-label={t('servers.create.removeUser', { name: u?.username ?? String(id) })}
                                        >
                                            <X className="h-3.5 w-3.5" aria-hidden="true" />
                                        </Button>
                                    </li>
                                );
                            })}
                        </ul>
                    )}
                    <p id="server-users-hint" className="text-xs text-muted-foreground">{t('servers.create.usersHint')}</p>
                </div>
            )}
        </div>
    );
}

/** The template grid of the chosen repository, or its loading / empty state. */
function TemplatePicker({ loadingTemplates, templateList, selectedTemplateName, setSelectedTemplateName, t }: any) {
    if (loadingTemplates) {
        return (
            <div className="flex items-center justify-center rounded-xl border border-border/60 bg-background/30 p-8">
                <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" aria-label={t('common.loading')} />
            </div>
        );
    }
    if (templateList.length === 0) {
        return (
            <div className="rounded-xl border border-dashed border-border/60 px-6 py-8 text-center text-sm text-muted-foreground">
                {t('servers.create.templatesEmpty')}
            </div>
        );
    }
    return (
        <div className="max-h-[280px] w-full min-w-0 overflow-y-auto rounded-xl border border-border/60 bg-background/30 p-2">
            <div className="grid w-full grid-cols-2 gap-1.5 sm:grid-cols-3">
                {templateList.map((tpl: any) => {
                    const active = selectedTemplateName === tpl.name;
                    const logo = gameLogo(tpl);
                    return (
                        <button
                            key={tpl.name}
                            type="button"
                            aria-pressed={active}
                            onClick={() => setSelectedTemplateName(tpl.name)}
                            title={tpl.name}
                            className={cn(
                                'focus-ring flex h-9 min-w-0 items-center gap-2 rounded-lg border px-2.5 text-left text-xs transition-colors duration-200',
                                active
                                    ? 'border-primary/60 bg-primary/15 text-foreground'
                                    : 'border-transparent text-muted-foreground hover:bg-accent/40 hover:text-foreground'
                            )}
                        >
                            {logo
                                ? <img src={logo} alt="" width={18} height={18} loading="lazy" className="h-[18px] w-[18px] shrink-0 rounded object-cover" />
                                : active
                                    ? <Check className="h-3.5 w-3.5 shrink-0 text-primary-bright" aria-hidden="true" />
                                    : <Code className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />}
                            <span className="min-w-0 truncate font-mono">{tpl.name}</span>
                        </button>
                    );
                })}
            </div>
        </div>
    );
}

function Step2Template({
    repos, selectedRepo, setSelectedRepo,
    templateList, selectedTemplateName, setSelectedTemplateName,
    loadingTemplates, templateDetails, templateError, t
}: any) {
    const activeRepo = repos.find((r: any) => r.id === selectedRepo);
    return (
        <div className="grid w-full gap-6">
            <StepSection title={t('servers.create.repoLabel')}>
                <div className="grid w-full grid-cols-1 gap-3 sm:grid-cols-2">
                    {repos.map((r: any) => {
                        const active = selectedRepo === r.id;
                        const RepoIcon = r.isLocal ? Shield : Globe;
                        return (
                            <button
                                key={r.id}
                                type="button"
                                aria-pressed={active}
                                onClick={() => setSelectedRepo(r.id)}
                                className={cn(
                                    'focus-ring flex min-w-0 items-center gap-3 rounded-xl border p-3.5 text-left transition-colors duration-200',
                                    active
                                        ? 'border-primary/60 bg-primary/10'
                                        : 'border-border/60 bg-background/30 hover:border-border hover:bg-accent/30'
                                )}
                            >
                                <RepoIcon className={cn('h-4 w-4 shrink-0', active ? 'text-primary-bright' : 'text-muted-foreground')} aria-hidden="true" />
                                <span className="min-w-0 flex-1">
                                    <span className="block truncate text-sm font-medium">{r.name}</span>
                                    <span className="block text-xs text-muted-foreground">{r.isLocal ? t('servers.create.repoLocal') : t('servers.create.repoCommunity')}</span>
                                </span>
                                {active && <Check className="h-4 w-4 shrink-0 text-primary-bright" aria-hidden="true" />}
                            </button>
                        );
                    })}
                    {repos.length === 0 && (
                        <div className="col-span-full flex items-center justify-center gap-2 rounded-xl border border-dashed border-border/60 p-8 text-sm text-muted-foreground">
                            <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
                            {t('servers.create.loadingRepos')}
                        </div>
                    )}
                </div>
            </StepSection>

            {selectedRepo !== null && (
                <StepSection
                    title={t('servers.create.templateLabel')}
                    hint={activeRepo ? t('servers.create.templatesFrom', { repo: activeRepo.name }) : undefined}
                >
                    <TemplatePicker
                        loadingTemplates={loadingTemplates}
                        templateList={templateList}
                        selectedTemplateName={selectedTemplateName}
                        setSelectedTemplateName={setSelectedTemplateName}
                        t={t}
                    />
                    {selectedTemplateName && !templateDetails && !templateError && (
                        <output className="flex items-center gap-2 text-xs text-muted-foreground">
                            <Loader2 className="h-3.5 w-3.5 shrink-0 animate-spin" aria-hidden="true" />
                            {t('servers.create.loadingTemplate', { name: selectedTemplateName })}
                        </output>
                    )}
                    {selectedTemplateName && templateError && (
                        <p className="flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/10 p-2.5 text-xs text-destructive-bright" role="alert">
                            <ShieldAlert className="mt-px h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                            {templateError}
                        </p>
                    )}
                </StepSection>
            )}
        </div>
    );
}

/** A template option's raw value: its `value`, or the option itself when it is a plain string. */
function rawOptionValue(opt: any) {
    if (opt.value !== undefined) return opt.value;
    return typeof opt === 'string' ? opt : "";
}

/** The input for one template variable: a select for its options or the game's releases, a checkbox for booleans, a text or number field otherwise. */
function TemplateVariableInput({ fieldId, varKey, variable, currentVal, stringifiedVal, setConfigData, versions, invalid, describedBy, t }: any) {
    if (Array.isArray(versions) && versions.length > 0) {
        // "latest" stays offered when the template starts there; a value typed before (or saved
        // by an older form) that is not a release is kept so it is not silently replaced
        const choices: string[] = [];
        if (stringifiedVal === 'latest' || String(variable.value ?? '') === 'latest') choices.push('latest');
        if (stringifiedVal && stringifiedVal !== 'latest' && !versions.includes(stringifiedVal)) choices.push(stringifiedVal);
        choices.push(...versions);
        return (
            <Select
                value={stringifiedVal || undefined}
                // an integer variable (FTB's Java version) keeps its type
                onValueChange={(val) => setConfigData((prev: any) => ({ ...prev, [varKey]: variable.type === 'integer' ? Number(val) : val }))}
            >
                <SelectTrigger id={fieldId} aria-invalid={invalid || undefined} aria-describedby={describedBy} className={cn(invalid && 'border-destructive')}>
                    <SelectValue placeholder={t('servers.create.versionPlaceholder')} />
                </SelectTrigger>
                <SelectContent className="z-[100] max-h-60">
                    {choices.map((v) => (
                        <SelectItem key={v} value={v} className={cn(v !== 'latest' && 'font-mono tabular-nums')}>
                            {v === 'latest' ? t('servers.create.versionLatest') : v}
                        </SelectItem>
                    ))}
                </SelectContent>
            </Select>
        );
    }
    if (Array.isArray(variable.options) && variable.options.length > 0) {
        return (
            <Select
                value={stringifiedVal === "" ? "_EMPTY_VALUE_" : stringifiedVal}
                onValueChange={(val) => setConfigData((prev: any) => ({ ...prev, [varKey]: val === "_EMPTY_VALUE_" ? "" : val }))}
            >
                <SelectTrigger id={fieldId} aria-invalid={invalid || undefined} aria-describedby={describedBy}>
                    <SelectValue />
                </SelectTrigger>
                <SelectContent className="z-[100] max-h-60">
                    {keyedByContent(variable.options as any[], (opt) => (opt ? String(rawOptionValue(opt)) : '')).map(({ key: optKey, item: opt }) => {
                        if (!opt) return null;
                        const rawOptVal = rawOptionValue(opt);
                        const optVal = rawOptVal === "" ? "_EMPTY_VALUE_" : String(rawOptVal);
                        return (
                            <SelectItem key={optKey} value={optVal}>
                                <SafeValue v={opt.display || (rawOptVal === "" ? t('servers.create.optionNone') : rawOptVal)} />
                            </SelectItem>
                        );
                    })}
                </SelectContent>
            </Select>
        );
    }
    if (variable.type === 'boolean') {
        return (
            <label
                htmlFor={fieldId}
                className="flex h-10 cursor-pointer items-center gap-2.5 rounded-lg border border-input bg-background/50 px-3 text-sm transition-colors duration-200 hover:bg-accent/30"
            >
                <Checkbox
                    id={fieldId}
                    checked={!!currentVal}
                    onCheckedChange={() => setConfigData((prev: any) => ({ ...prev, [varKey]: !currentVal }))}
                />
                <span className="select-none">{t('servers.create.enable')}</span>
            </label>
        );
    }
    return (
        <Input
            id={fieldId}
            type={variable.type === 'integer' ? 'number' : 'text'}
            inputMode={variable.type === 'integer' ? 'numeric' : undefined}
            min={variable.type === 'integer' && typeof variable.min === 'number' ? variable.min : undefined}
            max={variable.type === 'integer' && typeof variable.max === 'number' ? variable.max : undefined}
            value={stringifiedVal}
            aria-invalid={invalid || undefined}
            aria-describedby={describedBy}
            // an emptied number field stays empty while the user types, instead of jumping to 0
            onChange={(e) => setConfigData((prev: any) => ({ ...prev, [varKey]: variable.type === 'integer' && e.target.value !== '' ? Number(e.target.value) : e.target.value }))}
            className={cn(variable.type === 'integer' && 'font-mono tabular-nums', invalid && 'border-destructive')}
        />
    );
}

function Step3Configuration({
    templateDetails, templateError, configData, setConfigData, fieldErrors,
    cpuLimit, setCpuLimit, memoryLimit, setMemoryLimit, diskLimit, setDiskLimit, ioLimit, setIoLimit,
    extraPorts, setExtraPorts,
    setCurrentStep, t
}: any) {
    const versionTarget = templateDetails ? templateVersionTarget(templateDetails, configData) : null;
    const gameVersions = useGameVersions(versionTarget?.platform);
    const javaVariable = templateDetails ? javaImageVariable(templateDetails) : null;

    // The Java image follows the Minecraft version: newer releases do not start on an older Java.
    // It is set again whenever the version changes; picking another one by hand is still allowed.
    const chosenVersion = versionTarget ? String(configData[versionTarget.variable] ?? '') : '';
    const versionForJava = chosenVersion === 'latest' ? gameVersions[0] ?? '' : chosenVersion;
    const neededJava = versionTarget && javaVariable
        ? minecraftJavaFor(versionForJava) ?? (chosenVersion === 'latest' ? TEMURIN_JAVA_VERSIONS[0] : null)
        : null;
    useEffect(() => {
        if (!neededJava || !javaVariable) return;
        setConfigData((prev: any) => {
            const value = typeof prev[javaVariable] === 'number' ? Number(neededJava) : neededJava;
            return String(prev[javaVariable] ?? '') === neededJava ? prev : { ...prev, [javaVariable]: value };
        });
    }, [neededJava, javaVariable, setConfigData]);

    if (templateError) {
        return (
            <div className="rounded-xl border border-dashed border-destructive/40 px-6 py-10 text-center">
                <ShieldAlert className="mx-auto mb-3 h-6 w-6 text-destructive-bright" aria-hidden="true" />
                <p className="text-base font-semibold">{t('servers.create.templateErrorTitle')}</p>
                <p className="mt-1 text-sm text-muted-foreground">{templateError}</p>
                <Button variant="outline" size="sm" className="mt-4" onClick={() => setCurrentStep(2)}>
                    <ChevronLeft aria-hidden="true" />
                    {t('common.back')}
                </Button>
            </div>
        );
    }

    if (!templateDetails) {
        return (
            <output className="flex flex-col items-center justify-center gap-3 px-6 py-16 text-center">
                <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" aria-hidden="true" />
                <div className="space-y-1">
                    <p className="text-base font-semibold">{t('servers.create.analyzingTitle')}</p>
                    <p className="text-sm text-muted-foreground">{t('servers.create.analyzingDescription')}</p>
                </div>
            </output>
        );
    }

    const variables = (templateDetails.data || templateDetails.variables || templateDetails.Variables || {}) as Record<string, any>;
    const entries = Object.entries(variables).filter(([key, v]) => v && !v.internal && !isResourceKey(key));

    return (
        <div className="grid w-full gap-6">
            <StepSection title={t('servers.create.resourcesTitle')} hint={t('servers.create.resourcesHint')}>
                <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                    <div className="grid min-w-0 gap-2">
                        <Label htmlFor="limit-cpu">{t('servers.create.cpuLabel')} <span className="font-normal text-muted-foreground">(%)</span></Label>
                        <Input id="limit-cpu" type="number" inputMode="numeric" value={cpuLimit} onChange={(e) => setCpuLimit(e.target.value === '' ? '' : Number(e.target.value))} className="font-mono tabular-nums" />
                    </div>
                    <div className="grid min-w-0 gap-2">
                        <Label htmlFor="limit-memory">{t('servers.create.memoryLabel')} <span className="font-normal text-muted-foreground">(MB)</span></Label>
                        <Input id="limit-memory" type="number" inputMode="numeric" value={memoryLimit} onChange={(e) => setMemoryLimit(e.target.value === '' ? '' : Number(e.target.value))} className="font-mono tabular-nums" />
                    </div>
                    <div className="grid min-w-0 gap-2">
                        <Label htmlFor="limit-disk">{t('servers.create.diskLabel')} <span className="font-normal text-muted-foreground">(MB)</span></Label>
                        <Input id="limit-disk" type="number" inputMode="numeric" value={diskLimit} onChange={(e) => setDiskLimit(e.target.value === '' ? '' : Number(e.target.value))} className="font-mono tabular-nums" />
                    </div>
                    <div className="grid min-w-0 gap-2">
                        <Label htmlFor="limit-io">{t('servers.create.ioLabel')} <span className="font-normal text-muted-foreground">(10–1000)</span></Label>
                        <Input id="limit-io" type="number" inputMode="numeric" min={10} max={1000} value={ioLimit} onChange={(e) => setIoLimit(e.target.value === '' ? '' : Number(e.target.value))} className="font-mono tabular-nums" />
                    </div>
                </div>
            </StepSection>

            {entries.length > 0 && (
                <StepSection title={t('servers.create.variablesTitle')}>
                    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
                        {entries.map(([key, variable]: [string, any], idx: number) => {
                            if (!variable || typeof variable !== 'object') return null;
                            const currentVal = configData[key] !== undefined ? configData[key] : "";
                            const stringifiedVal = (typeof currentVal === 'object' && currentVal !== null)
                                ? JSON.stringify(currentVal)
                                : String(currentVal ?? "");

                            const label = variable.display || key;
                            const isWide = (label.toLowerCase().includes('motd') || label.toLowerCase().includes('arguments') || label.toLowerCase().includes('command'));
                            const fieldId = variableFieldId(key);
                            const description = plainDescription(variable.desc);
                            const error: FieldError | undefined = fieldErrors?.[key];
                            const descriptionId = description ? `${fieldId}-desc` : undefined;
                            const errorId = error ? `${fieldId}-error` : undefined;

                            return (
                                <div key={key + idx} className={cn('grid min-w-0 content-start gap-2', isWide && 'sm:col-span-2')}>
                                    <Label htmlFor={fieldId} className="truncate" title={label}>
                                        <SafeValue v={label} />
                                        {variable.required && <RequiredMark />}
                                    </Label>
                                    <TemplateVariableInput
                                        fieldId={fieldId}
                                        varKey={key}
                                        variable={variable}
                                        currentVal={currentVal}
                                        stringifiedVal={stringifiedVal}
                                        setConfigData={setConfigData}
                                        versions={
                                            versionTarget?.variable === key ? gameVersions
                                                : javaVariable === key && !(Array.isArray(variable.options) && variable.options.length > 0) ? TEMURIN_JAVA_VERSIONS
                                                    : undefined
                                        }
                                        invalid={!!error}
                                        describedBy={[errorId, descriptionId].filter(Boolean).join(' ') || undefined}
                                        t={t}
                                    />
                                    {error && (
                                        <p id={errorId} className="text-pretty text-xs font-medium text-destructive-bright">
                                            {t(`servers.create.fieldErrors.${error.code}`, { limit: error.limit ?? '' })}
                                        </p>
                                    )}
                                    {javaVariable === key && neededJava && stringifiedVal !== neededJava && (
                                        <p className="text-pretty text-xs font-medium text-warning">
                                            {t('servers.create.javaMismatch', { version: versionForJava || chosenVersion, java: neededJava })}
                                        </p>
                                    )}
                                    {description && (
                                        <p id={descriptionId} className="text-pretty text-xs leading-relaxed text-muted-foreground [overflow-wrap:anywhere]">{description}</p>
                                    )}
                                </div>
                            );
                        })}
                    </div>
                </StepSection>
            )}

            <StepSection
                title={t('servers.create.portsTitle')}
                hint={<>{t('servers.create.mainPort')} <span className="font-mono text-foreground">{configData['port'] || '—'}</span></>}
            >
                {extraPorts.length > 0 && (
                    <div className="grid gap-2 sm:grid-cols-2">
                        {extraPorts.map((p: string, idx: number) => (
                            <div key={idx} className="flex items-center gap-2">
                                <Input
                                    type="number"
                                    inputMode="numeric"
                                    value={p}
                                    placeholder={t('servers.create.portPlaceholder')}
                                    aria-label={t('servers.create.extraPortLabel', { index: idx + 1 })}
                                    onChange={(e) => {
                                        const next = [...extraPorts];
                                        next[idx] = e.target.value;
                                        setExtraPorts(next);
                                    }}
                                    className="font-mono tabular-nums"
                                />
                                <Button
                                    type="button"
                                    variant="ghost"
                                    size="icon"
                                    className="shrink-0 text-muted-foreground hover:bg-destructive/10 hover:text-destructive-bright"
                                    onClick={() => setExtraPorts(extraPorts.filter((_: string, i: number) => i !== idx))}
                                    aria-label={t('servers.create.removePort')}
                                >
                                    <X aria-hidden="true" />
                                </Button>
                            </div>
                        ))}
                    </div>
                )}
                <Button type="button" variant="outline" size="sm" className="justify-self-start" onClick={() => setExtraPorts([...extraPorts, ''])}>
                    <Plus aria-hidden="true" />
                    {t('servers.create.addPort')}
                </Button>
            </StepSection>
        </div>
    );
}

export function CreateServerStepper({ onComplete, forcedParentId, forcedNodeId, initialRepo, initialTemplate }: Readonly<{
    onComplete: () => void,
    forcedParentId?: string,
    forcedNodeId?: string,
    /** opened from a template's "Use" button: that repo and template start selected */
    initialRepo?: number,
    initialTemplate?: string,
}>) {
    const { t } = useTranslations();

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

    const [selectedRepo, setSelectedRepo] = useState<number | null>(initialTemplate ? initialRepo ?? 0 : null);
    // picked once the chosen repo's templates arrive
    const pendingTemplate = useRef(initialTemplate ?? '');
    const [templateList, setTemplateList] = useState<any[]>([]);
    const [selectedTemplateName, setSelectedTemplateName] = useState('');
    const [loadingTemplates, setLoadingTemplates] = useState(false);

    // Ensure the "Local" repo is always selectable even if the API omits it.
    const repoList = useMemo(() => {
        const list = [...(repos || [])];
        if (!list.some((r: any) => r.id === 0)) {
            list.push({ id: 0, name: 'Local', url: '', isLocal: true });
        }
        return list;
    }, [repos]);

    const [templateDetails, setTemplateDetails] = useState<any>(null);
    const [templateError, setTemplateError] = useState<string | null>(null);
    const [configData, setConfigData] = useState<Record<string, any>>({});
    const [fieldErrors, setFieldErrors] = useState<Record<string, FieldError>>({});

    // once a field was marked, its error goes away as soon as it is fixed
    useEffect(() => {
        setFieldErrors((prev) => {
            if (Object.keys(prev).length === 0 || !templateDetails) return prev;
            const now = validateTemplateValues(templateDetails.data || {}, configData);
            const still: Record<string, FieldError> = {};
            for (const key of Object.keys(prev)) if (now[key]) still[key] = now[key];
            return still;
        });
    }, [configData, templateDetails]);
    
    const [cpuLimit, setCpuLimit] = useState<number | ''>(100);
    const [memoryLimit, setMemoryLimit] = useState<number | ''>(1024);
    const [diskLimit, setDiskLimit] = useState<number | ''>(10240);
    const [ioLimit, setIoLimit] = useState<number | ''>(500);
    const [extraPorts, setExtraPorts] = useState<string[]>([]);

    useEffect(() => {
        if (repoList.length === 1 && selectedRepo === null) {
            setSelectedRepo(repoList[0].id);
        }
    }, [repoList]);

    useEffect(() => {
        if (selectedRepo === null) return;
        // a repo picked after this one makes this answer out of date
        let cancelled = false;
        setLoadingTemplates(true);
        setSelectedTemplateName('');
        setTemplateDetails(null);
        setTemplateError(null);
        getTemplatesForRepo(selectedRepo).then(list => {
            if (cancelled) return;
            setTemplateList(list);
            setLoadingTemplates(false);
            const wanted = pendingTemplate.current;
            pendingTemplate.current = '';
            if (wanted && list.some((tpl: any) => tpl.name === wanted)) {
                setSelectedTemplateName(wanted);
            }
        }).catch(() => {
            if (cancelled) return;
            setTemplateList([]);
            setLoadingTemplates(false);
        });
        return () => {
            cancelled = true;
        };
    }, [selectedRepo]);

    const setResourceLimit = (key: string, value: number) => {
        if (key === 'cpu') setCpuLimit(value);
        else if (key === 'memory') setMemoryLimit(value);
        else if (key === 'io') setIoLimit(value);
        else setDiskLimit(value);
    };

    const parseTemplateVariables = (details: any) => {
        const initial: Record<string, any> = {};
        const rawVars = details.data || details.variables || details.Variables || {};
        const variables = (typeof rawVars === 'object' && rawVars !== null) ? rawVars : {};

        Object.entries(variables).forEach(([key, val]: [string, any]) => {
            if (val && typeof val === 'object') {
                const rawValue = templateVariableValue(val);
                if (isResourceKey(key)) {
                    setResourceLimit(key, rawValue ? Number(rawValue) : RESOURCE_DEFAULTS[key]);
                } else {
                    initial[key] = toConfigValue(rawValue);
                }
            } else if (val !== undefined && val !== null) {
                if (isResourceKey(key)) setResourceLimit(key, Number(val));
                else initial[key] = String(val);
            }
        });
        setConfigData(initial);
    };

    useEffect(() => {
        if (!selectedTemplateName || selectedRepo === null) return;
        // switching template quickly must not let the earlier one's answer win
        let cancelled = false;
        setTemplateError(null);
        setTemplateDetails(null);
        setExtraPorts([]);
        getTemplateDetails(selectedRepo, selectedTemplateName).then(details => {
            if (cancelled) return;
            if (!details) {
                setTemplateError(t('servers.create.templateLoadFailed'));
                return;
            }
            setTemplateDetails(details);
            parseTemplateVariables(details);
        }).catch(err => {
            if (cancelled) return;
            setTemplateError(err.message || t('servers.create.templateFetchFailed'));
        });
        return () => {
            cancelled = true;
        };
    }, [selectedTemplateName, selectedRepo]);

    const handleNext = () => {
        if (currentStep === 1) {
            if (!name || (!selectedNode && !forcedParentId)) {
                sileo.error({ title: t('common.error'), description: t('servers.create.requiredFields') });
                return;
            }
            setCurrentStep(2);
        } else if (currentStep === 2) {
            if (!selectedTemplateName) {
                sileo.error({ title: t('common.error'), description: t('servers.create.selectTemplate') });
                return;
            }
            if (templateError) {
                sileo.error({ title: t('common.error'), description: templateError });
                return;
            }
            if (!templateDetails) {
                sileo.info({ title: t('servers.create.templateLoadingTitle'), description: t('servers.create.templateLoadingDescription') });
                return;
            }
            setCurrentStep(3);
        }
    };

    const handleCreate = async () => {
        if (!templateDetails) return;
        const errors = validateTemplateValues(templateDetails.data || {}, configData);
        setFieldErrors(errors);
        const firstInvalid = Object.keys(errors)[0];
        if (firstInvalid) {
            sileo.error({ title: t('servers.create.fixFieldsTitle'), description: t('servers.create.fixFieldsDescription') });
            document.getElementById(variableFieldId(firstInvalid))?.focus();
            return;
        }
        setLoading(true);
        try {
            const identifier = serverIdFromName(name);

            // each variable keeps the template's definition (name, description, type, options,
            // whether the user may edit it) with the chosen value: sending only the value left
            // the server's Settings with nothing the user could change
            const templateVars: Record<string, any> = (templateDetails.data && typeof templateDetails.data === 'object') ? templateDetails.data : {};
            const withDefinition = (k: string, value: any) => {
                const definition = templateVars[k];
                return (definition && typeof definition === 'object') ? { ...definition, value } : { value };
            };

            const vars: Record<string, any> = {};
            Object.entries(configData).forEach(([k, v]) => {
                vars[k] = withDefinition(k, v);
            });

            vars['cpu'] = withDefinition('cpu', cpuLimit === '' ? 100 : cpuLimit);
            vars['memory'] = withDefinition('memory', memoryLimit === '' ? 1024 : memoryLimit);
            vars['disk'] = withDefinition('disk', diskLimit === '' ? 10240 : diskLimit);
            vars['io'] = withDefinition('io', ioLimit === '' ? 500 : ioLimit);

            // Inject extra ports as port2, port3, ... so the daemon binds them
            extraPorts.map(Number).filter(p => p > 0).forEach((p, i) => {
                vars[`port${i + 2}`] = { value: p };
            });

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

            let resolvedNode = 0;
            if (selectedNode !== '') resolvedNode = Number(selectedNode);
            else if (forcedNodeId) resolvedNode = Number(forcedNodeId);

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

            sileo.success({ title: t('common.success'), description: t('servers.create.success', { name }) });
            onComplete();
        } catch (e: any) {
            sileo.error({ title: t('servers.create.createFailed'), description: e?.message || t('servers.create.unknownError') });
        } finally {
            setLoading(false);
        }
    };

    const steps: { id: Step; label: string }[] = [
        { id: 1, label: t('servers.create.steps.environment') },
        { id: 2, label: t('servers.create.steps.template') },
        { id: 3, label: t('servers.create.steps.config') },
    ];
    const waitingForTemplate = currentStep === 2 && !!selectedTemplateName && !templateDetails && !templateError;

    return (
        <ErrorBoundary t={t}>
            {/* Fills its dialog: step indicator and footer stay put, only the step body scrolls. */}
            <div className="flex min-h-0 w-full max-w-full flex-1 flex-col gap-5">
                <ol className="flex w-full min-w-0 items-center gap-2 sm:gap-3" aria-label={t('servers.create.stepsLabel')}>
                    {steps.map((step) => {
                        const done = currentStep > step.id;
                        const current = currentStep === step.id;
                        return (
                            <React.Fragment key={step.id}>
                                <li
                                    className={cn('flex items-center gap-2', current ? 'min-w-0' : 'shrink-0')}
                                    aria-current={current ? 'step' : undefined}
                                >
                                    <span
                                        className={cn(
                                            'grid h-7 w-7 shrink-0 place-items-center rounded-full border text-xs font-semibold tabular-nums transition-colors duration-200',
                                            current && 'border-primary bg-primary text-primary-foreground',
                                            done && 'border-primary/40 bg-primary/15 text-primary-bright',
                                            !current && !done && 'border-border text-muted-foreground'
                                        )}
                                    >
                                        {done ? <Check className="h-3.5 w-3.5" aria-hidden="true" /> : step.id}
                                    </span>
                                    <span
                                        className={cn(
                                            'truncate text-sm font-medium',
                                            current ? 'text-foreground' : 'hidden text-muted-foreground sm:inline'
                                        )}
                                    >
                                        {step.label}
                                    </span>
                                </li>
                                {step.id < 3 && (
                                    <li
                                        aria-hidden="true"
                                        className={cn('h-px min-w-4 flex-1 transition-colors duration-200', done ? 'bg-primary/50' : 'bg-border')}
                                    />
                                )}
                            </React.Fragment>
                        );
                    })}
                </ol>

                <div className="-mx-1 min-h-0 flex-1 overflow-y-auto px-1 pb-1">
                    <div key={currentStep} className="animate-in fade-in duration-200 motion-reduce:animate-none">
                        {currentStep === 1 && (
                            <Step1Environment
                                name={name} setName={setName}
                                selectedNode={selectedNode} setSelectedNode={setSelectedNode}
                                selectedEnvironment={selectedEnvironment} setSelectedEnvironment={setSelectedEnvironment}
                                selectedUsers={selectedUsers} setSelectedUsers={setSelectedUsers}
                                forcedParentId={forcedParentId} nodes={nodes} users={users} t={t}
                            />
                        )}
                        {currentStep === 2 && (
                            <Step2Template
                                repos={repoList} selectedRepo={selectedRepo} setSelectedRepo={setSelectedRepo}
                                templateList={templateList} selectedTemplateName={selectedTemplateName} setSelectedTemplateName={setSelectedTemplateName}
                                loadingTemplates={loadingTemplates} templateDetails={templateDetails} templateError={templateError} t={t}
                            />
                        )}
                        {currentStep === 3 && (
                            <Step3Configuration
                                fieldErrors={fieldErrors}
                                templateDetails={templateDetails} templateError={templateError}
                                configData={configData} setConfigData={setConfigData}
                                cpuLimit={cpuLimit} setCpuLimit={setCpuLimit}
                                memoryLimit={memoryLimit} setMemoryLimit={setMemoryLimit}
                                diskLimit={diskLimit} setDiskLimit={setDiskLimit}
                                ioLimit={ioLimit} setIoLimit={setIoLimit}
                                extraPorts={extraPorts} setExtraPorts={setExtraPorts}
                                setCurrentStep={setCurrentStep} t={t}
                            />
                        )}
                    </div>
                </div>

                <div className="flex shrink-0 items-center justify-between gap-3 border-t border-border/50 pt-4">
                    <Button
                        variant="ghost"
                        onClick={() => setCurrentStep((prev) => (prev - 1) as Step)}
                        disabled={currentStep === 1 || loading}
                        className={cn('shrink-0', currentStep === 1 && 'invisible')}
                    >
                        <ChevronLeft aria-hidden="true" />
                        {t('common.back')}
                    </Button>
                    <span className="whitespace-nowrap text-xs tabular-nums text-muted-foreground sm:hidden" aria-hidden="true">
                        {currentStep} / 3
                    </span>
                    {currentStep < 3 ? (
                        <Button
                            onClick={handleNext}
                            disabled={waitingForTemplate}
                            className="shrink-0"
                        >
                            {waitingForTemplate && <Loader2 className="animate-spin" aria-hidden="true" />}
                            {t('common.next')}
                            <ChevronRight aria-hidden="true" />
                        </Button>
                    ) : (
                        <Button onClick={handleCreate} disabled={loading} className="shrink-0">
                            {loading
                                ? <Loader2 className="animate-spin" aria-hidden="true" />
                                : <Check aria-hidden="true" />}
                            {t('servers.create.submit')}
                        </Button>
                    )}
                </div>
            </div>
        </ErrorBoundary>
    );
}
