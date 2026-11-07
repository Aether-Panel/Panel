import { presetHead, presetVariable, runDefaults, type TemplatePreset } from './preset';

// From the community "minecraft" template, Paper only: Paper's own download (paperdl), Java on
// the host through javadl, eclipse-temurin in Docker (the Java variable picks its tag, which the
// create form keeps in step with the Minecraft version).

const JAVA_OPTIONS = ['8', '11', '17', '21', '25'].map((v) => ({ value: v, display: `Java ${v}` }));

const JVM_FLAGS = '-Xmx${memory}M -Dterminal.jline=false -Dterminal.ansi=true -Dlog4j2.formatMsgNoLookups=true';

export const minecraftPaper: TemplatePreset = {
  id: 'minecraft-paper',
  logo: 'minecraft-java',
  build: (t) => {
    const v = (key: string, spec: Parameters<typeof presetVariable>[3]) => presetVariable(t, 'minecraft-paper', key, spec);
    return {
      ...presetHead(t, 'minecraft-paper', 'minecraft-java', 'minecraft-java'),
      data: {
        eula: v('eula', { type: 'boolean', value: false, required: true, userEdit: true }),
        version: v('version', { type: 'string', value: 'latest', required: true, userEdit: true }),
        paperbuild: v('paperbuild', { type: 'string', value: 'latest', required: true, userEdit: true }),
        javaversion: v('javaversion', { type: 'option', value: '25', required: true, userEdit: true, options: JAVA_OPTIONS }),
        motd: v('motd', { type: 'string', value: 'A Minecraft Server', required: true, userEdit: true }),
        memory: v('memory', { type: 'integer', value: 2048, required: true, userEdit: false }),
        port: v('port', { type: 'integer', value: 25565, required: true, userEdit: false }),
      },
      install: [
        { if: "env == 'host'", type: 'javadl', version: '${javaversion}' },
        { type: 'paperdl', project: 'paper', minecraftVersion: '${version}', build: '${paperbuild}', target: 'server.jar' },
        { if: "!file_exists('server.properties')", type: 'writefile', target: 'server.properties', text: 'server-port=${port}\nmotd=${motd}\n' },
        { type: 'writefile', target: 'eula.txt', text: 'eula=${eula}\n' },
      ],
      run: {
        // on the host javadl installs java<version>; the Docker image has plain java
        command: [
          { if: "env == 'host'", command: `java\${javaversion} ${JVM_FLAGS} -jar server.jar nogui` },
          { command: `java ${JVM_FLAGS} -jar server.jar nogui` },
        ],
        stop: 'stop',
        ...runDefaults(),
      },
      environment: { type: 'host' },
      supportedEnvironments: [
        { type: 'host' },
        { type: 'docker', image: 'eclipse-temurin:${javaversion}', portBindings: ['0.0.0.0:${port}:${port}/tcp'] },
      ],
      stats: { type: 'jcmd' },
      query: { type: 'minecraft' },
    };
  },
};
