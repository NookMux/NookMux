import type { KnipConfig } from 'knip';

const config: KnipConfig = {
  ignore: [
    'src/components/ui/**',
    'src/i18n/static-keys.ts',
    '**/*.test.{ts,tsx}',
  ],
  ignoreDependencies: ['@tanstack/table-core'],
};

export default config;
