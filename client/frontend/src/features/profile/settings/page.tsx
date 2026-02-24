'use client';
import { PageHeader } from '@/components/page-header';
import { Card, CardContent, CardDescription, CardHeader, CardTitle, CardFooter } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useAuth } from '@/contexts/providers';

import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Settings, User, KeyRound, ShieldCheck, Code } from 'lucide-react';
import { useState } from 'react';
import { useTranslations } from '@/contexts/translations-context';

export default function ProfileSettingsPage() {
  const { user } = useAuth();
  const { t, language, setLanguage } = useTranslations();
  const [activeTab, setActiveTab] = useState('settings');

  const profileSettingsTabs = [
      { value: 'settings', label: t('profileSettings.tabs.settings'), icon: Settings },
      { value: 'details', label: t('profileSettings.tabs.details'), icon: User },
      { value: 'password', label: t('profileSettings.tabs.password'), icon: KeyRound },
      { value: '2fa', label: t('profileSettings.tabs.twoFactor'), icon: ShieldCheck },
      { value: 'oauth', label: t('profileSettings.tabs.oauth'), icon: Code },
  ];

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
                    <Input id="username" defaultValue={user?.name.split(' ')[0].toLowerCase() || 'admin'} />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="email">{t('profileSettings.accountDetails.email')}</Label>
                    <Input id="email" type="email" defaultValue={user?.email} />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="confirm-password-details">{t('profileSettings.accountDetails.confirmPassword')}</Label>
                    <Input id="confirm-password-details" type="password" placeholder={t('profileSettings.accountDetails.confirmPassword')} />
                  </div>
                </CardContent>
                <CardFooter>
                  <Button>{t('profileSettings.accountDetails.save')}</Button>
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
                    <Input id="old-password" type="password" placeholder={t('profileSettings.changePassword.oldPassword')} />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="new-password">{t('profileSettings.changePassword.newPassword')}</Label>
                    <Input id="new-password" type="password" placeholder={t('profileSettings.changePassword.newPassword')} />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="confirm-password-change">{t('profileSettings.changePassword.confirmPassword')}</Label>
                    <Input id="confirm-password-change" type="password" placeholder={t('profileSettings.changePassword.confirmPassword')} />
                  </div>
                </CardContent>
                <CardFooter>
                  <Button>{t('profileSettings.changePassword.save')}</Button>
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
                <CardFooter>
                  <Button>{t('profileSettings.twoFactor.enable')}</Button>
                </CardFooter>
              </Card>
            </div>
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
                  <p className="text-center text-sm text-muted-foreground py-4">
                    {t('profileSettings.oauth.noClients')}
                  </p>
                </CardContent>
                <CardFooter>
                  <Button>{t('profileSettings.oauth.create')}</Button>
                </CardFooter>
              </Card>
            </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}
