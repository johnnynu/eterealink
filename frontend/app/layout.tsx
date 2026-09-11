import type { Metadata } from "next";
import { AuthProvider } from "@/components/auth-context";
import { SiteHeader } from "@/components/site-header";
import "./globals.css";

export const metadata: Metadata = {
	metadataBase: new URL("https://aurealink.app"),
  title: { default: "Aurea Link — Share a file simply", template: "%s · Aurea Link" },
  description: "Share files without an account using 24-hour links. Sign in for a private file library, folders, collaboration, and control over your share links.",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>
        <AuthProvider>
          <div className="site-shell">
            <SiteHeader />
            <main>{children}</main>
            <footer>
              <span>© {new Date().getFullYear()} Aurea Link</span>
              <span>Private by design · Temporary by default</span>
            </footer>
          </div>
        </AuthProvider>
      </body>
    </html>
  );
}
