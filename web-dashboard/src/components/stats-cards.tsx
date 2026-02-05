"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import type { DownloadStats } from "@/lib/types";
import { Download, Clock, Loader2, CheckCircle, XCircle, Ban } from "lucide-react";

interface StatsCardsProps {
  stats: DownloadStats | null;
  loading: boolean;
}

const statCards = [
  { key: "total", label: "Total Downloads", icon: Download, color: "text-blue-500" },
  { key: "queued", label: "Queued", icon: Clock, color: "text-yellow-500" },
  { key: "processing", label: "Processing", icon: Loader2, color: "text-blue-500" },
  { key: "completed", label: "Completed", icon: CheckCircle, color: "text-green-500" },
  { key: "failed", label: "Failed", icon: XCircle, color: "text-red-500" },
  { key: "cancelled", label: "Cancelled", icon: Ban, color: "text-gray-500" },
] as const;

export function StatsCards({ stats, loading }: StatsCardsProps) {
  return (
    <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
      {statCards.map((stat) => (
        <Card key={stat.key}>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">{stat.label}</CardTitle>
            <stat.icon className={`h-4 w-4 ${stat.color}`} />
          </CardHeader>
          <CardContent>
            {loading ? (
              <Skeleton className="h-8 w-16" />
            ) : (
              <div className="text-2xl font-bold">
                {stats?.[stat.key] ?? 0}
              </div>
            )}
          </CardContent>
        </Card>
      ))}
    </div>
  );
}

