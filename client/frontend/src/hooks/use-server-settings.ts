import { useState, useEffect } from 'react';
import { api } from '@/lib/api-client';

export type SettingVariable = {
    display: string;
    desc: string;
    type: 'string' | 'integer' | 'boolean' | 'option';
    value: string | number | boolean;
    options?: any[];
    required?: boolean;
    userEdit?: boolean;
    internal?: boolean;
};

export type SettingGroup = {
    display: string;
    description?: string;
    order: number;
    variables: string[];
    if?: any;
};

export type ServerSettings = {
    variables: Record<string, SettingVariable>;
    groups: SettingGroup[];
    flags: {
        autoStart: boolean;
        autoRestartOnCrash: boolean;
        autoRestartOnGraceful: boolean;
    };
    definition: any;
};

export function useServerSettings(serverId: string) {
    const [settings, setSettings] = useState<ServerSettings | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<Error | null>(null);
    const [isMinecraftJava, setIsMinecraftJava] = useState(false);

    const fetchSettings = async () => {
        try {
            setLoading(true);
            const [serverData, flagsData, definitionData] = await Promise.all([
                api.get(`/api/servers/${serverId}/data`).catch(() => ({ data: {} })),
                api.get(`/api/servers/${serverId}/flags`).catch(() => ({})),
                api.get(`/api/servers/${serverId}/definition`).catch(() => ({}))
            ]);

            const variables = {
                ...(serverData.data || {}),
                ...(definitionData.data || {})
            };
            const groups = definitionData.groups || serverData.groups || [];
            const type = definitionData.type || '';

            setIsMinecraftJava(type === 'minecraft' || type === 'minecraft-java');

            setSettings({
                variables,
                groups,
                flags: {
                    autoStart: !!flagsData.autoStart,
                    autoRestartOnCrash: !!flagsData.autoRestartOnCrash,
                    autoRestartOnGraceful: !!flagsData.autoRestartOnGraceful,
                },
                definition: definitionData
            });
            setError(null);
        } catch (e: any) {
            setError(e);
            console.error('Failed to fetch server settings:', e);
        } finally {
            setLoading(false);
        }
    };

    const saveSettings = async (newSettings: ServerSettings) => {
        try {
            // Prepare data for saving
            const data: Record<string, any> = {};
            Object.entries(newSettings.variables).forEach(([name, variable]) => {
                data[name] = variable.value;
            });

            // Sync variables into definition before saving if definition exists
            const definition = newSettings.definition ? {
                ...newSettings.definition,
                data: {
                    ...(newSettings.definition.data || {}),
                    ...(newSettings.variables || {})
                }
            } : null;

            // Save variables
            await api.post(`/api/servers/${serverId}/data`, data);

            // If we have a definition, update its internal run flags to match the new flags
            // This prevents overwriting the flags when saving the definition
            let finalDefinition = definition;
            if (finalDefinition) {
                if (!finalDefinition.run) finalDefinition.run = {};
                finalDefinition.run.autostart = newSettings.flags.autoStart;
                finalDefinition.run.autorecover = newSettings.flags.autoRestartOnCrash;
                finalDefinition.run.autorestart = newSettings.flags.autoRestartOnGraceful;

                await api.put(`/api/servers/${serverId}/definition`, finalDefinition);
            }

            // Also save via flags endpoint for consistency/completeness
            await api.post(`/api/servers/${serverId}/flags`, {
                autoStart: newSettings.flags.autoStart,
                autoRestartOnCrash: newSettings.flags.autoRestartOnCrash,
                autoRestartOnGraceful: newSettings.flags.autoRestartOnGraceful,
            });

            // If Minecraft Java, sync server.properties
            if (isMinecraftJava) {
                await syncServerProperties(serverId, data);
            }

            setSettings(newSettings);
            return true;
        } catch (e: any) {
            console.error('Failed to save settings:', e);
            throw e;
        }
    };

    useEffect(() => {
        if (serverId) {
            fetchSettings();
        }
    }, [serverId]);

    return { settings, loading, error, saveSettings, isMinecraftJava, refresh: fetchSettings };
}

async function syncServerProperties(serverId: string, data: Record<string, any>) {
    try {
        // This replicates the logic from the old frontend
        // Get current server.properties
        let content = '';
        try {
            content = await api.get(`/api/servers/${serverId}/file/server.properties`);
        } catch (e) {
            // If file doesn't exist, we'll start with empty
        }

        if (typeof content !== 'string') {
            // Handle if it returned JSON (file list) instead of string
            return;
        }

        const lines = content.split('\n');
        const updatedLines = [];
        const propsToUpdate: Record<string, string> = {};

        if (data.motd !== undefined) propsToUpdate['motd'] = String(data.motd).replace(/\n/g, '\\n');
        if (data.ip !== undefined) propsToUpdate['server-ip'] = String(data.ip);
        if (data.port !== undefined) propsToUpdate['server-port'] = String(data.port);

        if (Object.keys(propsToUpdate).length === 0) return;

        const existingKeys = new Set();
        for (const line of lines) {
            const trimmed = line.trim();
            if (!trimmed || trimmed.startsWith('#')) {
                updatedLines.push(line);
                continue;
            }

            const eqIndex = trimmed.indexOf('=');
            if (eqIndex === -1) {
                updatedLines.push(line);
                continue;
            }

            const key = trimmed.substring(0, eqIndex).trim().toLowerCase();
            if (propsToUpdate[key] !== undefined) {
                updatedLines.push(`${key}=${propsToUpdate[key]}`);
                existingKeys.add(key);
            } else if (key === 'server-ip' && propsToUpdate['server-ip'] !== undefined) {
                updatedLines.push(`server-ip=${propsToUpdate['server-ip']}`);
                existingKeys.add('server-ip');
            } else if (key === 'server-port' && propsToUpdate['server-port'] !== undefined) {
                updatedLines.push(`server-port=${propsToUpdate['server-port']}`);
                existingKeys.add('server-port');
            } else {
                updatedLines.push(line);
            }
        }

        // Add missing ones
        Object.entries(propsToUpdate).forEach(([key, value]) => {
            if (!existingKeys.has(key)) {
                updatedLines.push(`${key}=${value}`);
            }
        });

        await fetch(`/api/servers/${serverId}/file/server.properties`, {
            method: 'PUT',
            body: updatedLines.join('\n'),
            credentials: 'include'
        });
    } catch (e) {
        console.warn('Failed to sync server.properties:', e);
    }
}
