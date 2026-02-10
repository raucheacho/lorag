/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        // Drizzle-inspired dark theme
        bg: {
          DEFAULT: '#0f0f10',
          secondary: '#18181b',
          tertiary: '#27272a',
        },
        border: {
          DEFAULT: '#27272a',
          strong: '#3f3f46',
        },
        text: {
          DEFAULT: '#fafafa',
          secondary: '#a1a1aa',
          tertiary: '#71717a',
        },
        accent: {
          DEFAULT: '#10b981', // Emerald green
          hover: '#059669',
          muted: 'rgba(16, 185, 129, 0.1)',
        },
        status: {
          success: '#22c55e',
          warning: '#f59e0b',
          error: '#ef4444',
          info: '#3b82f6',
        }
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'monospace'],
      },
      fontSize: {
        '2xs': '0.625rem',
      },
    },
  },
  plugins: [],
}
