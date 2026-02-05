"use client";

import { useState, useCallback, useEffect } from "react";
import { Header } from "@/components/layout/header";
import { ToastProvider } from "@/components/ui/toast";
import { AddDownloadDialog } from "@/components/add-download-dialog";
import { ErrorBoundary } from "@/components/error-boundary";

interface ClientLayoutProps {
  children: React.ReactNode;
}

// Create a context for refresh callback
import { createContext, useContext } from "react";

interface RefreshContextType {
  refresh: () => void;
  registerRefresh: (callback: () => void) => void;
}

export const RefreshContext = createContext<RefreshContextType>({
  refresh: () => {},
  registerRefresh: () => {},
});

export function useRefresh() {
  return useContext(RefreshContext);
}

export function ClientLayout({ children }: ClientLayoutProps) {
  const [dialogOpen, setDialogOpen] = useState(false);
  const [refreshCallback, setRefreshCallback] = useState<(() => void) | null>(null);

  const registerRefresh = useCallback((callback: () => void) => {
    setRefreshCallback(() => callback);
  }, []);

  const refresh = useCallback(() => {
    refreshCallback?.();
  }, [refreshCallback]);

  // Suppress Next.js error overlay for expected network errors (server offline)
  // This runs once on mount and applies globally to the entire application
  useEffect(() => {
    if (typeof window === "undefined" || process.env.NODE_ENV !== "development") {
      return;
    }

    // Override console.error to filter out expected errors
    const originalConsoleError = console.error;
    console.error = (...args: unknown[]) => {
      const errorMessage = String(args[0]);

      // Suppress proxy/network errors when backend is offline (expected state)
      if (
        errorMessage.includes("ECONNREFUSED") ||
        errorMessage.includes("Failed to proxy") ||
        errorMessage.includes("fetch failed") ||
        errorMessage.includes("ENOTFOUND") ||
        errorMessage.includes("network error")
      ) {
        // Log to debug console instead (won't trigger error overlay)
        console.debug("Backend server offline (expected):", ...args);
        return;
      }

      // Pass through other errors normally
      originalConsoleError.apply(console, args);
    };

    // Also suppress unhandled promise rejections from network errors
    const handleUnhandledRejection = (event: PromiseRejectionEvent) => {
      const reason = String(event.reason);
      if (
        reason.includes("ECONNREFUSED") ||
        reason.includes("Failed to proxy") ||
        reason.includes("fetch failed") ||
        reason.includes("ENOTFOUND") ||
        reason.includes("network error")
      ) {
        event.preventDefault(); // Prevent error overlay
        console.debug("Unhandled rejection (backend offline):", event.reason);
      }
    };

    window.addEventListener("unhandledrejection", handleUnhandledRejection);

    // Cleanup: restore original handlers on unmount
    return () => {
      console.error = originalConsoleError;
      window.removeEventListener("unhandledrejection", handleUnhandledRejection);
    };
  }, []); // Empty dependency array = runs once on mount

  return (
    <ErrorBoundary>
      <ToastProvider>
        <RefreshContext.Provider value={{ refresh, registerRefresh }}>
          <div className="relative min-h-screen flex flex-col">
            <Header onAddDownload={() => setDialogOpen(true)} />
            <main className="flex-1 container mx-auto px-4 py-6">{children}</main>
            <AddDownloadDialog
              open={dialogOpen}
              onOpenChange={setDialogOpen}
              onSuccess={refresh}
            />
          </div>
        </RefreshContext.Provider>
      </ToastProvider>
    </ErrorBoundary>
  );
}

