"use client";

import { useState, useEffect } from "react";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import type { DownloadStatus, Platform } from "@/lib/types";
import { STATUS_COLORS, PLATFORM_LABELS } from "@/lib/types";
import { Search, X, Trash2, RefreshCw } from "lucide-react";

interface DownloadsFiltersProps {
  status: DownloadStatus | "";
  platform: Platform | "";
  search: string;
  completedCount: number;
  failedCount: number;
  onStatusChange: (status: DownloadStatus | "") => void;
  onPlatformChange: (platform: Platform | "") => void;
  onSearchChange: (search: string) => void;
  onCleanup: () => void;
  onRetryAllFailed: () => void;
  cleanupLoading: boolean;
  retryAllLoading: boolean;
}

const statusOptions = [
  { value: "" as const, label: "All Statuses" },
  ...(Object.keys(STATUS_COLORS) as DownloadStatus[]).map((s) => ({
    value: s,
    label: s.charAt(0).toUpperCase() + s.slice(1),
  })),
];

const platformOptions = [
  { value: "" as const, label: "All Platforms" },
  ...(Object.keys(PLATFORM_LABELS) as Platform[]).map((p) => ({
    value: p,
    label: PLATFORM_LABELS[p],
  })),
];

export function DownloadsFilters({
  status, platform, search,
  completedCount, failedCount,
  onStatusChange, onPlatformChange, onSearchChange,
  onCleanup, onRetryAllFailed,
  cleanupLoading, retryAllLoading,
}: DownloadsFiltersProps) {
  const [confirmCleanup, setConfirmCleanup] = useState(false);

  // If completed records disappear while confirm is open, dismiss it
  useEffect(() => {
    if (completedCount === 0) setConfirmCleanup(false);
  }, [completedCount]);

  const handleCleanupClick = () => {
    if (completedCount === 0) return;
    if (!confirmCleanup) { setConfirmCleanup(true); return; }
    setConfirmCleanup(false);
    onCleanup();
  };

  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:flex-wrap">
      {/* Search */}
      <div className="relative flex-1 min-w-[180px] max-w-sm">
        <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
        <Input
          placeholder="Search URL, title, uploader…"
          value={search}
          onChange={(e) => onSearchChange(e.target.value)}
          className="pl-8 pr-8"
        />
        {search && (
          <button
            className="absolute right-2.5 top-2.5 text-muted-foreground hover:text-foreground"
            onClick={() => onSearchChange("")}
          >
            <X className="h-4 w-4" />
          </button>
        )}
      </div>

      {/* Status + Platform selects */}
      <div className="flex gap-2">
        <Select
          options={statusOptions}
          value={status}
          onChange={(e) => onStatusChange(e.target.value as DownloadStatus | "")}
          className="w-[140px]"
        />
        <Select
          options={platformOptions}
          value={platform}
          onChange={(e) => onPlatformChange(e.target.value as Platform | "")}
          className="w-[140px]"
        />
      </div>

      {/* Action buttons — pushed to right */}
      <div className="flex gap-2 sm:ml-auto">
        {/* Retry All Failed */}
        {failedCount > 0 && (
          <Button
            variant="outline"
            size="sm"
            onClick={onRetryAllFailed}
            disabled={retryAllLoading}
            className="text-amber-600 border-amber-300 hover:bg-amber-50 dark:text-amber-400 dark:border-amber-800 dark:hover:bg-amber-950"
          >
            <RefreshCw className={`h-4 w-4 mr-1.5 ${retryAllLoading ? "animate-spin" : ""}`} />
            Retry Failed ({failedCount})
          </Button>
        )}

        {/* Cleanup Completed — two-step confirm */}
        {completedCount > 0 && (
          confirmCleanup ? (
            <div className="flex items-center gap-1">
              <span className="text-xs text-muted-foreground whitespace-nowrap">
                Delete {completedCount} records?
              </span>
              <Button
                variant="destructive"
                size="sm"
                onClick={handleCleanupClick}
                disabled={cleanupLoading}
              >
                {cleanupLoading ? <RefreshCw className="h-4 w-4 animate-spin" /> : "Yes, delete"}
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setConfirmCleanup(false)}
                disabled={cleanupLoading}
              >
                Cancel
              </Button>
            </div>
          ) : (
            <Button
              variant="outline"
              size="sm"
              onClick={handleCleanupClick}
              disabled={cleanupLoading}
            >
              <Trash2 className="h-4 w-4 mr-1.5" />
              Cleanup ({completedCount})
            </Button>
          )
        )}
      </div>
    </div>
  );
}
