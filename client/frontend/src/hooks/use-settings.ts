import { useState, useEffect } from 'react';
import { api } from '@/lib/api-client';

export function useSettings() {
    const [settings, setSettings] = useState<Record<string, any>>({});
    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState<Error | null>(null);

    const fetchSettings = async () => {
        try {
            setLoading(true);
            const data = await api.get('/api/settings');
            setSettings(data);
            setError(null);
        } catch (e: any) {
            setError(e);
            console.error('Failed to fetch settings:', e);
        } finally {
            setLoading(false);
        }
    };

    const saveSettings = async (newSettings: Record<string, any>) => {
        try {
            setSaving(true);
            await api.post('/api/settings', newSettings);
            setSettings(prev => ({ ...prev, ...newSettings }));
            return true;
        } catch (e: any) {
            console.error('Failed to save settings:', e);
            throw e;
        } finally {
            setSaving(false);
        }
    };

    const sendTestEmail = async (overrides: Record<string, any> = {}) => {
        try {
            await api.post('/api/settings/test/email', overrides);
            return true;
        } catch (e: any) {
            console.error('Failed to send test email:', e);
            throw e;
        }
    };

    /** Tests the webhooks as they are in the form, saved or not (keys panel.notifications.discordWebhook*). */
    const sendTestDiscord = async (unsaved: Record<string, any> = {}) => {
        const webhooks: Record<string, string> = {};
        for (const [key, value] of Object.entries(unsaved)) {
            if (key.startsWith('panel.notifications.discordWebhook')) webhooks[key] = String(value ?? '');
        }
        try {
            await api.post('/api/settings/test/discord', webhooks);
            return true;
        } catch (e: any) {
            console.error('Failed to send test discord webhook:', e);
            throw e;
        }
    };

    useEffect(() => {
        void fetchSettings();
    }, []);

    return {
        settings,
        loading,
        saving,
        error,
        saveSettings,
        sendTestEmail,
        sendTestDiscord,
        refresh: fetchSettings
    };
}
