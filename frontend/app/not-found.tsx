import Link from "next/link";

export default function NotFound() {
  return (
    <div className="share-page centered-page">
      <section className="share-card unavailable-card">
        <span className="unavailable-mark" aria-hidden="true">404</span>
        <p className="eyebrow">Lost in transit</p>
        <h1>That page isn’t here</h1>
        <p>The address may be incomplete, or the page may have moved.</p>
        <Link className="primary-button" href="/">Return home</Link>
      </section>
    </div>
  );
}
