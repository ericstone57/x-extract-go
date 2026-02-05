"use client";

import React from "react";

interface ErrorBoundaryProps {
  children: React.ReactNode;
}

interface ErrorBoundaryState {
  hasError: boolean;
  error: Error | null;
}

/**
 * Error boundary that catches React errors and prevents the Next.js error overlay
 * from showing for expected network errors (backend server offline).
 */
export class ErrorBoundary extends React.Component<
  ErrorBoundaryProps,
  ErrorBoundaryState
> {
  constructor(props: ErrorBoundaryProps) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    const errorMessage = error.message || String(error);

    // Check if this is a network error (backend server offline)
    const isNetworkError =
      errorMessage.includes("ECONNREFUSED") ||
      errorMessage.includes("Failed to proxy") ||
      errorMessage.includes("fetch failed") ||
      errorMessage.includes("ENOTFOUND") ||
      errorMessage.includes("network error") ||
      errorMessage.includes("NetworkError");

    if (isNetworkError) {
      // Suppress the error - don't show error overlay
      console.debug("Network error caught by error boundary (expected):", error);
      // Reset the error state so the app continues to work
      this.setState({ hasError: false, error: null });
    } else {
      // For other errors, log them normally
      console.error("Error caught by error boundary:", error, errorInfo);
    }
  }

  render() {
    if (this.state.hasError && this.state.error) {
      // Only show error UI for non-network errors
      const errorMessage = this.state.error.message || String(this.state.error);
      const isNetworkError =
        errorMessage.includes("ECONNREFUSED") ||
        errorMessage.includes("Failed to proxy") ||
        errorMessage.includes("fetch failed") ||
        errorMessage.includes("ENOTFOUND") ||
        errorMessage.includes("network error");

      if (isNetworkError) {
        // Don't show error UI for network errors - just render children normally
        return this.props.children;
      }

      // Show error UI for other errors
      return (
        <div className="flex items-center justify-center min-h-screen p-4">
          <div className="max-w-md w-full bg-destructive/10 border border-destructive rounded-lg p-6">
            <h2 className="text-lg font-semibold text-destructive mb-2">
              Something went wrong
            </h2>
            <p className="text-sm text-muted-foreground mb-4">
              {this.state.error.message}
            </p>
            <button
              onClick={() => this.setState({ hasError: false, error: null })}
              className="px-4 py-2 bg-primary text-primary-foreground rounded-md hover:bg-primary/90"
            >
              Try again
            </button>
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}

