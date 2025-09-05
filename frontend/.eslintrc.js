/** @type {import('eslint').Linter.Config} */
module.exports = {
  extends: [
    'next/core-web-vitals',
  ],
  env: {
    browser: true,
    es2021: true,
    node: true,
    jest: true,
  },
  rules: {
    // Basic rules to keep code clean
    'no-console': 'warn',
    'no-debugger': 'error',
    'no-unused-vars': 'warn',
  },
  ignorePatterns: [
    '.next',
    'node_modules',
    'dist',
    'build',
    'coverage',
    '*.config.js',
    'public',
  ],
};