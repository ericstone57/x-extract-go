"use client";

import { useState, useEffect } from "react";
import { api } from "@/lib/api";

/**
 * Polls the backend health endpoint every 5 seconds.
 * Returns null while the initial check is in progress, then true/false.
 */
export function useServerHealth(): boolean | null {
  const [serverOnline, setServerOnline] = useState<boolean | null>(null);

  useEffect(() => {
    const check = async () => setServerOnline(await api.checkHealth());
    check();
    const id = setInterval(check, 5000);
    return () => clearInterval(id);
  }, []);

  return serverOnline;
}
