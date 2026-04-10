import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import mermaid from 'astro-mermaid';

export default defineConfig({
  site: 'https://posit-dev.github.io',
  base: '/team-operator',
  integrations: [
    starlight({
      title: 'Team Operator',
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
      sidebar: [
        { label: 'Overview', slug: 'index' },
        { label: 'Architecture', slug: 'architecture' },
        { label: 'API Reference', slug: 'api-reference' },
        {
          label: 'Guides',
          items: [
            { label: 'Site Management', slug: 'guides/product-team-site-management' },
            // AKS Deployment guide will be added when docs/guides/aks-deployment.md exists (PR 1)
            { label: 'Authentication', slug: 'guides/authentication-setup' },
            { label: 'Workbench', slug: 'guides/workbench-configuration' },
            { label: 'Connect', slug: 'guides/connect-configuration' },
            { label: 'Package Manager', slug: 'guides/packagemanager-configuration' },
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
