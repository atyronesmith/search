import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// Replaces react-scripts (Create React App). Kept compatible with the
// Electron wrapper: dev server on 3000 (electron/main.js and the `dev`
// script expect it), production build in build/ (electron-builder's
// `files` and main.js's loadURL), relative asset paths because the app is
// loaded over file://.
export default defineConfig({
  plugins: [react()],
  base: './',
  server: { port: 3000, strictPort: true },
  // One bundle is fine for an app loaded from disk; the 500 kB warning is
  // web-page advice.
  build: { outDir: 'build', sourcemap: true, chunkSizeWarningLimit: 4000 },
});
