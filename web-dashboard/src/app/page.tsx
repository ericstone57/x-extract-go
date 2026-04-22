"use client";

import { useEffect, useState, useCallback, useMemo } from "react";
import { DownloadsTable } from "@/components/downloads-table";
import { DownloadsFilters } from "@/components/downloads-filters";
import { api } from "@/lib/api";
import type { Download, DownloadStatus, Platform } from "@/lib/types";
import { useRefresh } from "./client-layout";
import { useToast } from "@/components/ui/toast";
import { parseMetadata } from "@/lib/utils";
import {
  Download as DownloadIcon, Clock, Loader2, CheckCircle, XCircle, Ban,
} from "lucide-react";

// ─── Compact stats bar ────────────────────────────────────────────────────────

const statConfig = {
  total:      { label: "Total",      Icon: DownloadIcon, color: "text-foreground" },
  queued:     { label: "Queued",     Icon: Clock,        color: "text-yellow-500" },
  processing: { label: "Processing", Icon: Loader2,      color: "text-blue-500" },
  completed:  { label: "Completed",  Icon: CheckCircle,  color: "text-green-500" },
  failed:     { label: "Failed",     Icon: XCircle,      color: "text-red-500" },
  cancelled:  { label: "Cancelled",  Icon: Ban,          color: "text-muted-foreground" },
} as const;

function StatPill({ k, value, onClick, active }: { k: keyof typeof statConfig; value: number; onClick?: () => void; active?: boolean }) {
  const { label, Icon, color } = statConfig[k];
  return (
    <button
      type="button"
      onClick={onClick}
      className={`flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition-colors
        ${active
          ? "bg-primary text-primary-foreground"
          : "bg-muted hover:bg-muted/80 text-foreground"
        }
        ${onClick ? "cursor-pointer" : "cursor-default pointer-events-none"}`}
      title={onClick ? `Filter by ${label.toLowerCase()}` : undefined}
    >
      <Icon className={`h-3.5 w-3.5 ${active ? "" : color}`} />
      <span>{value}</span>
      <span className={`hidden sm:inline ${active ? "" : "text-muted-foreground"}`}>{label}</span>
    </button>
  );
}

// ─── Main page ────────────────────────────────────────────────────────────────

export default function HomePage() {
  const [downloads, setDownloads] = useState<Download[]>([]);
  const [loading, setLoading] = useState(true);
  const [statusFilter, setStatusFilter] = useState<DownloadStatus | "">("");
  const [platformFilter, setPlatformFilter] = useState<Platform | "">("");
  const [search, setSearch] = useState("");
  const [cleanupLoading, setCleanupLoading] = useState(false);
  const [retryAllLoading, setRetryAllLoading] = useState(false);
  const { registerRefresh } = useRefresh();
  const { addToast } = useToast();

  const fetchDownloads = useCallback(async () => {
    try {
      const data = await api.getDownloads();
      setDownloads(
        data.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
      );
    } catch (err) {
      console.debug("Failed to fetch downloads:", err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchDownloads();
    registerRefresh(fetchDownloads);
    const interval = setInterval(fetchDownloads, 5000);
    return () => clearInterval(interval);
  }, [fetchDownloads, registerRefresh]);

  // Stats always derived from the full unfiltered list
  const stats = useMemo(() => ({
    total:      downloads.length,
    queued:     downloads.filter((d) => d.status === "queued").length,
    processing: downloads.filter((d) => d.status === "processing").length,
    completed:  downloads.filter((d) => d.status === "completed").length,
    failed:     downloads.filter((d) => d.status === "failed").length,
    cancelled:  downloads.filter((d) => d.status === "cancelled").length,
  }), [downloads]);

  // Client-side filter: status + platform + search
  const filteredDownloads = useMemo(() => {
    return downloads.filter((d) => {
      if (statusFilter && d.status !== statusFilter) return false;
      if (platformFilter && d.platform !== platformFilter) return false;
      if (search) {
        const q = search.toLowerCase();
        if (d.url.toLowerCase().includes(q)) return true;
        if (d.id.toLowerCase().includes(q)) return true;
        const meta = parseMetadata(d.metadata);
        if (meta?.title?.toLowerCase().includes(q)) return true;
        if (meta?.uploader?.toLowerCase().includes(q)) return true;
        if (meta?.uploader_id?.toLowerCase().includes(q)) return true;
        return false;
      }
      return true;
    });
  }, [downloads, statusFilter, platformFilter, search]);

  // Stat pill click: toggle status filter
  const handleStatClick = (key: DownloadStatus) => {
    setStatusFilter((prev) => (prev === key ? "" : key));
  };

  const bulkAction = useCallback(async (
    filter: (d: Download) => boolean,
    apiCall: (id: string) => Promise<unknown>,
    setLoading: (v: boolean) => void,
    toast: (count: number) => { title: string; description: string },
  ) => {
    setLoading(true);
    let count = 0;
    for (const d of downloads.filter(filter)) {
      try { await apiCall(d.id); count++; } catch {}
    }
    addToast({ type: "success", ...toast(count) });
    setLoading(false);
    fetchDownloads();
  }, [downloads, addToast, fetchDownloads]);

  const handleCleanup = () => bulkAction(
    (d) => d.status === "completed",
    (id) => api.deleteDownload(id),
    setCleanupLoading,
    (n) => ({ title: "Cleanup complete", description: `Deleted ${n} completed records.` }),
  );

  const handleRetryAllFailed = () => bulkAction(
    (d) => d.status === "failed" || d.status === "cancelled",
    (id) => api.retryDownload(id),
    setRetryAllLoading,
    (n) => ({ title: "Retrying", description: `Queued ${n} downloads for retry.` }),
  );

  return (
    <div className="space-y-4">
      {/* Compact stats bar */}
      <div className="flex flex-wrap items-center gap-2">
        <StatPill k="total"      value={stats.total}      />
        <StatPill k="queued"     value={stats.queued}     onClick={() => handleStatClick("queued")}     active={statusFilter === "queued"} />
        <StatPill k="processing" value={stats.processing} onClick={() => handleStatClick("processing")} active={statusFilter === "processing"} />
        <StatPill k="completed"  value={stats.completed}  onClick={() => handleStatClick("completed")}  active={statusFilter === "completed"} />
        <StatPill k="failed"     value={stats.failed}     onClick={() => handleStatClick("failed")}     active={statusFilter === "failed"} />
        <StatPill k="cancelled"  value={stats.cancelled}  onClick={() => handleStatClick("cancelled")}  active={statusFilter === "cancelled"} />
      </div>

      {/* Filters row */}
      <DownloadsFilters
        status={statusFilter}
        platform={platformFilter}
        search={search}
        completedCount={stats.completed}
        failedCount={stats.failed}
        onStatusChange={setStatusFilter}
        onPlatformChange={setPlatformFilter}
        onSearchChange={setSearch}
        onCleanup={handleCleanup}
        onRetryAllFailed={handleRetryAllFailed}
        cleanupLoading={cleanupLoading}
        retryAllLoading={retryAllLoading}
      />

      {/* Downloads table */}
      <DownloadsTable
        downloads={filteredDownloads}
        loading={loading}
        onRefresh={fetchDownloads}
      />
    </div>
  );
}
