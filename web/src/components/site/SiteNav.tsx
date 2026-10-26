"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const links = [
  { href: "/scan", label: "Scan" },
  { href: "/compare", label: "Compare" },
  { href: "/demo", label: "Demo" },
  { href: "/about", label: "About" },
] as const;

export function SiteNav() {
  const pathname = usePathname();

  return (
    <nav aria-label="Primary" className="w-full sm:w-auto">
      <ul className="flex items-stretch gap-1 sm:gap-2">
        {links.map((link) => {
          const active =
            pathname === link.href || pathname.startsWith(`${link.href}/`);
          return (
            <li key={link.href} className="flex-1 sm:flex-none">
              <Link
                href={link.href}
                aria-current={active ? "page" : undefined}
                className={`rule-label flex min-h-10 items-center justify-center border-b-2 px-2.5 py-2 transition-colors sm:px-3 ${
                  active
                    ? "border-safety text-paper"
                    : "border-transparent text-[#c6cfd2] hover:border-shell-line hover:text-paper"
                }`}
              >
                {link.label}
              </Link>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
