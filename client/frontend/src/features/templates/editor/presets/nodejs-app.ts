import { presetHead, presetVariable, runDefaults, type TemplatePreset } from './preset';

// From the community "nodejs" template without the git and package-manager options: Node.js on
// the host through nodejsdl, the official node image in Docker, npm install when there is a
// package.json and `node <start file>` to run.

// nodejsdl installs node<version> and npm<version> on the host; npm needs `node` on the PATH, so
// the folder they point to goes first. In Docker there is no node<version> and plain node is used.
const NODE_ON_PATH =
  'N=\\"$(command -v node${nodeversion} 2>/dev/null || true)\\"; if [ -n \\"$N\\" ]; then export PATH=\\"$(dirname \\"$(readlink -f \\"$N\\")\\"):$PATH\\"; fi;';

const PLACEHOLDER =
  '// Placeholder written by Aether Panel. Replace this file with your app.\nconsole.log("Aether Panel: Node.js " + process.version + " is running ${startfile}. Upload your app.");\n';

export const nodejsApp: TemplatePreset = {
  id: 'nodejs',
  logo: 'nodejs',
  build: (t) => {
    const v = (key: string, spec: Parameters<typeof presetVariable>[3]) => presetVariable(t, 'nodejs', key, spec);
    return {
      ...presetHead(t, 'nodejs', 'nodejs', 'nodejs'),
      data: {
        nodeversion: v('nodeversion', {
          type: 'option',
          value: '24',
          required: true,
          userEdit: true,
          options: [
            { value: '22', display: 'Node.js 22 (LTS)' },
            { value: '24', display: 'Node.js 24 (LTS)' },
          ],
        }),
        startfile: v('startfile', { type: 'string', value: 'index.js', required: true, userEdit: true }),
        port: v('port', { type: 'integer', value: 3000, required: true, userEdit: false }),
      },
      install: [
        { if: "env == 'host'", type: 'nodejsdl', version: '${nodeversion}' },
        { if: '!file_exists(startfile)', type: 'writefile', target: '${startfile}', text: PLACEHOLDER },
        { if: "file_exists('package.json')", type: 'command', commands: [`bash -c "set -e; ${NODE_ON_PATH} npm install --no-audit --no-fund"`] },
      ],
      run: {
        command: `bash -c "${NODE_ON_PATH} exec node \\"\${startfile}\\""`,
        stopCode: 15,
        environmentVars: { PORT: '${port}', NODE_ENV: 'production' },
        ...runDefaults(),
      },
      environment: { type: 'host' },
      supportedEnvironments: [
        { type: 'host' },
        { type: 'docker', image: 'node:${nodeversion}', portBindings: ['0.0.0.0:${port}:${port}/tcp'], hostConfig: { Init: true } },
      ],
      requirements: { os: 'linux', binaries: ['bash'] },
    };
  },
};
