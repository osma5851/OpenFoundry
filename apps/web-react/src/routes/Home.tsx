export function Home() {
  return (
    <section className="of-page">
      <header className="of-hero-strip">
        <p className="of-eyebrow">OpenFoundry</p>
        <h1 className="of-heading-xl">React shell ready</h1>
        <p className="of-text-muted">
          Port routes from <code>apps/web/src/routes</code> incrementally. Each Svelte page maps
          to a React component registered in <code>src/router.tsx</code>.
        </p>
      </header>
    </section>
  );
}
