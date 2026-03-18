"use client";

import { useMemo } from "react";

// Mock data - in real app, this comes from API
const mockChartData = [
  { date: "Dec 1", count: 450 },
  { date: "Dec 8", count: 620 },
  { date: "Dec 15", count: 780 },
  { date: "Dec 22", count: 950 },
  { date: "Dec 29", count: 1100 },
  { date: "Jan 5", count: 1234 },
];

export function DocumentsChart() {
  const { areaPath, linePath } = useMemo(() => {
    const max = Math.max(...mockChartData.map((d) => d.count));
    const padding = max * 0.1;
    const maxWithPadding = max + padding;

    const width = 100;
    const height = 100;

    const pts = mockChartData.map((d, i) => ({
      x: (i / (mockChartData.length - 1)) * width,
      y: height - (d.count / maxWithPadding) * height,
    }));

    // Create smooth curve path
    const line = pts.map((p, i) => (i === 0 ? `M ${p.x},${p.y}` : `L ${p.x},${p.y}`)).join(" ");

    // Area path (closed shape for gradient fill)
    const area = `${line} L ${width},${height} L 0,${height} Z`;

    return { areaPath: area, linePath: line };
  }, []);

  return (
    <div className="rounded-2xl border border-sercha-silverline bg-white p-6">
      <div className="mb-4 flex items-baseline justify-between">
        <div>
          <h3 className="text-sm font-medium text-sercha-fog-grey">Documents Indexed</h3>
          <p className="mt-1 text-3xl font-bold text-sercha-ink-slate">
            {mockChartData[mockChartData.length - 1].count.toLocaleString()}
          </p>
        </div>
        <span className="text-sm text-emerald-500">+12% this week</span>
      </div>

      {/* Chart */}
      <div className="relative h-40">
        <svg
          viewBox="0 0 100 100"
          preserveAspectRatio="none"
          className="h-full w-full"
        >
          <defs>
            <linearGradient id="chartGradient" x1="0" x2="0" y1="0" y2="1">
              <stop offset="0%" stopColor="#6675FF" stopOpacity="0.3" />
              <stop offset="100%" stopColor="#6675FF" stopOpacity="0" />
            </linearGradient>
          </defs>

          {/* Grid lines */}
          {[0, 25, 50, 75].map((y) => (
            <line
              key={y}
              x1="0"
              y1={y}
              x2="100"
              y2={y}
              stroke="#ECECEC"
              strokeWidth="0.5"
              vectorEffect="non-scaling-stroke"
            />
          ))}

          {/* Area fill */}
          <path d={areaPath} fill="url(#chartGradient)" />

          {/* Line */}
          <path
            d={linePath}
            fill="none"
            stroke="#6675FF"
            strokeWidth="2"
            vectorEffect="non-scaling-stroke"
            strokeLinecap="round"
            strokeLinejoin="round"
          />

        </svg>

        {/* X-axis labels */}
        <div className="absolute bottom-0 left-0 right-0 flex translate-y-6 justify-between text-xs text-sercha-fog-grey">
          {mockChartData.map((d, i) => (
            <span key={i}>{d.date}</span>
          ))}
        </div>
      </div>

      {/* Bottom padding for x-axis labels */}
      <div className="h-6" />
    </div>
  );
}
