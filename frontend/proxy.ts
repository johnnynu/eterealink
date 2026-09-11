import { NextRequest, NextResponse } from "next/server";

function configuredHosts(value: string | undefined) {
  return new Set((value ?? "")
    .split(",")
    .map((host) => host.trim().toLowerCase())
    .filter(Boolean));
}

export function proxy(request: NextRequest) {
  const canonicalHost = process.env.CANONICAL_HOST?.trim().toLowerCase();
  const requestHost = request.headers.get("host")?.split(":", 1)[0].toLowerCase();
  if (!canonicalHost || !requestHost || !configuredHosts(process.env.LEGACY_HOSTS).has(requestHost)) {
    return NextResponse.next();
  }

  const destination = request.nextUrl.clone();
  destination.protocol = "https:";
  destination.host = canonicalHost;
  return NextResponse.redirect(destination, 308);
}
