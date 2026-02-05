"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Download, BarChart3, Moon, Sun, Plus, AlertCircle, CheckCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";

const navigation = [
  { name: "Dashboard", href: "/", icon: BarChart3 },
  { name: "Downloads", href: "/downloads", icon: Download },
];

interface HeaderProps {
  onAddDownload: () => void;
}

export function Header({ onAddDownload }: HeaderProps) {
  const pathname = usePathname();
  const [isDark, setIsDark] = useState(false);
  const [serverOnline, setServerOnline] = useState<boolean | null>(null); // null = checking

  useEffect(() => {
    const dark = document.documentElement.classList.contains("dark");
    setIsDark(dark);
  }, []);

  useEffect(() => {
    // Check server health on mount and every 5 seconds
    const checkHealth = async () => {
      const isOnline = await api.checkHealth();
      setServerOnline(isOnline);
    };

    checkHealth();
    const interval = setInterval(checkHealth, 5000); // Check every 5 seconds for faster updates
    return () => clearInterval(interval);
  }, []);

  const toggleTheme = () => {
    document.documentElement.classList.toggle("dark");
    setIsDark(!isDark);
  };

  return (
    <header className="sticky top-0 z-40 w-full border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div className="container mx-auto px-4 flex h-14 items-center">
        <div className="mr-4 flex">
          <Link href="/" className="mr-6 flex items-center space-x-2">
            <Download className="h-6 w-6" />
            <span className="font-bold">X-Extract</span>
          </Link>
          <nav className="flex items-center space-x-6 text-sm font-medium">
            {navigation.map((item) => (
              <Link
                key={item.href}
                href={item.href}
                className={cn(
                  "flex items-center gap-2 transition-colors hover:text-foreground/80",
                  pathname === item.href
                    ? "text-foreground"
                    : "text-foreground/60"
                )}
              >
                <item.icon className="h-4 w-4" />
                {item.name}
              </Link>
            ))}
          </nav>
        </div>
        <div className="flex flex-1 items-center justify-end space-x-2">
          {/* Server Status Indicator */}
          <div className="flex items-center gap-2 text-sm">
            {serverOnline === null ? (
              // Checking status
              <div className="flex items-center gap-2 text-muted-foreground">
                <div className="h-2 w-2 rounded-full bg-muted-foreground animate-pulse" />
                <span>Checking...</span>
              </div>
            ) : serverOnline ? (
              // Server is running
              <div className="flex items-center gap-2 text-green-600 dark:text-green-500">
                <CheckCircle className="h-4 w-4" />
                <span className="font-medium">Server Running</span>
              </div>
            ) : (
              // Server is stopped
              <div className="flex items-center gap-2 text-destructive">
                <AlertCircle className="h-4 w-4" />
                <span className="font-medium">Server Stopped</span>
              </div>
            )}
          </div>
          <Button onClick={onAddDownload} size="sm" disabled={!serverOnline}>
            <Plus className="h-4 w-4 mr-1" />
            Add Download
          </Button>
          <Button variant="ghost" size="icon" onClick={toggleTheme}>
            {isDark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
          </Button>
        </div>
      </div>
    </header>
  );
}

