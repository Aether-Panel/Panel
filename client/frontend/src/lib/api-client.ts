import en from '@/lib/locales/en.json';
import es from '@/lib/locales/es.json';

// Errors the panel names by code read in the viewer's language when the locale has them;
// the rest keep the panel's own message.
const API_ERRORS: Record<string, Record<string, string>> = { en: en.apiErrors, es: es.apiErrors };

function localizedError(code: unknown): string | undefined {
    if (typeof code !== 'string') return undefined;
    let language = 'es';
    try {
        language = localStorage.getItem('aether_panel_language') || 'es';
    } catch {
        // storage blocked: the panel's default language
    }
    return (API_ERRORS[language] ?? API_ERRORS.es)[code];
}

export class ApiError extends Error {
    constructor(public status: number, message: string, public data?: any) {
        super(message);
        this.name = 'ApiError';
    }
}

async function handleResponse(response: Response) {
    const contentType = response.headers.get('content-type');
    let data = null;

    if (contentType?.includes('application/json')) {
        data = await response.json();
    } else {
        data = await response.text();
    }

    if (!response.ok) {
        let message = response.statusText;
        if (data?.error) {
            if (typeof data.error === 'string') {
                message = data.error;
            } else if (typeof data.error === 'object' && data.error.msg) {
                message = localizedError(data.error.code) ?? data.error.msg;
            }
        }
        throw new ApiError(response.status, message, data);
    }

    return data;
}

export const api = {
    async get<T = any>(url: string): Promise<T> {
        const response = await fetch(url, {
            method: 'GET',
            headers: {
                'Accept': 'application/json',
            },
            credentials: 'include',
        });
        return handleResponse(response);
    },

    async post<T = any>(url: string, body: any, signal?: AbortSignal): Promise<T> {
        const response = await fetch(url, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Accept': 'application/json',
            },
            body: JSON.stringify(body),
            credentials: 'include',
            signal,
        });
        return handleResponse(response);
    },

    async postForm<T = any>(url: string, formData: FormData): Promise<T> {
        const response = await fetch(url, {
            method: 'POST',
            headers: {
                'Accept': 'application/json',
            },
            body: formData,
            credentials: 'include',
        });
        return handleResponse(response);
    },

    async put<T = any>(url: string, body: any): Promise<T> {
        const response = await fetch(url, {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
                'Accept': 'application/json',
            },
            body: JSON.stringify(body),
            credentials: 'include',
        });
        return handleResponse(response);
    },

    async delete<T = any>(url: string): Promise<T> {
        const response = await fetch(url, {
            method: 'DELETE',
            headers: {
                'Accept': 'application/json',
            },
            credentials: 'include',
        });
        return handleResponse(response);
    },
};
