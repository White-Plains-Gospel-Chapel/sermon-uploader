/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'export',
  reactStrictMode: true,
  swcMinify: true,
  experimental: {
    serverComponentsExternalPackages: [],
  },
}

module.exports = nextConfig