/** @type {import('prettier').Config} */
module.exports = {
  // Core formatting
  printWidth: 100,
  tabWidth: 2,
  useTabs: false,
  semi: true,
  singleQuote: true,
  quoteProps: 'as-needed',
  
  // JSX formatting
  jsxSingleQuote: true,
  
  // Trailing commas
  trailingComma: 'all',
  
  // Spacing
  bracketSpacing: true,
  bracketSameLine: false,
  arrowParens: 'avoid',
  
  // Line endings
  endOfLine: 'lf',
  
  // Embedded formatting
  embeddedLanguageFormatting: 'auto',
  
  // HTML formatting
  htmlWhitespaceSensitivity: 'css',
  
  // Plugins
  plugins: ['prettier-plugin-tailwindcss'],
  
  // Plugin-specific options
  tailwindConfig: './tailwind.config.js',
  tailwindFunctions: ['clsx', 'cn'],
  
  // File-specific overrides
  overrides: [
    {
      files: '*.json',
      options: {
        tabWidth: 2,
      },
    },
    {
      files: '*.md',
      options: {
        printWidth: 80,
        proseWrap: 'preserve',
      },
    },
    {
      files: '*.yaml',
      options: {
        tabWidth: 2,
        singleQuote: false,
      },
    },
  ],
};