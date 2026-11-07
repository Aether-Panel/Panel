import { describe, expect, it } from 'vitest';
import { createHeldIssues, unappliedJsonIssue } from './held-issues';

describe('held issues', () => {
  it('lists what fields hold and reports every change', () => {
    const changes: number[] = [];
    const held = createHeldIssues((list) => changes.push(list.length));
    const offA = held.register(unappliedJsonIssue(['environment', 'hostConfig'], 'templates.editor.jsonBlock.unapplied'));
    held.register(unappliedJsonIssue(['install', 0], 'templates.editor.steps.json.unapplied'));
    expect(held.list.map((i) => i.path)).toEqual(['environment.hostConfig', 'install.0']);
    offA();
    offA();
    expect(held.list.map((i) => i.path)).toEqual(['install.0']);
    expect(changes).toEqual([1, 2, 1]);
  });

  it('needs one removal per registration of the same issue', () => {
    const held = createHeldIssues();
    const issue = unappliedJsonIssue(['environment', 'config'], 'templates.editor.jsonBlock.unapplied');
    const off1 = held.register(issue);
    held.register(issue);
    off1();
    expect(held.list).toHaveLength(1);
  });

  it('is a blocking error at the field path', () => {
    expect(unappliedJsonIssue(['supportedEnvironments', 1, 'hostConfig'], 'k')).toEqual({
      path: 'supportedEnvironments.1.hostConfig',
      messageKey: 'k',
      severity: 'error',
      blocking: true,
    });
  });
});
