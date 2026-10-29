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

/** Pages where a 401 is part of the flow (signing in), not an expired session */
const AUTH_PAGES = ['/login', '/register', '/forgot-password', '/reset-password'];
let sentToLogin = false;

/** sessionStorage key with the page to return to after signing in */
export const RETURN_TO_KEY = 'aether:return-to';

/**
 * Remembers this page for after signing in. It is kept in sessionStorage, not in the login
 * address: a ?next= anyone can put in a link would let a crafted link send the user elsewhere.
 */
export function rememberPageForLogin() {
    try {
        sessionStorage.setItem(RETURN_TO_KEY, window.location.pathname + window.location.search);
    } catch {
        // without storage the login goes to the dashboard
    }
}

/**
 * The session ended while the page was open (it expired, or was closed elsewhere): every call
 * would fail from here on and the page would show misleading errors, so go to the login page,
 * which brings the user back here afterwards.
 */
function sessionExpired(url: string) {
    if (sentToLogin || typeof window === 'undefined') return;
    // the panel's own sign-in calls answer 401 for a wrong password; that is not an expired session
    if (url.startsWith('/auth/')) return;
    const path = window.location.pathname.replace(/\/$/, '') || '/';
    if (AUTH_PAGES.includes(path)) return;
    sentToLogin = true;
    rememberPageForLogin();
    window.location.href = '/login/?expired=1';
}

async function handleResponse(response: Response, url = '') {
    const contentType = response.headers.get('content-type');
    let data = null;

    if (contentType?.includes('application/json')) {
        data = await response.json();
    } else {
        data = await response.text();
    }

    if (!response.ok) {
        if (response.status === 401) sessionExpired(url);
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
        return handleResponse(response, url);
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
        return handleResponse(response, url);
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
        return handleResponse(response, url);
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
        return handleResponse(response, url);
    },

    async delete<T = any>(url: string): Promise<T> {
        const response = await fetch(url, {
            method: 'DELETE',
            headers: {
                'Accept': 'application/json',
            },
            credentials: 'include',
        });
        return handleResponse(response, url);
    },
};
