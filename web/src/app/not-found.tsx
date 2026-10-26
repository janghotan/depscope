import Link from "next/link";

export default function NotFound() {
  return (
    <div className="record-container py-12 lg:py-16">
      <div className="max-w-2xl border-t-2 border-ink pt-5">
        <p className="rule-label text-ink-soft">Reference 404 · no record</p>
        <h1 className="mt-3 text-3xl leading-tight font-bold tracking-[-0.02em] sm:text-4xl">
          No page exists at this address.
        </h1>
        <p className="mt-3 max-w-xl text-sm leading-6 text-ink-soft">
          The address may be incomplete or the specimen may not exist. No scan
          was run and no condition record was produced.
        </p>
        <div className="mt-6 flex flex-wrap items-center gap-3">
          <Link href="/scan" className="button-primary text-sm">
            Scan a package
          </Link>
          <Link href="/demo" className="button-secondary text-sm">
            Open the demo specimens
          </Link>
          <Link href="/" className="link-quiet text-sm">
            Back to the inspection desk
          </Link>
        </div>
      </div>
    </div>
  );
}
