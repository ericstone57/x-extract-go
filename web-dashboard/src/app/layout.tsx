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
        {/* Apply saved theme before first paint to avoid flash */}
        <script
          dangerouslySetInnerHTML={{
            __html: `(function(){try{var t=localStorage.getItem('theme');if(t==='dark'||(t===null&&window.matchMedia('(prefers-color-scheme:dark)').matches)){document.documentElement.classList.add('dark');}}catch(e){}})();`,
          }}
        />
        {/* Suppress Next.js error overlay for network errors in development */}
        {process.env.NODE_ENV === "development" && (
          <script
            dangerouslySetInnerHTML={{
              __html: `
                (function() {
                  if (typeof window !== 'undefined') {
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

