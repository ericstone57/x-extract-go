"use client";

import { useEffect, useState, useCallback } from "react";
import { StatsCards } from "@/components/stats-cards";
import { StatsChart } from "@/components/stats-chart";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import type { DownloadStats, Download } from "@/lib/types";
import { useRefresh } from "./client-layout";
import { RefreshCw } from "lucide-react";

export default function DashboardPage() {
  const [stats, setStats] = useState<DownloadStats | null>(null);
  const [recentDownloads, setRecentDownloads] = useState<Download[]>([]);
  const [loading, setLoading] = useState(true);
  const { registerRefresh } = useRefresh();

  const fetchData = useCallback(async () => {
    try {
      const [statsData, downloadsData] = await Promise.all([
        api.getStats(),
        api.getDownloads(),
      ]);
      setStats(statsData);
      // Show recent 5 downloads
      setRecentDownloads(
        downloadsData
          .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
          .slice(0, 5)
      );
    } catch (error) {
      console.error("Failed to fetch data:", error);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
    registerRefresh(fetchData);

    // Auto-refresh every 10 seconds
    const interval = setInterval(fetchData, 10000);
    return () => clearInterval(interval);
  }, [fetchData, registerRefresh]);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Dashboard</h1>
          <p className="text-muted-foreground">
            Overview of your download queue
          </p>
        </div>
        <Button variant="outline" onClick={fetchData} disabled={loading}>
          <RefreshCw className={`h-4 w-4 mr-2 ${loading ? "animate-spin" : ""}`} />
          Refresh
        </Button>
      </div>

      <StatsCards stats={stats} loading={loading} />

      <div className="grid gap-6 md:grid-cols-2">
        <StatsChart stats={stats} loading={loading} />
        <Card>
          <CardHeader>
            <CardTitle>Recent Downloads</CardTitle>
          </CardHeader>
          <CardContent>
            {recentDownloads.length > 0 ? (
              <div className="space-y-2">
                {recentDownloads.map((download) => (
                  <div
                    key={download.id}
                    className="flex items-center justify-between text-sm"
                  >
                    <span className="truncate max-w-[200px]" title={download.url}>
                      {download.url}
                    </span>
                    <span className={`px-2 py-0.5 rounded text-xs ${
                      download.status === "completed" ? "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300" :
                      download.status === "failed" ? "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-300" :
                      download.status === "processing" ? "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300" :
                      "bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300"
                    }`}>
                      {download.status}
                    </span>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-muted-foreground text-center py-4">
                No recent downloads
              </p>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

