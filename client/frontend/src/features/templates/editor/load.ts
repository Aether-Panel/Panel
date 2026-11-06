import { api, ApiError } from '@/lib/api-client';
import { blankTemplate, duplicateTemplate, prepareLoadedTemplate, readImportedTemplate, type EditorSource } from './initial';
import { isPlainObject, type JsonObject } from './paths';

// The editor's calls to the templates API.

export type EditorStart =
  | {
      ok: true;
      template: JsonObject;
      /** The local template being edited; undefined for a new one. */
      originalName?: string;
      /** False for a copy or an import, which only exist in the editor until saved. */
      saved: boolean;
      takenNames: string[];
    }
  | { ok: false; problem: 'notFound' | 'importMissing' | 'loadFailed'; detail?: string };

const templateUrl = (repoId: number, name: string) => `/api/templates/${repoId}/${encodeURIComponent(name)}`;

/** Names of the local templates; empty when they cannot be read (then nothing is reported taken). */
async function localTemplateNames(): Promise<string[]> {
  try {
    const list: unknown = await api.get('/api/templates/0');
    return Array.isArray(list) ? list.flatMap((t) => (isPlainObject(t) && typeof t.name === 'string' ? [t.name] : [])) : [];
  } catch {
    return [];
  }
}

async function fetchTemplate(repoId: number, name: string): Promise<JsonObject | 'notFound'> {
  try {
    const data: unknown = await api.get(templateUrl(repoId, name));
    return isPlainObject(data) ? data : 'notFound';
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) return 'notFound';
    throw e;
  }
}

/** Everything the editor needs to open `source`. `copyLabel` marks a renamed copy's display name. */
export async function loadEditorStart(source: EditorSource, copyLabel: string): Promise<EditorStart> {
  try {
    const takenNames = await localTemplateNames();
    switch (source.kind) {
      case 'blank':
        return { ok: true, template: blankTemplate(), saved: true, takenNames };
      case 'import': {
        const template = readImportedTemplate();
        return template ? { ok: true, template, saved: false, takenNames } : { ok: false, problem: 'importMissing' };
      }
      case 'local': {
        const raw = await fetchTemplate(0, source.name);
        if (raw === 'notFound') return { ok: false, problem: 'notFound', detail: source.name };
        return { ok: true, template: prepareLoadedTemplate(raw, source.name), originalName: source.name, saved: true, takenNames };
      }
      case 'copy': {
        const raw = await fetchTemplate(source.repoId, source.name);
        if (raw === 'notFound') return { ok: false, problem: 'notFound', detail: source.name };
        return { ok: true, template: duplicateTemplate(raw, source.name, takenNames, copyLabel), saved: false, takenNames };
      }
    }
  } catch (e) {
    return { ok: false, problem: 'loadFailed', detail: e instanceof Error ? e.message : undefined };
  }
}

/** Saves `template` as the local template `name`, creating or replacing it. */
export async function saveLocalTemplate(name: string, template: JsonObject): Promise<void> {
  await api.put(templateUrl(0, name), template);
}
