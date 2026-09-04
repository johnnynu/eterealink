import type { Metadata } from "next";
import Link from "next/link";
import { Brand } from "@/components/brand";
import { AuthMenu } from "@/components/auth-menu";
import { AuthProvider } from "@/components/auth-context";
import "./globals.css";

export const metadata: Metadata = {
  title: { default: "Eterealink — Share a file simply", template: "%s · Eterealink" },
  description: "Send a file with a private link that expires automatically after 24 hours.",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>
        <AuthProvider>
          <div className="site-shell">
            <header className="site-header">
              <Brand />
              <nav aria-label="Primary navigation">
                <span className="nav-note">No account needed</span>
                <Link className="nav-link" href="/">Send a file</Link>
                <AuthMenu />
              </nav>
            </header>
            <main>{children}</main>
            <footer>
              <span>© {new Date().getFullYear()} Eterealink</span>
              <span>Private by design · Temporary by default</span>
            </footer>
          </div>
        </AuthProvider>
      </body>
    </html>
  );
}
