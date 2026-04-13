import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import mermaid from 'astro-mermaid';

export default defineConfig({
  site: 'https://posit-dev.github.io',
  base: '/team-operator',
  integrations: [
    starlight({
      title: 'Team Operator',
      logo: {
        src: './public/logo-posit-light.svg',
        alt: 'Posit',
      },
      customCss: [
        '@fontsource/open-sans/400.css',
        '@fontsource/open-sans/600.css',
        '@fontsource/open-sans/700.css',
        '@fontsource/source-code-pro/400.css',
        '@fontsource/source-code-pro/500.css',
        './src/assets/custom.css',
      ],
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/posit-dev/team-operator' },
      ],
      // Force light mode — no dark mode, matching VIP.
      // Must run before Starlight's theme JS by intercepting StarlightThemeProvider.
      head: [
        {
          tag: 'script',
          content: `
            localStorage.setItem('starlight-theme', 'light');
            document.documentElement.dataset.theme = 'light';
            window.StarlightThemeProvider = new Proxy({}, {
              get: () => () => { document.documentElement.dataset.theme = 'light'; }
            });
          `.trim(),
        },
      ],
      sidebar: [
        { label: 'Overview', slug: 'index' },
        {
          label: 'Getting Started',
          items: [
            { label: 'Installation', slug: 'guides/installation' },
            { label: 'AWS Deployment (EKS)', slug: 'guides/aws-deployment' },
            // AKS Deployment guide will be added when docs/guides/aks-deployment.md exists
            { label: 'Site Management', slug: 'guides/product-team-site-management' },
          ],
        },
        {
          label: 'Configuration',
          items: [
            { label: 'Authentication', slug: 'guides/authentication-setup' },
            { label: 'Workbench', slug: 'guides/workbench-configuration' },
            { label: 'Connect', slug: 'guides/connect-configuration' },
            { label: 'Package Manager', slug: 'guides/packagemanager-configuration' },
          ],
        },
        {
          label: 'Reference',
          items: [
            { label: 'Architecture', slug: 'architecture' },
            { label: 'API Reference', slug: 'api-reference' },
          ],
        },
        {
          label: 'Operations',
          items: [
            { label: 'Troubleshooting', slug: 'guides/troubleshooting' },
            { label: 'Upgrading', slug: 'guides/upgrading' },
          ],
        },
        {
          label: 'Contributing',
          items: [
            { label: 'Testing', slug: 'testing' },
            { label: 'Adding Config Options', slug: 'guides/adding-config-options' },
          ],
        },
      ],
    }),
    mermaid({
      autoTheme: true,
    }),
  ],
});
