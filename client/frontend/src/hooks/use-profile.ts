import { useState, useEffect } from 'react';
import { api } from '@/lib/api-client';

export type OAuthClient = {
    clientId: string;
    userId: number;
    name: string;
    description: string;
    clientSecret?: string;
};

export function useProfile() {
    const [otpEnabled, setOtpEnabled] = useState(false);
    const [oauthClients, setOauthClients] = useState<OAuthClient[]>([]);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<Error | null>(null);

    const fetchOtpStatus = async () => {
        try {
            const data = await api.get('/api/self/otp');
            setOtpEnabled(data.otpEnabled);
        } catch (e: any) {
            console.error('Failed to fetch OTP status:', e);
        }
    };

    const fetchOauthClients = async () => {
        try {
            const data = await api.get('/api/self/oauth2');
            setOauthClients(Array.isArray(data) ? data : []);
        } catch (e: any) {
            console.error('Failed to fetch OAuth clients:', e);
        }
    };

    const updateDetails = async (details: { username?: string; email?: string; password?: string; newPassword?: string }) => {
        setLoading(true);
        try {
            await api.put('/api/self', details);
            setError(null);
        } catch (e: any) {
            setError(e);
            throw e;
        } finally {
            setLoading(false);
        }
    };

    const startOtpEnroll = async () => {
        try {
            return await api.post('/api/self/otp', {});
        } catch (e: any) {
            console.error('Failed to start OTP enroll:', e);
            throw e;
        }
    };

    const validateOtpEnroll = async (token: string) => {
        try {
            const res = await api.put('/api/self/otp', { token });
            await fetchOtpStatus();
            return res;
        } catch (e: any) {
            console.error('Failed to validate OTP enroll:', e);
            throw e;
        }
    };

    const disableOtp = async (token: string) => {
        try {
            await api.delete(`/api/self/otp/${token}`);
            await fetchOtpStatus();
        } catch (e: any) {
            console.error('Failed to disable OTP:', e);
            throw e;
        }
    };

    const regenerateRecoveryCodes = async (token: string) => {
        try {
            return await api.post('/api/self/otp/recovery', { token });
        } catch (e: any) {
            console.error('Failed to regenerate recovery codes:', e);
            throw e;
        }
    };

    const createOauthClient = async (client: { name: string; description: string }) => {
        try {
            const res = await api.post('/api/self/oauth2', client);
            await fetchOauthClients();
            return res;
        } catch (e: any) {
            console.error('Failed to create OAuth client:', e);
            throw e;
        }
    };

    const deleteOauthClient = async (clientId: string) => {
        try {
            await api.delete(`/api/self/oauth2/${clientId}`);
            await fetchOauthClients();
        } catch (e: any) {
            console.error('Failed to delete OAuth client:', e);
            throw e;
        }
    };

    useEffect(() => {
        fetchOtpStatus();
        fetchOauthClients();
    }, []);

    return {
        otpEnabled,
        oauthClients,
        loading,
        error,
        updateDetails,
        startOtpEnroll,
        validateOtpEnroll,
        disableOtp,
        regenerateRecoveryCodes,
        createOauthClient,
        deleteOauthClient,
        refresh: () => { fetchOtpStatus(); fetchOauthClients(); }
    };
}
