import type { Config } from 'tailwindcss'

export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        brand: {
          50: '#FFF4ED', 100: '#FFE6D5', 200: '#FECDAA', 300: '#FDAC74',
          400: '#FB8B3C', 500: '#F97316', 600: '#EA5A0B', 700: '#C2440C',
        },
      },
      fontFamily: {
        sans: ['"Noto Sans SC"', 'system-ui', 'sans-serif'],
        serif: ['"Noto Serif SC"', 'serif'],
      },
      maxWidth: { container: '1140px' },
    },
  },
  plugins: [],
} satisfies Config
