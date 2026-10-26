import type { Metadata, Viewport } from "next";

import { SiteFooter } from "@/components/site/SiteFooter";
import { SiteHeader } from "@/components/site/SiteHeader";

import "./globals.css";

export const metadata: Metadata = {
  title: {
    default: "DepScope — dependency condition records",
    template: "%s · DepScope",
  },
  description:
    "Inspect an npm or PyPI package for known advisories, heuristic signals, coverage gaps, and the dependency path that connects them.",
  applicationName: "DepScope",
};

export const viewport: Viewport = {
  themeColor: "#16232b",
  colorScheme: "light",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className="h-full">
      <body className="flex min-h-full flex-col antialiased">
        <a
          href="#main"
          className="sr-only focus:not-sr-only focus:absolute focus:top-3 focus:left-3 focus:z-50 focus:border focus:border-ink focus:bg-paper focus:px-4 focus:py-2 focus:text-ink"
        >
          Skip to content
        </a>
        <SiteHeader />
        <main id="main" tabIndex={-1} className="flex-1">
          {children}
        </main>
        <SiteFooter />
      </body>
    </html>
  );
}
