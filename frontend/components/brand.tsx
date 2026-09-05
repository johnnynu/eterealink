import Link from "next/link";
import { PlantIcon } from "@phosphor-icons/react/ssr";

export function Brand({ href = "/" }: { href?: string }) {
  return (
    <Link className="brand" href={href} aria-label="Eterealink home">
      <span className="brand-mark" aria-hidden="true">
        <PlantIcon size={31} weight="duotone" />
      </span>
      <span>Eterealink</span>
    </Link>
  );
}
