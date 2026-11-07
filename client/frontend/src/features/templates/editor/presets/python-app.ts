import { presetHead, presetVariable, runDefaults, type TemplatePreset } from './preset';

// From the community "python" template without the git options: a virtual environment in .venv,
// pip install -r requirements.txt when there is one, and `python <start file>` to run. The
// official python image in Docker; on the host python<version> when installed, else python3.

const PICK_PYTHON = 'PY=python3; if command -v \\"python${pyversion}\\" >/dev/null 2>&1; then PY=\\"python${pyversion}\\"; fi;';

const PLACEHOLDER =
  '# Placeholder written by Aether Panel. Replace this file with your app.\nimport platform\n\nprint("Aether Panel: Python " + platform.python_version() + " is running ${startfile}. Upload your app.")\n';

export const pythonApp: TemplatePreset = {
  id: 'python',
  logo: 'python',
  build: (t) => {
    const v = (key: string, spec: Parameters<typeof presetVariable>[3]) => presetVariable(t, 'python', key, spec);
    return {
      ...presetHead(t, 'python', 'python', 'python'),
      data: {
        pyversion: v('pyversion', {
          type: 'option',
          value: '3.13',
          required: true,
          userEdit: true,
          options: ['3.12', '3.13', '3.14'].map((version) => ({ value: version, display: `Python ${version}` })),
        }),
        startfile: v('startfile', { type: 'string', value: 'main.py', required: true, userEdit: true }),
        port: v('port', { type: 'integer', value: 8000, required: true, userEdit: false }),
      },
      install: [
        { if: '!file_exists(startfile)', type: 'writefile', target: '${startfile}', text: PLACEHOLDER },
        {
          type: 'command',
          commands: [
            `bash -c "set -e; ${PICK_PYTHON} if [ ! -x .venv/bin/python ]; then $PY -m venv .venv; fi; if [ -f requirements.txt ]; then .venv/bin/python -m pip install -q -r requirements.txt; fi"`,
          ],
        },
      ],
      run: {
        command: 'bash -c "exec .venv/bin/python -u \\"${startfile}\\""',
        stopCode: 2,
        environmentVars: { PORT: '${port}', PYTHONUNBUFFERED: '1' },
        ...runDefaults(),
      },
      environment: { type: 'host' },
      supportedEnvironments: [
        { type: 'host' },
        { type: 'docker', image: 'python:${pyversion}-slim', portBindings: ['0.0.0.0:${port}:${port}/tcp'] },
      ],
      requirements: { os: 'linux', binaries: ['bash', 'python3'] },
    };
  },
};
