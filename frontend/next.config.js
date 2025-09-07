/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'export',
  reactStrictMode: true,
  swcMinify: true,
  experimental: {
    serverComponentsExternalPackages: [],
  },
  webpack: (config, { isServer }) => {
    // Disable webpack cache to avoid snapshot errors
    config.cache = false;
    return config;
  },
}

module.exports = nextConfig