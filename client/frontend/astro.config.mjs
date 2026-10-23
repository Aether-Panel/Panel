import { defineConfig } from 'astro/config';
import react from '@astrojs/react';
import tailwind from '@astrojs/tailwind';
import { fileURLToPath } from 'node:url';
import { createRequire } from 'node:module';
import path from 'node:path';

// monaco-editor's export map only exposes its ESM modules, so point at the stylesheet of
// its prebuilt copy directly. That file embeds the codicon font, so no other asset is needed.
const require = createRequire(import.meta.url);
const monacoMinDir = path.dirname(require.resolve('monaco-editor'));

export default defineConfig({
    integrations: [
        react(),
        tailwind({
            applyBaseStyles: false,
        }),
    ],
    vite: {
        resolve: {
            alias: [
                { find: '@', replacement: fileURLToPath(new URL('./src', import.meta.url)) },
                // a regex keeps the ?raw suffix, which a plain string alias does not match
                { find: /^monaco-editor-main-css(\?.*)?$/, replacement: path.join(monacoMinDir, 'editor/editor.main.css') + '$1' },
            ],
        },
        optimizeDeps: {
            include: ['lucide-react', 'react', 'react-dom']
        },
        server: {
            fs: {
                allow: ['..'],
            },
        },
    },
});
