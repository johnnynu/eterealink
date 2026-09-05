import Link from "next/link";
import { PlantIcon } from "@phosphor-icons/react/ssr";
import type { MouseEventHandler } from "react";

export function Brand({ href = "/", onClick }: { href?: string; onClick?: MouseEventHandler<HTMLAnchorElement> }) {
  return (
    <Link className="brand" href={href} aria-label="Eterealink home" onClick={onClick}>
      <span className="brand-mark" aria-hidden="true">
        <PlantIcon size={31} weight="duotone" />
      </span>
      <span>Eterealink</span>
    </Link>
  );
}
