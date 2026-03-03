import { useState, useEffect } from 'react';
import { api } from '@/lib/api-client';

export type TemplateRepo = {
    id: number;
    name: string;
    url: string;
    isLocal: boolean;
};

export type TemplateInfo = {
    name: string;
    readme?: string;
};

export function useTemplates() {
    const [repos, setRepos] = useState<TemplateRepo[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<Error | null>(null);

    const fetchRepos = async () => {
        try {
            setLoading(true);
            const data = await api.get('/api/templates');
            setRepos(Array.isArray(data) ? data : []);
            setError(null);
        } catch (e: any) {
            setError(e);
            console.error('Failed to fetch template repos:', e);
        } finally {
            setLoading(false);
        }
    };

    const getTemplatesForRepo = async (repoId: number) => {
        try {
            const data = await api.get(`/api/templates/${repoId}`);
            return Array.isArray(data) ? data : [];
        } catch (e) {
            console.error(`Failed to fetch templates for repo ${repoId}:`, e);
            return [];
        }
    };

    const getTemplateDetails = async (repoId: number, templateName: string) => {
        try {
            const data = await api.get(`/api/templates/${repoId}/${templateName}`);
            return data;
        } catch (e) {
            console.error(`Failed to fetch details for template ${templateName}:`, e);
            return null;
        }
    };

    const saveTemplate = async (templateName: string, templateData: any) => {
        try {
            await api.put(`/api/templates/0/${templateName}`, templateData);
            return true;
        } catch (e) {
            console.error(`Failed to save template ${templateName}:`, e);
            throw e;
        }
    };

    useEffect(() => {
        fetchRepos();
    }, []);

    return { repos, loading, error, refresh: fetchRepos, getTemplatesForRepo, getTemplateDetails, saveTemplate };
}
