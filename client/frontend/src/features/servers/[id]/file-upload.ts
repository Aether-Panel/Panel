/**
 * What an upload sends: files and folders dropped on the file manager or picked with the
 * upload buttons, each with its path relative to the folder being browsed.
 */

/** One thing to upload: a file, or a folder with nothing in it (file is then absent). */
export type UploadItem = { path: string; file?: File };

/** True when a drag carries files from the computer (not text or a link from the page). */
export function dragHasFiles(dataTransfer: DataTransfer | null): boolean {
  return !!dataTransfer && Array.from(dataTransfer.types).includes('Files');
}

/**
 * Files picked with an <input type="file">. A folder picked with webkitdirectory gives each
 * file a webkitRelativePath ("folder/sub/file.txt"), which keeps the folder's layout.
 */
export function itemsFromFiles(files: ArrayLike<File>): UploadItem[] {
  return Array.from(files).map((file) => ({ path: file.webkitRelativePath || file.name, file }));
}

// the entry API of a drop: FileSystemEntry and friends, typed here because not every lib has them
type Entry = {
  isFile: boolean;
  isDirectory: boolean;
  name: string;
  file?: (ok: (file: File) => void, fail: (err: unknown) => void) => void;
  createReader?: () => { readEntries: (ok: (entries: Entry[]) => void, fail: (err: unknown) => void) => void };
};

const readFile = (entry: Entry) => new Promise<File>((resolve, reject) => entry.file!(resolve, reject));

/** Every entry of a folder: readEntries hands them out in batches until one comes back empty. */
async function readFolder(entry: Entry): Promise<Entry[]> {
  const reader = entry.createReader!();
  const all: Entry[] = [];
  for (;;) {
    const batch = await new Promise<Entry[]>((resolve, reject) => reader.readEntries(resolve, reject));
    if (batch.length === 0) return all;
    all.push(...batch);
  }
}

async function walk(entry: Entry, prefix: string, out: UploadItem[]): Promise<void> {
  const path = prefix ? `${prefix}/${entry.name}` : entry.name;
  if (entry.isFile) {
    out.push({ path, file: await readFile(entry) });
    return;
  }
  if (!entry.isDirectory) return;
  const children = await readFolder(entry);
  if (children.length === 0) {
    out.push({ path });
    return;
  }
  for (const child of children) await walk(child, path, out);
}

/**
 * What a drop holds, folders walked into. The entries must be taken in the drop event itself,
 * before any await: the browser empties the DataTransfer as soon as the handler returns.
 */
export function itemsFromDrop(dataTransfer: DataTransfer): Promise<UploadItem[]> {
  const entries: Entry[] = [];
  for (const item of Array.from(dataTransfer.items ?? [])) {
    if (item.kind !== 'file') continue;
    const entry = (item as DataTransferItem & { webkitGetAsEntry?: () => Entry | null }).webkitGetAsEntry?.();
    if (entry) entries.push(entry);
  }
  // a browser without the entry API still gives the files (not folders)
  if (entries.length === 0) return Promise.resolve(itemsFromFiles(dataTransfer.files));

  return (async () => {
    const out: UploadItem[] = [];
    for (const entry of entries) await walk(entry, '', out);
    return out;
  })();
}

/** Where an upload is: items and bytes sent so far, and the item going up now. */
export type UploadProgress = {
  done: number;
  total: number;
  sentBytes: number;
  totalBytes: number;
  current: UploadItem | null;
};

/** Sends one item, telling how many of its bytes have gone; it must stop when signal aborts. */
export type UploadOne = (item: UploadItem, onBytes: (loaded: number) => void, signal: AbortSignal) => Promise<void>;

const isAbort = (error: unknown) => error instanceof DOMException && error.name === 'AbortError';

/**
 * Uploads the items one by one through upload, empty folders first, reporting the bytes as they
 * go. It stops at the first failure, or when signal aborts, and says how many items went up.
 */
export async function uploadAll(
  items: UploadItem[],
  upload: UploadOne,
  onProgress: (progress: UploadProgress) => void,
  signal: AbortSignal,
): Promise<{ done: number; error?: unknown; cancelled?: boolean }> {
  const ordered = [...items.filter((i) => !i.file), ...items.filter((i) => i.file)];
  const total = ordered.length;
  const totalBytes = ordered.reduce((sum, i) => sum + (i.file?.size ?? 0), 0);
  let done = 0;
  let finishedBytes = 0;
  for (const item of ordered) {
    if (signal.aborted) return { done, cancelled: true };
    const size = item.file?.size ?? 0;
    onProgress({ done, total, sentBytes: finishedBytes, totalBytes, current: item });
    try {
      await upload(
        item,
        (loaded) => onProgress({ done, total, sentBytes: finishedBytes + Math.min(loaded, size), totalBytes, current: item }),
        signal,
      );
    } catch (error) {
      return isAbort(error) || signal.aborted ? { done, cancelled: true } : { done, error };
    }
    done++;
    finishedBytes += size;
  }
  onProgress({ done, total, sentBytes: finishedBytes, totalBytes, current: null });
  return { done };
}

/**
 * A PUT whose upload progress can be followed (fetch cannot report it). It rejects with the
 * daemon's own message when it answers with an error, and with an AbortError when cancelled.
 */
export function putWithProgress(
  url: string,
  body: FormData | null,
  onBytes: (loaded: number) => void,
  signal: AbortSignal,
  fallbackError: string,
): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal.aborted) {
      reject(new DOMException('Upload cancelled', 'AbortError'));
      return;
    }
    const xhr = new XMLHttpRequest();
    const abort = () => xhr.abort();
    signal.addEventListener('abort', abort, { once: true });
    const finish = () => signal.removeEventListener('abort', abort);

    xhr.open('PUT', url);
    xhr.withCredentials = true;
    xhr.upload.onprogress = (e) => onBytes(e.loaded);
    xhr.onload = () => {
      finish();
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve();
        return;
      }
      reject(new Error(errorFromBody(xhr.responseText) || fallbackError));
    };
    xhr.onerror = () => {
      finish();
      reject(new Error(fallbackError));
    };
    xhr.onabort = () => {
      finish();
      reject(new DOMException('Upload cancelled', 'AbortError'));
    };
    xhr.send(body);
  });
}

/** The message of an error answer ({"error": "…"} or {"error": {"msg": "…"}}), if it has one. */
export function errorFromBody(text: string): string {
  try {
    const error = JSON.parse(text)?.error;
    if (typeof error === 'string') return error;
    return typeof error?.msg === 'string' ? error.msg : '';
  } catch {
    return '';
  }
}

/**
 * The upload speed over the last few seconds, in bytes per second: an average over the whole
 * upload would lag behind a connection that speeds up or slows down.
 */
export function createRateMeter(windowMs = 4000) {
  let samples: { at: number; bytes: number }[] = [];
  return {
    sample(bytes: number, at: number) {
      samples.push({ at, bytes });
      samples = samples.filter((s) => at - s.at <= windowMs);
    },
    /** null until there is enough to tell */
    rate(): number | null {
      if (samples.length < 2) return null;
      const first = samples[0];
      const last = samples.at(-1)!;
      const seconds = (last.at - first.at) / 1000;
      if (seconds < 0.5) return null;
      return Math.max(0, (last.bytes - first.bytes) / seconds);
    },
  };
}
