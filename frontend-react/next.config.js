/** @type {import('next').NextConfig} */
const nextConfig = {
  // For production deployment on admin.wpgc.church
  basePath: '',
  images: {
    domains: ['uploads.wpgc.church', 'wpgc.church'],
  },
  async rewrites() {
    // Proxy API calls to the upload server
    return [
      {
        source: '/api/upload/:path*',
        destination: 'https://uploads.wpgc.church/api/:path*',
      },
      {
        source: '/api/:path*',
        destination: process.env.NEXT_PUBLIC_API_URL 
          ? `${process.env.NEXT_PUBLIC_API_URL}/api/:path*`
          : 'http://localhost:8000/api/:path*',
      },
    ];
  },
  async headers() {
    return [
      {
        source: '/:path*',
        headers: [
          {
            key: 'X-Frame-Options',
            value: 'SAMEORIGIN',
          },
        ],
      },
    ];
  },
};

module.exports = nextConfig;