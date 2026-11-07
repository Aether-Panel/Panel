import { presetHead, presetVariable, runDefaults, type TemplatePreset } from './preset';

// The least a Docker template needs: the project's generic image (bash, curl, git, unzip…), a
// port, and a start.sh the author replaces with what the server runs.

const PLACEHOLDER = '#!/bin/bash\n# Written by Aether Panel: replace this with what starts your server.\necho "Aether Panel: edit start.sh to start your server."\n';

export const dockerGeneric: TemplatePreset = {
  id: 'docker',
  build: (t) => ({
    ...presetHead(t, 'docker', 'custom'),
    data: {
      port: presetVariable(t, 'docker', 'port', { type: 'integer', value: 8080, required: true, userEdit: false }),
    },
    install: [{ if: "!file_exists('start.sh')", type: 'writefile', target: 'start.sh', text: PLACEHOLDER }],
    run: {
      command: 'bash start.sh',
      stopCode: 15,
      environmentVars: { PORT: '${port}' },
      ...runDefaults(),
    },
    environment: { type: 'docker', image: 'ghcr.io/aether-panel/generic' },
    supportedEnvironments: [{ type: 'docker', image: 'ghcr.io/aether-panel/generic', portBindings: ['0.0.0.0:${port}:${port}/tcp'] }],
  }),
};
