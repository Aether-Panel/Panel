'use client';

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { api } from '@/lib/api-client';

export interface Config {
    themes: {
        active: string;
        settings: string;
        available: string[];
    };
    branding: {
        name: string;
    };
    registrationEnabled: boolean;
    turnstileSiteKey: string;
    turnstileEnabled: boolean;
    /** The anti-bot check the sign-in forms use */
    captcha?: 'turnstile' | 'builtin' | '';
    experimental?: ExperimentalFeatures;
    version: string;
}

/** Features an admin turns on in Settings › General › Experimental features. */
export interface ExperimentalFeatures {
    versionSwitch: boolean;
    propertiesEditor: boolean;
    serverClone: boolean;
}

const NO_EXPERIMENTAL: ExperimentalFeatures = { versionSwitch: false, propertiesEditor: false, serverClone: false };

interface ConfigContextType {
    config: Config | null;
    loading: boolean;
    refresh: () => Promise<void>;
}

const ConfigContext = createContext<ConfigContextType | undefined>(undefined);

export function ConfigProvider({ children }: Readonly<{ children: ReactNode }>) {
    const [config, setConfig] = useState<Config | null>(null);
    const [loading, setLoading] = useState(true);

    const fetchConfig = useCallback(async () => {
        try {
            setLoading(true);
            const data = await api.get('/api/config');
            setConfig(data);
        } catch (e) {
            console.error('Failed to fetch config:', e);
        } finally {
            setLoading(false);
        }
    }, []);

    useEffect(() => {
        void fetchConfig();
    }, [fetchConfig]);

    const value = useMemo(() => ({ config, loading, refresh: fetchConfig }), [config, loading, fetchConfig]);

    return (
        <ConfigContext.Provider value={value}>
            {children}
        </ConfigContext.Provider>
    );
}

export function useConfig() {
    const context = useContext(ConfigContext);
    if (context === undefined) {
        throw new Error('useConfig must be used within a ConfigProvider');
    }
    return context;
}

/**
 * Which experimental features are on. Everything reads as off until the config
 * has loaded (and on panels too old to send it), so nothing flashes in and out.
 */
export function useExperimental(): { features: ExperimentalFeatures; ready: boolean } {
    const { config, loading } = useConfig();
    return { features: config?.experimental ?? NO_EXPERIMENTAL, ready: !loading };
}
