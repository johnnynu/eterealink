import { afterEach, describe, expect, it } from "vitest";
import { NextRequest } from "next/server";
import { proxy } from "./proxy";

const originalCanonicalHost = process.env.CANONICAL_HOST;
const originalLegacyHosts = process.env.LEGACY_HOSTS;

afterEach(() => {
  process.env.CANONICAL_HOST = originalCanonicalHost;
  process.env.LEGACY_HOSTS = originalLegacyHosts;
});

describe("canonical domain proxy", () => {
  it("preserves the path and query when redirecting a legacy hostname", () => {
    process.env.CANONICAL_HOST = "aurealink.app";
    process.env.LEGACY_HOSTS = "legacy.example,www.legacy.example";

    const request = new NextRequest("https://legacy.example/s/share-code?download=1", {
      headers: { host: "legacy.example" },
    });
    const response = proxy(request);

    expect(response.status).toBe(308);
    expect(response.headers.get("location")).toBe("https://aurealink.app/s/share-code?download=1");
  });

  it("continues normally for the canonical and service hostnames", () => {
    process.env.CANONICAL_HOST = "aurealink.app";
    process.env.LEGACY_HOSTS = "legacy.example";

    const request = new NextRequest("https://aurealink.app/app", {
      headers: { host: "aurealink.app" },
    });

    expect(proxy(request).headers.get("x-middleware-next")).toBe("1");
  });
});
