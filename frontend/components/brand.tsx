import Link from "next/link";

export function Brand() {
  return (
    <Link className="brand" href="/" aria-label="Eterealink home">
      <span className="brand-mark" aria-hidden="true">
        <span />
        <span />
      </span>
      <span>Eterealink</span>
    </Link>
  );
}
