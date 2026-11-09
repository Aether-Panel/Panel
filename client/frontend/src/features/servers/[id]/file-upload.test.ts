import { describe, expect, it } from 'vitest';
import { createRateMeter, dragHasFiles, errorFromBody, itemsFromDrop, itemsFromFiles, uploadAll, type UploadItem, type UploadProgress } from './file-upload';

const file = (name: string, relative = '') => {
  const f = new File(['x'], name);
  if (relative) Object.defineProperty(f, 'webkitRelativePath', { value: relative });
  return f;
};

// a folder entry whose reader hands its children out in batches of two, as browsers do
const folder = (name: string, children: any[]) => ({
  isFile: false,
  isDirectory: true,
  name,
  createReader: () => {
    let at = 0;
    return {
      readEntries: (ok: (e: any[]) => void) => {
        ok(children.slice(at, at + 2));
        at += 2;
      },
    };
  },
});
const fileEntry = (name: string) => ({
  isFile: true,
  isDirectory: false,
  name,
  file: (ok: (f: File) => void) => ok(file(name)),
});

const drop = (entries: any[], files: File[] = []) =>
  ({
    types: ['Files'],
    items: entries.map((entry) => ({ kind: 'file', webkitGetAsEntry: () => entry })),
    files,
  }) as unknown as DataTransfer;

describe('dragHasFiles', () => {
  it('only takes drags with files from the computer', () => {
    expect(dragHasFiles({ types: ['Files'] } as unknown as DataTransfer)).toBe(true);
    expect(dragHasFiles({ types: ['text/plain'] } as unknown as DataTransfer)).toBe(false);
    expect(dragHasFiles(null)).toBe(false);
  });
});

describe('itemsFromFiles', () => {
  it('keeps a picked folder layout and plain names otherwise', () => {
    expect(itemsFromFiles([file('a.txt'), file('b.txt', 'mods/b.txt')]).map((i) => i.path)).toEqual(['a.txt', 'mods/b.txt']);
  });
});

describe('itemsFromDrop', () => {
  it('walks folders past the first batch, keeps empty ones and loose files', async () => {
    const tree = folder('plugins', [fileEntry('a.jar'), fileEntry('b.jar'), fileEntry('c.jar'), folder('empty', []), folder('cfg', [fileEntry('x.yml')])]);
    const items = await itemsFromDrop(drop([tree, fileEntry('server.zip')]));
    expect(items.map((i) => [i.path, !!i.file])).toEqual([
      ['plugins/a.jar', true],
      ['plugins/b.jar', true],
      ['plugins/c.jar', true],
      ['plugins/empty', false],
      ['plugins/cfg/x.yml', true],
      ['server.zip', true],
    ]);
  });

  it('falls back to the plain files without the entry API', async () => {
    const dt = { types: ['Files'], items: [{ kind: 'file' }], files: [file('a.txt')] } as unknown as DataTransfer;
    expect((await itemsFromDrop(dt)).map((i) => i.path)).toEqual(['a.txt']);
  });
});

describe('uploadAll', () => {
  const sized = (name: string, bytes: number) => new File(['x'.repeat(bytes)], name);
  const items: UploadItem[] = [{ path: 'a', file: sized('a', 100) }, { path: 'empty' }, { path: 'b', file: sized('b', 300) }];
  const signal = () => new AbortController().signal;

  it('makes empty folders first and counts the bytes as they go', async () => {
    const sent: string[] = [];
    const seen: UploadProgress[] = [];
    const result = await uploadAll(items, async (i, onBytes) => {
      sent.push(i.path);
      if (i.file) {
        onBytes(i.file.size / 2);
        onBytes(i.file.size * 2); // a progress event past the size counts as the size
      }
    }, (p) => seen.push(p), signal());
    expect(sent).toEqual(['empty', 'a', 'b']);
    expect(result).toEqual({ done: 3 });
    expect(seen.map((p) => p.sentBytes)).toEqual([0, 0, 50, 100, 100, 250, 400, 400]);
    expect(seen.at(-1)).toMatchObject({ done: 3, total: 3, totalBytes: 400, current: null });
  });

  it('stops at the first failure with how many went up', async () => {
    const boom = new Error('disk full');
    const result = await uploadAll(items, async (i) => {
      if (i.path === 'b') throw boom;
    }, () => {}, signal());
    expect(result).toEqual({ done: 2, error: boom });
  });

  it('stops when cancelled, without calling it an error', async () => {
    const controller = new AbortController();
    const result = await uploadAll(items, async (i) => {
      if (i.path === 'a') {
        controller.abort();
        throw new DOMException('Upload cancelled', 'AbortError');
      }
    }, () => {}, controller.signal);
    expect(result).toEqual({ done: 1, cancelled: true });
  });
});

describe('errorFromBody', () => {
  it('reads both error shapes and gives up on anything else', () => {
    expect(errorFromBody('{"error":"disk full"}')).toBe('disk full');
    expect(errorFromBody('{"error":{"msg":"no space"}}')).toBe('no space');
    expect(errorFromBody('<html>bad gateway</html>')).toBe('');
  });
});

describe('createRateMeter', () => {
  it('measures the last seconds, not the whole upload', () => {
    const meter = createRateMeter(4000);
    meter.sample(0, 0);
    expect(meter.rate()).toBeNull();
    meter.sample(1000, 1000);
    expect(meter.rate()).toBe(1000);
    // a fast stretch later: the samples older than 4 s fall out, so only 5 s → 7 s counts
    meter.sample(1000, 5000);
    meter.sample(21000, 7000);
    expect(meter.rate()).toBe(10000);
  });
});
