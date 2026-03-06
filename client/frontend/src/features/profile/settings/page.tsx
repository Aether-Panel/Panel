'use client';
import { PageHeader } from '@/components/page-header';
import { Card, CardContent, CardDescription, CardHeader, CardTitle, CardFooter } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useAuth } from '@/contexts/providers';
import { useProfile } from '@/hooks/use-profile';
import { useToast } from '@/hooks/use-toast';

import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Settings, User, KeyRound, ShieldCheck, Code, Loader2, Download, Trash2, Plus } from 'lucide-react';
import { useState, useEffect } from 'react';
import { useTranslations } from '@/contexts/translations-context';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';

export default function ProfileSettingsPage() {
  const { user, fetchSelf } = useAuth();
  const { otpEnabled, oauthClients, loading, updateDetails, startOtpEnroll, validateOtpEnroll, disableOtp, regenerateRecoveryCodes, createOauthClient, deleteOauthClient } = useProfile();
  const { t, language, setLanguage } = useTranslations();
  const { toast } = useToast();
  const [activeTab, setActiveTab] = useState('settings');
  const [isMounted, setIsMounted] = useState(false);

  // Form states
  const [detailsForm, setDetailsForm] = useState({ username: '', email: '', password: '' });
  const [passwordForm, setPasswordForm] = useState({ oldPassword: '', newPassword: '', confirmPassword: '' });
  const [oauthForm, setOauthForm] = useState({ name: '', description: '' });

  // 2FA state
  const [otpDialog, setOtpDialog] = useState(false);
  const [otpStep, setOtpStep] = useState<'enroll' | 'confirm' | 'recovery'>('enroll');
  const [otpData, setOtpData] = useState<{ secret?: string; img?: string; recoveryCodes?: string[] }>({});
  const [otpToken, setOtpToken] = useState('');

  useEffect(() => {
    setIsMounted(true);
    if (user) {
      setDetailsForm({ username: user.username, email: user.email, password: '' });
    }
  }, [user]);

  if (!isMounted) {
    return null;
  }

  const profileSettingsTabs = [
    { value: 'settings', label: t('profileSettings.tabs.settings'), icon: Settings },
    { value: 'details', label: t('profileSettings.tabs.details'), icon: User },
    { value: 'password', label: t('profileSettings.tabs.password'), icon: KeyRound },
    { value: '2fa', label: t('profileSettings.tabs.twoFactor'), icon: ShieldCheck },
    { value: 'oauth', label: t('profileSettings.tabs.oauth'), icon: Code },
  ];

  const handleUpdateDetails = async () => {
    try {
      await updateDetails({ username: detailsForm.username, email: detailsForm.email, password: detailsForm.password });
      await fetchSelf();
      toast({ title: t('common.success'), description: t('profileSettings.messages.infoChanged') });
      setDetailsForm(prev => ({ ...prev, password: '' }));
    } catch (e: any) {
      toast({ title: t('common.error'), description: e.message, variant: 'destructive' });
    }
  };

  const handleChangePassword = async () => {
    if (passwordForm.newPassword !== passwordForm.confirmPassword) {
      toast({ title: t('common.error'), description: "Passwords do not match", variant: "destructive" });
      return;
    }
    try {
      await updateDetails({ password: passwordForm.oldPassword, newPassword: passwordForm.newPassword });
      toast({ title: t('common.success'), description: t('profileSettings.messages.passwordChanged') });
      setPasswordForm({ oldPassword: '', newPassword: '', confirmPassword: '' });
    } catch (e: any) {
      toast({ title: t('common.error'), description: e.message, variant: 'destructive' });
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
      toast({ title: t('common.error'), description: e.message, variant: 'destructive' });
    }
  };

  const handleConfirmEnroll = async () => {
    try {
      const res = await validateOtpEnroll(otpToken);
      setOtpData(prev => ({ ...prev, recoveryCodes: res.recoveryCodes }));
      setOtpStep('recovery');
      toast({ title: t('common.success'), description: t('profileSettings.messages.otpEnabled') });
    } catch (e: any) {
      toast({ title: t('common.error'), description: e.message, variant: 'destructive' });
    }
  };

  const handleDisableOtp = async () => {
    const token = prompt(t('profileSettings.twoFactorSetup.confirmHint'));
    if (!token) return;
    try {
      await disableOtp(token);
      toast({ title: t('common.success'), description: t('profileSettings.messages.otpDisabled') });
    } catch (e: any) {
      toast({ title: t('common.error'), description: e.message, variant: 'destructive' });
    }
  };

  const handleRegenerateRecovery = async () => {
    const token = prompt(t('profileSettings.twoFactorSetup.confirmHint'));
    if (!token) return;
    try {
      const res = await regenerateRecoveryCodes(token);
      setOtpData({ recoveryCodes: res.recoveryCodes });
      setOtpStep('recovery');
      setOtpDialog(true);
      toast({ title: t('common.success'), description: t('profileSettings.messages.recoveryCodesRegenerated') });
    } catch (e: any) {
      toast({ title: t('common.error'), description: e.message, variant: 'destructive' });
    }
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
      toast({ title: t('common.success'), description: `OAuth client ${client.name} created` });
      setOauthForm({ name: '', description: '' });
      // Show secret to user if available
      if (client.clientSecret) {
        alert(`Client Secret: ${client.clientSecret}\nPLEASE SAVE THIS NOW! IT WILL NOT BE SHOWN AGAIN.`);
      }
    } catch (e: any) {
      toast({ title: t('common.error'), description: e.message, variant: 'destructive' });
    }
  };

  const handleDeleteClient = async (id: string) => {
    if (!confirm("Are you sure you want to delete this OAuth client?")) return;
    try {
      await deleteOauthClient(id);
      toast({ title: t('common.success'), description: "OAuth client deleted" });
    } catch (e: any) {
      toast({ title: t('common.error'), description: e.message, variant: 'destructive' });
    }
  };

  return (
    <div className="flex flex-col gap-8">
      <PageHeader title={t('profileSettings.title')} description={t('profileSettings.description')} />

      <Tabs value={activeTab} onValueChange={setActiveTab} defaultValue="settings" className="w-full">
        <div className="md:hidden mb-4">
          <Select value={activeTab} onValueChange={setActiveTab}>
            <SelectTrigger>
              <SelectValue placeholder={t('profileSettings.selectTabPlaceholder')} />
            </SelectTrigger>
            <SelectContent>
              {profileSettingsTabs.map((tab) => (
                <SelectItem key={tab.value} value={tab.value}>
                  <div className="flex items-center gap-2">
                    <tab.icon className="h-4 w-4" />
                    <span>{tab.label}</span>
                  </div>
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <TabsList className="hidden md:grid w-full grid-cols-5">
          {profileSettingsTabs.map((tab) => (
            <TabsTrigger key={tab.value} value={tab.value}>
              <tab.icon className="mr-2 h-4 w-4" />
              {tab.label}
            </TabsTrigger>
          ))}
        </TabsList>
        <TabsContent value="settings">
          <div className="mt-6 rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
            <Card className="border-0">
              <CardHeader>
                <CardTitle>{t('profileSettings.general.title')}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="language">{t('profileSettings.general.language')}</Label>
                  <Select value={language} onValueChange={(value) => setLanguage(value as 'es' | 'en')}>
                    <SelectTrigger id="language">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="es">{t('profileSettings.general.language_es')}</SelectItem>
                      <SelectItem value="en">{t('profileSettings.general.language_en')}</SelectItem>
                    </SelectContent>
                  </Select>
                  <p className="text-sm text-muted-foreground">
                    {t('profileSettings.general.language_help')}
                  </p>
                </div>
              </CardContent>
              <CardFooter>
                <Button>{t('profileSettings.general.save')}</Button>
              </CardFooter>
            </Card>
          </div>
        </TabsContent>
        <TabsContent value="details">
          <div className="mt-6 rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
            <Card className="border-0">
              <CardHeader>
                <CardTitle>{t('profileSettings.accountDetails.title')}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="username">{t('profileSettings.accountDetails.username')}</Label>
                  <Input id="username" value={detailsForm.username} onChange={e => setDetailsForm(prev => ({ ...prev, username: e.target.value }))} />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="email">{t('profileSettings.accountDetails.email')}</Label>
                  <Input id="email" type="email" value={detailsForm.email} onChange={e => setDetailsForm(prev => ({ ...prev, email: e.target.value }))} />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="confirm-password-details">{t('profileSettings.accountDetails.confirmPassword')}</Label>
                  <Input id="confirm-password-details" type="password" value={detailsForm.password} onChange={e => setDetailsForm(prev => ({ ...prev, password: e.target.value }))} placeholder={t('profileSettings.accountDetails.confirmPassword')} />
                </div>
              </CardContent>
              <CardFooter>
                <Button onClick={handleUpdateDetails} disabled={loading}>
                  {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                  {t('profileSettings.accountDetails.save')}
                </Button>
              </CardFooter>
            </Card>
          </div>
        </TabsContent>
        <TabsContent value="password">
          <div className="mt-6 rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
            <Card className="border-0">
              <CardHeader>
                <CardTitle>{t('profileSettings.changePassword.title')}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="old-password">{t('profileSettings.changePassword.oldPassword')}</Label>
                  <Input id="old-password" type="password" value={passwordForm.oldPassword} onChange={e => setPasswordForm(prev => ({ ...prev, oldPassword: e.target.value }))} placeholder={t('profileSettings.changePassword.oldPassword')} />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="new-password">{t('profileSettings.changePassword.newPassword')}</Label>
                  <Input id="new-password" type="password" value={passwordForm.newPassword} onChange={e => setPasswordForm(prev => ({ ...prev, newPassword: e.target.value }))} placeholder={t('profileSettings.changePassword.newPassword')} />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="confirm-password-change">{t('profileSettings.changePassword.confirmPassword')}</Label>
                  <Input id="confirm-password-change" type="password" value={passwordForm.confirmPassword} onChange={e => setPasswordForm(prev => ({ ...prev, confirmPassword: e.target.value }))} placeholder={t('profileSettings.changePassword.confirmPassword')} />
                </div>
              </CardContent>
              <CardFooter>
                <Button onClick={handleChangePassword} disabled={loading}>
                  {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                  {t('profileSettings.changePassword.save')}
                </Button>
              </CardFooter>
            </Card>
          </div>
        </TabsContent>
        <TabsContent value="2fa">
          <div className="mt-6 rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
            <Card className="border-0">
              <CardHeader>
                <CardTitle>{t('profileSettings.twoFactor.title')}</CardTitle>
                <CardDescription>
                  {t('profileSettings.twoFactor.description')}
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="flex items-center gap-2 mb-4">
                  <div className={`h-2 w-2 rounded-full ${otpEnabled ? 'bg-green-500' : 'bg-red-500'}`} />
                  <span className="text-sm font-medium">
                    {otpEnabled ? "2FA is enabled" : "2FA is disabled"}
                  </span>
                </div>
              </CardContent>
              <CardFooter className="flex gap-4">
                {otpEnabled ? (
                  <>
                    <Button variant="outline" onClick={handleRegenerateRecovery}>
                      <Download className="mr-2 h-4 w-4" />
                      {t('profileSettings.twoFactorSetup.downloadCodes')}
                    </Button>
                    <Button variant="destructive" onClick={handleDisableOtp}>
                      {t('profileSettings.twoFactor.enable').replace('Enable', 'Disable')}
                    </Button>
                  </>
                ) : (
                  <Button onClick={handleStartEnroll}>{t('profileSettings.twoFactor.enable')}</Button>
                )}
              </CardFooter>
            </Card>
          </div>

          <Dialog open={otpDialog} onOpenChange={setOtpDialog}>
            <DialogContent className="sm:max-w-md">
              <DialogHeader>
                <DialogTitle>{t('profileSettings.twoFactorSetup.title')}</DialogTitle>
                <DialogDescription>
                  {otpStep === 'enroll' && t('profileSettings.twoFactorSetup.qrCodeHint')}
                  {otpStep === 'confirm' && t('profileSettings.twoFactorSetup.confirmHint')}
                  {otpStep === 'recovery' && t('profileSettings.twoFactorSetup.recoveryHint')}
                </DialogDescription>
              </DialogHeader>

              {otpStep === 'enroll' && (
                <div className="flex flex-col items-center gap-4 py-4">
                  {otpData.img && <img src={otpData.img} alt="QR Code" className="rounded-lg shadow-md" />}
                  <div className="w-full space-y-2">
                    <Label>{t('profileSettings.twoFactorSetup.secretCode')}</Label>
                    <div className="bg-muted p-2 rounded font-mono text-center break-all select-all">
                      {otpData.secret}
                    </div>
                  </div>
                </div>
              )}

              {otpStep === 'enroll' && (
                <DialogFooter>
                  <Button onClick={() => setOtpStep('confirm')}>{t('common.next') || "Next"}</Button>
                </DialogFooter>
              )}

              {otpStep === 'confirm' && (
                <div className="py-4 space-y-4">
                  <div className="space-y-2">
                    <Label>{t('profileSettings.twoFactorSetup.confirmHint')}</Label>
                    <Input
                      value={otpToken}
                      onChange={e => setOtpToken(e.target.value)}
                      placeholder="000000"
                      maxLength={6}
                      className="text-center text-2xl tracking-widest"
                    />
                  </div>
                </div>
              )}

              {otpStep === 'confirm' && (
                <DialogFooter className="gap-2">
                  <Button variant="outline" onClick={() => setOtpStep('enroll')}>{t('common.back') || "Back"}</Button>
                  <Button onClick={handleConfirmEnroll} disabled={otpToken.length < 6 || loading}>
                    {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                    Confirm
                  </Button>
                </DialogFooter>
              )}

              {otpStep === 'recovery' && (
                <div className="py-4 space-y-4">
                  <Alert>
                    <Download className="h-4 w-4" />
                    <AlertTitle>{t('profileSettings.twoFactorSetup.recoveryTitle')}</AlertTitle>
                    <AlertDescription>{t('profileSettings.twoFactorSetup.recoveryHint')}</AlertDescription>
                  </Alert>
                  <div className="grid grid-cols-2 gap-2 bg-muted p-4 rounded-lg font-mono text-sm">
                    {otpData.recoveryCodes?.map(code => (
                      <div key={code}>{code}</div>
                    ))}
                  </div>
                </div>
              )}

              {otpStep === 'recovery' && (
                <DialogFooter className="gap-2">
                  <Button variant="outline" onClick={downloadRecoveryCodes}>
                    <Download className="mr-2 h-4 w-4" />
                    Download
                  </Button>
                  <Button onClick={() => setOtpDialog(false)}>Done</Button>
                </DialogFooter>
              )}
            </DialogContent>
          </Dialog>
        </TabsContent>
        <TabsContent value="oauth">
          <div className="mt-6 rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
            <Card className="border-0">
              <CardHeader>
                <CardTitle>{t('profileSettings.oauth.title')}</CardTitle>
                <CardDescription>
                  {t('profileSettings.oauth.description')}{' '}
                  {t('profileSettings.oauth.apiDocs')}{' '}
                  <a href="#" className="text-primary hover:underline">{t('profileSettings.oauth.here')}</a>.
                </CardDescription>
              </CardHeader>
              <CardContent>
                {oauthClients.length === 0 ? (
                  <p className="text-center text-sm text-muted-foreground py-4">
                    {t('profileSettings.oauth.noClients')}
                  </p>
                ) : (
                  <div className="space-y-4">
                    {oauthClients.map(client => (
                      <div key={client.clientId} className="flex items-center justify-between p-4 rounded-lg border bg-muted/30">
                        <div>
                          <div className="font-bold">{client.name}</div>
                          <div className="text-xs text-muted-foreground font-mono">{client.clientId}</div>
                          <div className="text-sm">{client.description}</div>
                        </div>
                        <Button variant="ghost" size="icon" onClick={() => handleDeleteClient(client.clientId)}>
                          <Trash2 className="h-4 w-4 text-destructive" />
                        </Button>
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
              <CardFooter>
                <Dialog>
                  <DialogTrigger asChild>
                    <Button>
                      <Plus className="mr-2 h-4 w-4" />
                      {t('profileSettings.oauth.create')}
                    </Button>
                  </DialogTrigger>
                  <DialogContent>
                    <DialogHeader>
                      <DialogTitle>{t('profileSettings.oauth.create')}</DialogTitle>
                    </DialogHeader>
                    <div className="space-y-4 py-4">
                      <div className="space-y-2">
                        <Label htmlFor="client-name">Name</Label>
                        <Input id="client-name" value={oauthForm.name} onChange={e => setOauthForm(prev => ({ ...prev, name: e.target.value }))} />
                      </div>
                      <div className="space-y-2">
                        <Label htmlFor="client-desc">Description</Label>
                        <Input id="client-desc" value={oauthForm.description} onChange={e => setOauthForm(prev => ({ ...prev, description: e.target.value }))} />
                      </div>
                    </div>
                    <DialogFooter>
                      <Button onClick={handleCreateClient} disabled={!oauthForm.name || loading}>
                        {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                        Create
                      </Button>
                    </DialogFooter>
                  </DialogContent>
                </Dialog>
              </CardFooter>
            </Card>
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}
