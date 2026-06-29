// SPDX-FileCopyrightText: 2026 Skaphos
// SPDX-License-Identifier: MIT
import type { Config } from 'tailwindcss'
import animate from 'tailwindcss-animate'

// shadcn/ui-compatible theme. CSS variables are defined in src/index.css and
// drive both light and dark themes. Status colors map to the canonical
// vocabulary in docs/design/ui-design-spec.md §3.1.
export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    container: {
      center: true,
      padding: '2rem',
      screens: { '2xl': '1400px' },
    },
    extend: {
      colors: {
        border: 'hsl(var(--border))',
        input: 'hsl(var(--input))',
        ring: 'hsl(var(--ring))',
        background: 'hsl(var(--background))',
        foreground: 'hsl(var(--foreground))',
        primary: {
          DEFAULT: 'hsl(var(--primary))',
          foreground: 'hsl(var(--primary-foreground))',
        },
        muted: {
          DEFAULT: 'hsl(var(--muted))',
          foreground: 'hsl(var(--muted-foreground))',
        },
        accent: {
          DEFAULT: 'hsl(var(--accent))',
          foreground: 'hsl(var(--accent-foreground))',
        },
        card: {
          DEFAULT: 'hsl(var(--card))',
          foreground: 'hsl(var(--card-foreground))',
        },
        // Canonical Keleustes status vocabulary (ui-design-spec §3.1).
        status: {
          healthy: 'hsl(var(--status-healthy))',
          progressing: 'hsl(var(--status-progressing))',
          degraded: 'hsl(var(--status-degraded))',
          drifted: 'hsl(var(--status-drifted))',
          blocked: 'hsl(var(--status-blocked))',
          frozen: 'hsl(var(--status-frozen))',
          missing: 'hsl(var(--status-missing))',
          failed: 'hsl(var(--status-failed))',
        },
      },
      borderRadius: {
        lg: 'var(--radius)',
        md: 'calc(var(--radius) - 2px)',
        sm: 'calc(var(--radius) - 4px)',
      },
      fontFamily: {
        mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'monospace'],
      },
    },
  },
  plugins: [animate],
} satisfies Config
