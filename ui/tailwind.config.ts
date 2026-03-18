import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./src/pages/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/components/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/app/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        // Sercha Palette C
        sercha: {
          snow: "#FAFAFA",
          mist: "#ECECEC",
          silverline: "#C7C8CC",
          "ink-slate": "#333F50",
          "fog-grey": "#7B8088",
          indigo: "#6675FF",
          "indigo-soft": "#E2E5FF",
        },
      },
      fontFamily: {
        sans: [
          "Chillax",
          "system-ui",
          "-apple-system",
          "BlinkMacSystemFont",
          "SF Pro Text",
          "sans-serif",
        ],
        mono: [
          "SF Mono",
          "ui-monospace",
          "Menlo",
          "Monaco",
          "Consolas",
          "monospace",
        ],
      },
      borderRadius: {
        "2xl": "1rem",
        "3xl": "1.5rem",
      },
    },
  },
  plugins: [],
};

export default config;
