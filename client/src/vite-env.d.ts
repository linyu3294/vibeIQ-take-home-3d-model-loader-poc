/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly NODE_ENV: string;
  readonly VITE_APP_TITLE: string;
  readonly VITE_API_HTTPS_URL: string;
  readonly VITE_API_WEBSOCKET_URL: string;
  readonly VITE_API_KEY: string;
  readonly VITE_API_GATEWAY_URL: string;
  // Add other environment variables here
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}