const isDev = process.env.NODE_ENV === 'development'

/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'export',
  assetPrefix: isDev ? undefined : 'https://remix.sentio.xyz',
  images: {
    unoptimized: true,
  },
};

export default nextConfig;
