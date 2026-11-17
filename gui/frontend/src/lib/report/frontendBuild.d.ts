// Set by vite.config.ts's define: the installed version of each runtime
// package, and the Node that built the bundle.
declare const __FRONTEND_BUILD__: {
  node: string;
  packages: Record<string, string>;
};
