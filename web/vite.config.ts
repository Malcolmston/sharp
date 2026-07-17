import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { fileURLToPath } from 'node:url';

// The sharp repo is served as a GitHub *project* page at
// https://malcolmston.github.io/sharp/, so assets must be based under /sharp/.
export default defineConfig({
  base: '/sharp/',
  plugins: [react()],
  resolve: {
    alias: {
      // Import the vendored shared library from source.
      'go-ui': fileURLToPath(new URL('./vendor/go/ui/src/index.ts', import.meta.url)),
    },
  },
  build: { outDir: 'dist', emptyOutDir: true },
});
