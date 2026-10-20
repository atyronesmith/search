import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// Replaces react-scripts (Create React App). Kept compatible with the
// Electron wrapper in public/: dev server on 3000, production build in
// build/, relative asset paths because the app is loaded over file://.
export default defineConfig({
  plugins: [react()],
  base: './',
  server: { port: 3000, strictPort: true },
  build: { outDir: 'build', sourcemap: true, chunkSizeWarningLimit: 4000 },
});
