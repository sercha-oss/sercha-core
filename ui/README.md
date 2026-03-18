# Sercha Admin UI

Admin interface for Sercha Core.

## Development

```bash
npm install
npm run dev
```

Open [http://localhost:3001](http://localhost:3001)

## Scripts

| Script | Description |
|--------|-------------|
| `npm run dev` | Start development server |
| `npm run build` | Build for production (static export) |
| `npm run lint` | Run ESLint |
| `npm run format` | Format code with Prettier |
| `npm run typecheck` | Run TypeScript type checking |
| `npm run test` | Run tests with Vitest |

## Tech Stack

- Next.js 15 (App Router, Static Export)
- React 19
- TypeScript
- Tailwind CSS
- Framer Motion
- shadcn/ui patterns

## Build Output

Production build outputs static files to `out/` directory, ready for nginx or any static host.
