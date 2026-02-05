import type { Metadata } from "next";
import { Inter } from "next/font/google";
import "./globals.css";
import { ClientLayout } from "./client-layout";

const inter = Inter({
  subsets: ["latin"],
  variable: "--font-inter",
});

export const metadata: Metadata = {
  title: "X-Extract Dashboard",
  description: "Download manager for X/Twitter and Telegram media",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        {/* Suppress Next.js error overlay for network errors in development */}
        {process.env.NODE_ENV === "development" && (
          <script
            dangerouslySetInnerHTML={{
              __html: `
                (function() {
                  if (typeof window !== 'undefined') {
                    // Intercept Next.js error overlay
                    const originalError = window.console.error;
                    window.console.error = function(...args) {
                      const msg = String(args[0]);
                      if (msg.includes('ECONNREFUSED') ||
                          msg.includes('Failed to proxy') ||
                          msg.includes('fetch failed') ||
                          msg.includes('ENOTFOUND')) {
                        console.debug('Backend offline (expected):', ...args);
                        return;
                      }
                      originalError.apply(console, args);
                    };
                  }
                })();
              `,
            }}
          />
        )}
      </head>
      <body className={`${inter.variable} font-sans antialiased`}>
        <ClientLayout>{children}</ClientLayout>
      </body>
    </html>
  );
}

