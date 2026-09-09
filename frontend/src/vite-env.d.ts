/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** "1" → demo build'i (bkz. src/demo/, vite.config.ts `--mode demo`) */
  readonly VITE_DEMO?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
