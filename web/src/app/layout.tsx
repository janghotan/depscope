import type { Metadata, Viewport } from "next";

import { SiteFooter } from "@/components/site/SiteFooter";
import { SiteHeader } from "@/components/site/SiteHeader";

import "./globals.css";

export const metadata: Metadata = {
  metadataBase: new URL(process.env.DEPSCOPE_SITE_URL ?? "http://localhost:3000"),
  title: {
    default: "DepScope — dependency condition records",
    template: "%s · DepScope",
  },
  description:
    "Inspect an npm or PyPI package for known advisories, heuristic signals, coverage gaps, and the dependency path that connects them.",
  applicationName: "DepScope",
  openGraph: {
    type: "website",
    siteName: "DepScope",
    title: "DepScope — dependency condition records",
    description:
      "Inspect package evidence, advisory matches, heuristic signals, and missing coverage before adding a dependency.",
    images: [{ url: "/opengraph-image", width: 1200, height: 630, alt: "DepScope dependency inspection record" }],
  },
  twitter: {
    card: "summary_large_image",
    title: "DepScope — dependency condition records",
    description:
      "Inspect package evidence, advisory matches, heuristic signals, and missing coverage before adding a dependency.",
    images: ["/opengraph-image"],
  },
};

export const viewport: Viewport = {
  themeColor: "#16232b",
  colorScheme: "dark",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className="h-full" data-scroll-behavior="smooth">
      <body className="flex min-h-full flex-col antialiased">
        <a
          href="#main"
          className="sr-only focus:not-sr-only focus:absolute focus:top-3 focus:left-3 focus:z-50 focus:border focus:border-rule-strong focus:bg-sheet focus:px-4 focus:py-2 focus:text-ink"
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
