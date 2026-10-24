'use client';
import { PageHeader } from '@/components/page-header';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { UserAvatar } from '@/components/user-avatar';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useAuth } from '@/contexts/providers';
import { useProfile } from '@/hooks/use-profile';
import { sileo } from "@/lib/toast";
import { cn } from '@/lib/utils';
import { Loader2, Download, Trash2, Plus, Copy, Check, KeyRound, TriangleAlert, RefreshCw, ExternalLink } from 'lucide-react';
import { useState, useEffect, useRef, type ReactNode } from 'react';
import { useTranslations } from '@/contexts/translations-context';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { EmptyState } from '@/components/empty-state';

/** Copy-to-clipboard icon button with a short "copied" state. */
function CopyButton({ value, label, copiedLabel, className }: { value: string; label: string; copiedLabel: string; className?: string }) {
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  useEffect(() => () => clearTimeout(timer.current), []);
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      className={cn('h-9 w-9 shrink-0 text-muted-foreground hover:text-foreground', className)}
      aria-label={copied ? copiedLabel : label}
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(value);
          setCopied(true);
          clearTimeout(timer.current);
          timer.current = setTimeout(() => setCopied(false), 1600);
        } catch {
          /* clipboard unavailable */
        }
      }}
    >
      {copied ? <Check className="text-success" aria-hidden="true" /> : <Copy aria-hidden="true" />}
    </Button>
  );
}

/** Title + description on the left, controls on the right; stacked on narrow screens. */
function ProfileSection({ id, title, description, children }: { id: string; title: string; description?: ReactNode; children: ReactNode }) {
  return (
    <section aria-labelledby={id} className="grid gap-5 p-5 md:p-6 lg:grid-cols-[minmax(0,16rem)_minmax(0,1fr)] lg:gap-10">
      <div className="space-y-1.5">
        <h2 id={id} className="text-sm font-semibold text-foreground">{title}</h2>
        {description && <div className="text-pretty text-sm text-muted-foreground">{description}</div>}
      </div>
      <div className="min-w-0">{children}</div>
    </section>
  );
}

function Field({ id, label, children }: { id: string; label: string; children: ReactNode }) {
  return (
    <div className="space-y-2">
      <Label htmlFor={id} className="text-sm font-medium">{label}</Label>
      {children}
    </div>
  );
}

export default function ProfileSettingsPage() {
  const { user, fetchSelf } = useAuth();
  const { otpEnabled, oauthClients, loading, updateDetails, startOtpEnroll, validateOtpEnroll, disableOtp, regenerateRecoveryCodes, createOauthClient, deleteOauthClient } = useProfile();
  const { t, language, setLanguage } = useTranslations();

  const [isMounted, setIsMounted] = useState(false);

  // Form states
  const [detailsForm, setDetailsForm] = useState({ username: '', email: '', password: '' });
  const [passwordForm, setPasswordForm] = useState({ oldPassword: '', newPassword: '', confirmPassword: '' });
  const [oauthForm, setOauthForm] = useState({ name: '', description: '' });
  const [oauthDialog, setOauthDialog] = useState(false);
  const [createdSecret, setCreatedSecret] = useState<{ name: string; clientId?: string; secret: string } | null>(null);
  const [pendingClientDelete, setPendingClientDelete] = useState<string | null>(null);

  // 2FA state
  const [otpDialog, setOtpDialog] = useState(false);
  const [otpStep, setOtpStep] = useState<'enroll' | 'confirm' | 'recovery'>('enroll');
  const [otpData, setOtpData] = useState<{ secret?: string; img?: string; recoveryCodes?: string[] }>({});
  const [otpToken, setOtpToken] = useState('');
  // Code prompt used to disable 2FA or regenerate the recovery codes.
  const [codePrompt, setCodePrompt] = useState<null | 'disable' | 'regenerate'>(null);
  const [promptToken, setPromptToken] = useState('');

  useEffect(() => {
    setIsMounted(true);
    if (user) {
      setDetailsForm({ username: user.username, email: user.email, password: '' });
    }
  }, [user]);

  if (!isMounted) {
    return null;
  }

  const handleUpdateDetails = async () => {
    try {
      await updateDetails({ username: detailsForm.username, email: detailsForm.email, password: detailsForm.password });
      await fetchSelf();
      sileo.success({ title: t('common.success'), description: t('profileSettings.messages.infoChanged') });
      setDetailsForm(prev => ({ ...prev, password: '' }));
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message });
    }
  };

  const handleChangePassword = async () => {
    if (passwordForm.newPassword !== passwordForm.confirmPassword) {
      sileo.error({ title: t('common.error'), description: t('profileSettings.messages.passwordMismatch') });
      return;
    }
    try {
      await updateDetails({ password: passwordForm.oldPassword, newPassword: passwordForm.newPassword });
      sileo.success({ title: t('common.success'), description: t('profileSettings.messages.passwordChanged') });
      setPasswordForm({ oldPassword: '', newPassword: '', confirmPassword: '' });
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message });
    }
  };

  // 2FA Handlers
  const handleStartEnroll = async () => {
    try {
      const data = await startOtpEnroll();
      setOtpData(data);
      setOtpStep('enroll');
      setOtpToken('');
      setOtpDialog(true);
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message });
    }
  };

  const handleConfirmEnroll = async () => {
    try {
      const res = await validateOtpEnroll(otpToken);
      setOtpData(prev => ({ ...prev, recoveryCodes: res.recoveryCodes }));
      setOtpStep('recovery');
      sileo.success({ title: t('common.success'), description: t('profileSettings.messages.otpEnabled') });
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message });
    }
  };

  const handleDisableOtp = async (token: string) => {
    if (!token) return;
    try {
      await disableOtp(token);
      setCodePrompt(null);
      sileo.success({ title: t('common.success'), description: t('profileSettings.messages.otpDisabled') });
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message });
    }
  };

  const handleRegenerateRecovery = async (token: string) => {
    if (!token) return;
    try {
      const res = await regenerateRecoveryCodes(token);
      setCodePrompt(null);
      setOtpData({ recoveryCodes: res.recoveryCodes });
      setOtpStep('recovery');
      setOtpDialog(true);
      sileo.success({ title: t('common.success'), description: t('profileSettings.messages.recoveryCodesRegenerated') });
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message });
    }
  };

  const openCodePrompt = (kind: 'disable' | 'regenerate') => {
    setPromptToken('');
    setCodePrompt(kind);
  };

  const downloadRecoveryCodes = () => {
    if (!otpData.recoveryCodes) return;
    const blob = new Blob([otpData.recoveryCodes.join('\n')], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'recovery-codes.txt';
    a.click();
    URL.revokeObjectURL(url);
  };

  // OAuth Handlers
  const handleCreateClient = async () => {
    try {
      const client = await createOauthClient(oauthForm);
      sileo.success({ title: t('common.success'), description: t('profileSettings.oauth.created', { name: client.name }) });
      setOauthForm({ name: '', description: '' });
      // Show the secret once, in the same dialog.
      if (client.clientSecret) {
        setCreatedSecret({ name: client.name, clientId: client.clientId, secret: client.clientSecret });
      } else {
        setOauthDialog(false);
      }
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message });
    }
  };

  const handleDeleteClient = async (id: string) => {
    try {
      await deleteOauthClient(id);
      sileo.success({ title: t('common.success'), description: t('profileSettings.oauth.deleted') });
    } catch (e: any) {
      sileo.error({ title: t('common.error'), description: e.message });
    }
  };

  const closeOauthDialog = (open: boolean) => {
    if (open) return;
    setOauthDialog(false);
    setCreatedSecret(null);
  };

  const displayName = user?.username || user?.name || user?.email || '';
  const initials = displayName.trim().slice(0, 2).toUpperCase() || '?';
  const otpInputClass = 'h-12 text-center font-mono text-xl tracking-[0.4em] tabular-nums md:text-xl';

  return (
    <div className="stagger flex flex-col gap-6 pb-12">
      <PageHeader title={t('profileSettings.title')} description={t('profileSettings.description')} />

      <Card className="divide-y divide-border/50">
        {/* Account details */}
        <ProfileSection id="profile-account" title={t('profileSettings.accountDetails.title')} description={t('profileSettings.accountDetails.description')}>
          <form
            className="space-y-5"
            onSubmit={(e) => {
              e.preventDefault();
              void handleUpdateDetails();
            }}
          >
            <div className="flex items-center gap-3">
              <UserAvatar
                identity={user?.email || displayName}
                src={user?.avatar}
                fallback={initials}
                className="h-12 w-12 ring-1 ring-border/60"
                fallbackClassName="bg-primary/15 text-primary-bright"
              />
              <div className="min-w-0">
                <p className="truncate text-sm font-semibold">{user?.username}</p>
                <p className="truncate text-sm text-muted-foreground">{user?.email}</p>
              </div>
            </div>
            <div className="grid gap-5 sm:grid-cols-2">
              <Field id="username" label={t('profileSettings.accountDetails.username')}>
                <Input id="username" autoComplete="username" value={detailsForm.username} onChange={e => setDetailsForm(prev => ({ ...prev, username: e.target.value }))} />
              </Field>
              <Field id="email" label={t('profileSettings.accountDetails.email')}>
                <Input id="email" type="email" autoComplete="email" value={detailsForm.email} onChange={e => setDetailsForm(prev => ({ ...prev, email: e.target.value }))} />
              </Field>
              <div className="space-y-2 sm:col-span-2">
                <Label htmlFor="confirm-password-details" className="text-sm font-medium">{t('profileSettings.accountDetails.confirmPassword')}</Label>
                <Input
                  id="confirm-password-details"
                  type="password"
                  autoComplete="current-password"
                  value={detailsForm.password}
                  onChange={e => setDetailsForm(prev => ({ ...prev, password: e.target.value }))}
                  className="sm:max-w-[calc(50%-0.625rem)]"
                />
                <p className="text-xs text-muted-foreground">{t('profileSettings.accountDetails.confirmPasswordHint')}</p>
              </div>
            </div>
            <div className="flex justify-end">
              <Button type="submit" disabled={loading}>
                {loading && <Loader2 className="animate-spin" aria-hidden="true" />}
                {t('profileSettings.accountDetails.save')}
              </Button>
            </div>
          </form>
        </ProfileSection>

        {/* Password */}
        <ProfileSection id="profile-password" title={t('profileSettings.changePassword.title')} description={t('profileSettings.changePassword.description')}>
          <form
            className="space-y-5"
            onSubmit={(e) => {
              e.preventDefault();
              void handleChangePassword();
            }}
          >
            <div className="grid gap-5 sm:grid-cols-2">
              <div className="sm:col-span-2 sm:max-w-[calc(50%-0.625rem)]">
                <Field id="old-password" label={t('profileSettings.changePassword.oldPassword')}>
                  <Input id="old-password" type="password" autoComplete="current-password" value={passwordForm.oldPassword} onChange={e => setPasswordForm(prev => ({ ...prev, oldPassword: e.target.value }))} />
                </Field>
              </div>
              <Field id="new-password" label={t('profileSettings.changePassword.newPassword')}>
                <Input id="new-password" type="password" autoComplete="new-password" value={passwordForm.newPassword} onChange={e => setPasswordForm(prev => ({ ...prev, newPassword: e.target.value }))} />
              </Field>
              <Field id="confirm-password-change" label={t('profileSettings.changePassword.confirmPassword')}>
                <Input id="confirm-password-change" type="password" autoComplete="new-password" value={passwordForm.confirmPassword} onChange={e => setPasswordForm(prev => ({ ...prev, confirmPassword: e.target.value }))} />
              </Field>
            </div>
            <div className="flex justify-end">
              <Button type="submit" disabled={loading}>
                {loading && <Loader2 className="animate-spin" aria-hidden="true" />}
                {t('profileSettings.changePassword.save')}
              </Button>
            </div>
          </form>
        </ProfileSection>

        {/* Two-factor authentication */}
        <ProfileSection id="profile-2fa" title={t('profileSettings.twoFactor.title')} description={t('profileSettings.twoFactor.description')}>
          <div className="flex flex-col gap-4 rounded-xl border border-border/60 bg-background/30 p-4 sm:flex-row sm:items-center sm:justify-between">
            <div className="min-w-0 space-y-1.5">
              <span
                className={cn(
                  'inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium',
                  otpEnabled ? 'bg-success/10 text-success' : 'bg-warning/10 text-warning',
                )}
              >
                <span className={cn('h-1.5 w-1.5 rounded-full', otpEnabled ? 'bg-success' : 'bg-warning')} aria-hidden="true" />
                {otpEnabled ? t('profileSettings.twoFactor.statusEnabled') : t('profileSettings.twoFactor.statusDisabled')}
              </span>
              <p className="text-pretty text-sm text-muted-foreground">
                {otpEnabled ? t('profileSettings.twoFactor.enabledHint') : t('profileSettings.twoFactor.disabledHint')}
              </p>
            </div>
            <div className="flex shrink-0 flex-wrap gap-2">
              {otpEnabled ? (
                <>
                  <Button variant="outline" size="sm" onClick={() => openCodePrompt('regenerate')}>
                    <RefreshCw aria-hidden="true" />
                    {t('profileSettings.twoFactor.regenerateCodes')}
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    className="text-destructive-bright hover:border-destructive/50 hover:bg-destructive/10 hover:text-destructive-bright"
                    onClick={() => openCodePrompt('disable')}
                  >
                    {t('profileSettings.twoFactor.disable')}
                  </Button>
                </>
              ) : (
                <Button size="sm" onClick={handleStartEnroll}>
                  <KeyRound aria-hidden="true" />
                  {t('profileSettings.twoFactor.enable')}
                </Button>
              )}
            </div>
          </div>
        </ProfileSection>

        {/* OAuth2 clients */}
        <ProfileSection
          id="profile-oauth"
          title={t('profileSettings.oauth.title')}
          description={
            <>
              <p>{t('profileSettings.oauth.description')}</p>
              <a
                href="/swagger/index.html"
                target="_blank"
                rel="noopener noreferrer"
                className="mt-2 inline-flex items-center gap-1 rounded text-primary-bright transition-colors duration-150 hover:underline focus-ring"
              >
                {t('profileSettings.oauth.apiDocsLink')}
                <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
              </a>
            </>
          }
        >
          {oauthClients.length === 0 ? (
            <EmptyState
              icon={KeyRound}
              size="sm"
              title={t('profileSettings.oauth.emptyTitle')}
              description={t('profileSettings.oauth.noClients')}
              action={
                <Button onClick={() => setOauthDialog(true)}>
                  <Plus aria-hidden="true" />
                  {t('profileSettings.oauth.create')}
                </Button>
              }
            />
          ) : (
            <div className="space-y-3">
              <ul className="divide-y divide-border/50 rounded-xl border border-border/60 bg-background/30">
                {oauthClients.map(client => (
                  <li key={client.clientId} className="flex items-start justify-between gap-3 px-4 py-3">
                    <div className="min-w-0 space-y-0.5">
                      <p className="truncate text-sm font-medium">{client.name}</p>
                      <div className="flex min-w-0 items-center gap-1">
                        <code className="truncate font-mono text-xs text-muted-foreground">{client.clientId}</code>
                        <CopyButton value={client.clientId} label={t('profileSettings.oauth.copyClientId')} copiedLabel={t('common.copied')} className="h-7 w-7 [&_svg]:size-3.5" />
                      </div>
                      {client.description && <p className="text-pretty text-sm text-muted-foreground">{client.description}</p>}
                    </div>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-9 w-9 shrink-0 text-muted-foreground hover:bg-destructive/10 hover:text-destructive-bright"
                      onClick={() => setPendingClientDelete(client.clientId)}
                      aria-label={t('profileSettings.oauth.deleteAria', { name: client.name })}
                    >
                      <Trash2 aria-hidden="true" />
                    </Button>
                  </li>
                ))}
              </ul>
              <Button variant="outline" size="sm" onClick={() => setOauthDialog(true)}>
                <Plus aria-hidden="true" />
                {t('profileSettings.oauth.create')}
              </Button>
            </div>
          )}
        </ProfileSection>

        {/* Preferences */}
        <ProfileSection id="profile-preferences" title={t('profileSettings.general.sectionTitle')} description={t('profileSettings.general.description')}>
          <div className="max-w-sm space-y-2">
            <Label htmlFor="language" className="text-sm font-medium">{t('profileSettings.general.language')}</Label>
            <Select value={language} onValueChange={(value) => setLanguage(value as 'es' | 'en')}>
              <SelectTrigger id="language">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="es">{t('profileSettings.general.language_es')}</SelectItem>
                <SelectItem value="en">{t('profileSettings.general.language_en')}</SelectItem>
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">{t('profileSettings.general.languageAppliesNow')}</p>
          </div>
        </ProfileSection>
      </Card>

      {/* 2FA enrollment / recovery codes */}
      <Dialog open={otpDialog} onOpenChange={setOtpDialog}>
        <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-md">
          <DialogHeader>
            <DialogTitle>
              {otpStep === 'recovery' ? t('profileSettings.twoFactorSetup.recoveryTitle') : t('profileSettings.twoFactorSetup.title')}
            </DialogTitle>
            <DialogDescription>
              {otpStep === 'enroll' && t('profileSettings.twoFactorSetup.qrCodeHint')}
              {otpStep === 'confirm' && t('profileSettings.twoFactorSetup.confirmHint')}
              {otpStep === 'recovery' && t('profileSettings.twoFactorSetup.recoveryHint')}
            </DialogDescription>
          </DialogHeader>

          {otpStep === 'enroll' && (
            <>
              <div className="flex flex-col items-center gap-4">
                {otpData.img && <img src={otpData.img} alt={t('profileSettings.twoFactorSetup.qrAlt')} className="h-48 w-48 rounded-lg border border-border/60" />}
                <div className="w-full space-y-2">
                  <p className="text-sm font-medium">{t('profileSettings.twoFactorSetup.secretCode')}</p>
                  <div className="flex items-center gap-1 rounded-lg border border-border/60 bg-background/30 py-1 pl-3 pr-1">
                    <code className="min-w-0 flex-1 break-all font-mono text-sm">{otpData.secret}</code>
                    {otpData.secret && <CopyButton value={otpData.secret} label={t('common.copy')} copiedLabel={t('common.copied')} />}
                  </div>
                </div>
              </div>
              <DialogFooter>
                <Button onClick={() => setOtpStep('confirm')}>{t('common.next')}</Button>
              </DialogFooter>
            </>
          )}

          {otpStep === 'confirm' && (
            <form
              className="grid gap-4"
              onSubmit={(e) => {
                e.preventDefault();
                if (otpToken.length >= 6 && !loading) void handleConfirmEnroll();
              }}
            >
              <div className="space-y-2">
                <Label htmlFor="otp-enroll-code" className="sr-only">{t('profileSettings.twoFactorSetup.codeLabel')}</Label>
                <Input
                  id="otp-enroll-code"
                  value={otpToken}
                  onChange={e => setOtpToken(e.target.value)}
                  placeholder="000000"
                  maxLength={6}
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  autoFocus
                  className={otpInputClass}
                />
              </div>
              <DialogFooter className="gap-2">
                <Button type="button" variant="outline" onClick={() => setOtpStep('enroll')}>{t('common.back')}</Button>
                <Button type="submit" disabled={otpToken.length < 6 || loading}>
                  {loading && <Loader2 className="animate-spin" aria-hidden="true" />}
                  {t('profileSettings.twoFactorSetup.confirm')}
                </Button>
              </DialogFooter>
            </form>
          )}

          {otpStep === 'recovery' && (
            <>
              <div className="space-y-3">
                <div className="flex items-start gap-2 rounded-lg border border-warning/30 bg-warning/10 px-3 py-2.5 text-sm text-warning">
                  <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
                  <span>{t('profileSettings.twoFactorSetup.recoveryWarning')}</span>
                </div>
                <div className="relative rounded-lg border border-border/60 bg-background/30 p-4 pr-12">
                  <ul className="grid grid-cols-2 gap-x-4 gap-y-1.5 font-mono text-sm tabular-nums">
                    {otpData.recoveryCodes?.map(code => (
                      <li key={code}>{code}</li>
                    ))}
                  </ul>
                  {otpData.recoveryCodes && (
                    <CopyButton
                      value={otpData.recoveryCodes.join('\n')}
                      label={t('profileSettings.twoFactorSetup.copyCodes')}
                      copiedLabel={t('common.copied')}
                      className="absolute right-1.5 top-1.5"
                    />
                  )}
                </div>
              </div>
              <DialogFooter className="gap-2">
                <Button variant="outline" onClick={downloadRecoveryCodes}>
                  <Download aria-hidden="true" />
                  {t('profileSettings.twoFactorSetup.download')}
                </Button>
                <Button onClick={() => setOtpDialog(false)}>{t('profileSettings.twoFactorSetup.done')}</Button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>

      {/* Code prompt: disable 2FA / regenerate recovery codes */}
      <Dialog open={codePrompt !== null} onOpenChange={(open) => !open && setCodePrompt(null)}>
        <DialogContent className="sm:max-w-md">
          <form
            className="grid gap-4"
            onSubmit={(e) => {
              e.preventDefault();
              if (codePrompt === 'disable') void handleDisableOtp(promptToken);
              else if (codePrompt === 'regenerate') void handleRegenerateRecovery(promptToken);
            }}
          >
            <DialogHeader>
              <DialogTitle>
                {codePrompt === 'disable' ? t('profileSettings.twoFactor.disableTitle') : t('profileSettings.twoFactor.regenerateTitle')}
              </DialogTitle>
              <DialogDescription>
                {codePrompt === 'disable' ? t('profileSettings.twoFactor.disableDescription') : t('profileSettings.twoFactor.regenerateDescription')}
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-2">
              <Label htmlFor="otp-prompt-code">{t('profileSettings.twoFactorSetup.confirmHint')}</Label>
              <Input
                id="otp-prompt-code"
                value={promptToken}
                onChange={e => setPromptToken(e.target.value)}
                placeholder="000000"
                inputMode="numeric"
                autoComplete="one-time-code"
                autoFocus
                className={otpInputClass}
              />
            </div>
            <DialogFooter className="gap-2">
              <Button type="button" variant="outline" onClick={() => setCodePrompt(null)}>{t('common.cancel')}</Button>
              <Button
                type="submit"
                disabled={!promptToken}
                className={cn(codePrompt === 'disable' && 'bg-destructive text-destructive-foreground hover:bg-destructive/90')}
              >
                {codePrompt === 'disable' ? t('profileSettings.twoFactor.disable') : t('profileSettings.twoFactor.regenerateCodes')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Create OAuth2 client, then show its secret once */}
      <Dialog open={oauthDialog} onOpenChange={closeOauthDialog}>
        <DialogContent className="sm:max-w-md">
          {createdSecret ? (
            <>
              <DialogHeader>
                <DialogTitle>{t('profileSettings.oauth.secretTitle')}</DialogTitle>
                <DialogDescription>{t('profileSettings.oauth.secretDescription', { name: createdSecret.name })}</DialogDescription>
              </DialogHeader>
              <div className="space-y-3">
                <div className="flex items-start gap-2 rounded-lg border border-warning/30 bg-warning/10 px-3 py-2.5 text-sm text-warning">
                  <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
                  <span>{t('profileSettings.oauth.secretWarning')}</span>
                </div>
                {createdSecret.clientId && (
                  <div className="space-y-1.5">
                    <p className="text-sm font-medium">{t('profileSettings.oauth.clientId')}</p>
                    <div className="flex items-center gap-1 rounded-lg border border-border/60 bg-background/30 py-1 pl-3 pr-1">
                      <code className="min-w-0 flex-1 break-all font-mono text-[13px]">{createdSecret.clientId}</code>
                      <CopyButton value={createdSecret.clientId} label={t('profileSettings.oauth.copyClientId')} copiedLabel={t('common.copied')} />
                    </div>
                  </div>
                )}
                <div className="space-y-1.5">
                  <p className="text-sm font-medium">{t('profileSettings.oauth.clientSecret')}</p>
                  <div className="flex items-center gap-1 rounded-lg border border-border/60 bg-background/30 py-1 pl-3 pr-1">
                    <code className="min-w-0 flex-1 break-all font-mono text-[13px]">{createdSecret.secret}</code>
                    <CopyButton value={createdSecret.secret} label={t('profileSettings.oauth.copySecret')} copiedLabel={t('common.copied')} />
                  </div>
                </div>
              </div>
              <DialogFooter>
                <Button onClick={() => closeOauthDialog(false)}>{t('profileSettings.twoFactorSetup.done')}</Button>
              </DialogFooter>
            </>
          ) : (
            <form
              className="grid gap-4"
              onSubmit={(e) => {
                e.preventDefault();
                if (oauthForm.name && !loading) void handleCreateClient();
              }}
            >
              <DialogHeader>
                <DialogTitle>{t('profileSettings.oauth.create')}</DialogTitle>
                <DialogDescription>{t('profileSettings.oauth.createDescription')}</DialogDescription>
              </DialogHeader>
              <Field id="client-name" label={t('profileSettings.oauth.nameLabel')}>
                <Input id="client-name" autoComplete="off" value={oauthForm.name} onChange={e => setOauthForm(prev => ({ ...prev, name: e.target.value }))} />
              </Field>
              <Field id="client-desc" label={t('profileSettings.oauth.descriptionLabel')}>
                <Input id="client-desc" autoComplete="off" value={oauthForm.description} onChange={e => setOauthForm(prev => ({ ...prev, description: e.target.value }))} />
              </Field>
              <DialogFooter className="gap-2">
                <Button type="button" variant="outline" onClick={() => closeOauthDialog(false)}>{t('common.cancel')}</Button>
                <Button type="submit" disabled={!oauthForm.name || loading}>
                  {loading && <Loader2 className="animate-spin" aria-hidden="true" />}
                  {t('profileSettings.oauth.createAction')}
                </Button>
              </DialogFooter>
            </form>
          )}
        </DialogContent>
      </Dialog>

      {/* Delete OAuth2 client */}
      <AlertDialog open={pendingClientDelete !== null} onOpenChange={(open) => !open && setPendingClientDelete(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('profileSettings.oauth.deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>{t('profileSettings.oauth.deleteDescription')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => pendingClientDelete && void handleDeleteClient(pendingClientDelete)}
            >
              {t('common.delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
