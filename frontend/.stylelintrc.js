/** @type {import('stylelint').Config} */
module.exports = {
  extends: [
    'stylelint-config-standard',
  ],
  rules: {
    // Tailwind CSS compatibility
    'at-rule-no-unknown': [
      true,
      {
        ignoreAtRules: [
          'tailwind',
          'apply',
          'variants',
          'responsive',
          'screen',
          'layer',
        ],
      },
    ],
    'function-no-unknown': [
      true,
      {
        ignoreFunctions: ['theme', 'screen'],
      },
    ],
    
    // CSS custom properties
    'custom-property-pattern': null,
    'selector-class-pattern': null,
    
    // Value patterns
    'keyframes-name-pattern': null,
    
    // Allow empty sources for CSS-in-JS
    'no-empty-source': null,
    
    // Disable rules that conflict with Tailwind
    'declaration-block-trailing-semicolon': null,
    'no-descending-specificity': null,
    
    // Property order
    'order/properties-alphabetical-order': null,
    
    // Units
    'unit-allowed-list': null,
    'length-zero-no-unit': true,
    
    // Colors
    'color-hex-case': 'lower',
    'color-hex-length': 'short',
    'color-no-invalid-hex': true,
    
    // Strings
    'string-quotes': 'single',
    
    // Numbers
    'number-leading-zero': 'never',
    'number-no-trailing-zeros': true,
    
    // Indentation and spacing
    'indentation': 2,
    'max-line-length': 100,
    'no-eol-whitespace': true,
    'no-missing-end-of-source-newline': true,
  },
  overrides: [
    {
      files: ['**/*.tsx', '**/*.jsx'],
      customSyntax: 'postcss-styled-syntax',
    },
  ],
  ignoreFiles: [
    '**/*.js',
    '**/*.jsx',
    '**/*.ts',
    '**/*.tsx',
    'node_modules/**',
    '.next/**',
    'build/**',
    'dist/**',
  ],
};