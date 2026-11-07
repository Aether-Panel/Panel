import { presetHead, presetVariable, runDefaults, type TemplatePreset } from './preset';

// From the community Steam games that use the project's steamcmd image (palworld, corekeeper,
// necesse…): steamgamedl on the host, steamcmd itself in Docker, each step behind its env
// condition. The app id and the file that starts the server are the author's to fill in.

const STEAMCMD_IN_DOCKER =
  'bash -c "for try in 1 2 3 4 5; do steamcmd +@sSteamCmdForcePlatformType linux +force_install_dir /SkyPanel +login anonymous +app_update ${appid} validate +quit && exit 0; ' +
  "echo 'steamcmd could not finish the download, trying again in 15s ($try/5)'; sleep 15; done; exit 1\"";

export const steamGame: TemplatePreset = {
  id: 'steam',
  build: (t) => {
    const v = (key: string, spec: Parameters<typeof presetVariable>[3]) => presetVariable(t, 'steam', key, spec);
    return {
      ...presetHead(t, 'steam', 'steam'),
      data: {
        appid: v('appid', { type: 'string', value: '', required: true, userEdit: false }),
        executable: v('executable', { type: 'string', value: '', required: true, userEdit: false }),
        args: v('args', { type: 'string', value: '', required: false, userEdit: true }),
        port: v('port', { type: 'integer', value: 27015, required: true, userEdit: false }),
      },
      install: [
        { if: "env == 'host'", type: 'steamgamedl', appId: '${appid}' },
        { if: "env == 'docker'", type: 'command', commands: [STEAMCMD_IN_DOCKER] },
      ],
      run: {
        command: './${executable} ${args}',
        stopCode: 2,
        ...runDefaults(),
      },
      environment: { type: 'host' },
      supportedEnvironments: [
        { type: 'host' },
        { type: 'docker', image: 'ghcr.io/aether-panel/steamcmd', portBindings: ['0.0.0.0:${port}:${port}/udp'] },
      ],
      requirements: { os: 'linux', arch: 'amd64' },
    };
  },
};
