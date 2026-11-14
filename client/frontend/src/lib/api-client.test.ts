import { afterEach, describe, expect, it, vi } from 'vitest';
import { api, ApiError } from './api-client';

function answer(status: number, body: unknown) {
  vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } }))));
}

describe('api errors', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // the nodes and templates pages show the error's message when a delete is refused
  it('names the store products that keep a template or node from being deleted', async () => {
    answer(409, { error: { code: 'ErrTemplateInUse', msg: 'in use', metadata: { products: ['minecraft_2gb', 'rust_4gb'] } } });
    await expect(api.delete('/api/templates/0/paper')).rejects.toThrow(
      'The store products minecraft_2gb, rust_4gb create their servers with this template.',
    );

    answer(409, { error: { code: 'ErrNodeHasProducts', msg: 'in use', metadata: { products: ['minecraft_2gb'] } } });
    const refused = await api.delete('/api/nodes/2').catch((e: unknown) => e);
    expect(refused).toBeInstanceOf(ApiError);
    expect((refused as ApiError).status).toBe(409);
    expect((refused as ApiError).message).toContain('minecraft_2gb create their servers on this node');
  });

  it('keeps the panel message for a code without a translation', async () => {
    answer(500, { error: { code: 'ErrSomethingNew', msg: 'something new failed' } });
    await expect(api.delete('/api/nodes/2')).rejects.toThrow('something new failed');
  });
});
