"use client";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { useEffect, useState } from "react";
import { useServerHealth } from "@/hooks/use-server-health";
import { Download, Moon, Sun, Plus, AlertCircle, CheckCircle } from "lucide-react";

interface HeaderProps {
  onAddDownload: () => void;
}

export function Header({ onAddDownload }: HeaderProps) {
  const [isDark, setIsDark] = useState(false);
  const serverOnline = useServerHealth();

  // Sync with whatever the anti-flash script already applied
  useEffect(() => {
    setIsDark(document.documentElement.classList.contains("dark"));
  }, []);

  const toggleTheme = () => {
    const next = !isDark;
    document.documentElement.classList.toggle("dark", next);
    setIsDark(next);
    try { localStorage.setItem("theme", next ? "dark" : "light"); } catch {}
  };

  return (
    <header className="sticky top-0 z-40 w-full border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div className="container mx-auto px-4 flex h-14 items-center gap-4">
        {/* Logo */}
        <div className="flex items-center gap-2 font-bold">
          <Download className="h-5 w-5" />
          <span>X-Extract</span>
        </div>

        <div className="flex flex-1 items-center justify-end gap-2">
          {/* Server status */}
          <div className={cn(
            "flex items-center gap-1.5 text-sm",
            serverOnline === null ? "text-muted-foreground" :
            serverOnline ? "text-green-600 dark:text-green-500" :
            "text-destructive"
          )}>
            {serverOnline === null ? (
              <div className="h-2 w-2 rounded-full bg-muted-foreground animate-pulse" />
            ) : serverOnline ? (
              <CheckCircle className="h-4 w-4" />
            ) : (
              <AlertCircle className="h-4 w-4" />
            )}
            <span className="hidden sm:inline font-medium">
              {serverOnline === null ? "Checking…" : serverOnline ? "Server Running" : "Server Stopped"}
            </span>
          </div>

          <Button
            size="sm"
            onClick={onAddDownload}
            disabled={!serverOnline}
            title={serverOnline ? "Add download" : "Server is not running"}
          >
            <Plus className="h-4 w-4 mr-1" />
            Add Download
          </Button>

          <Button variant="ghost" size="icon" onClick={toggleTheme} title="Toggle theme">
            {isDark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
          </Button>
        </div>
      </div>
    </header>
  );
}
