"use client";

import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import type { DownloadStatus, Platform } from "@/lib/types";
import { Search, X, Trash2 } from "lucide-react";

interface DownloadsFiltersProps {
  status: DownloadStatus | "";
  platform: Platform | "";
  search: string;
  onStatusChange: (status: DownloadStatus | "") => void;
  onPlatformChange: (platform: Platform | "") => void;
  onSearchChange: (search: string) => void;
  onCleanup: () => void;
  cleanupLoading: boolean;
}

const statusOptions = [
  { value: "", label: "All Statuses" },
  { value: "queued", label: "Queued" },
  { value: "processing", label: "Processing" },
  { value: "completed", label: "Completed" },
  { value: "failed", label: "Failed" },
  { value: "cancelled", label: "Cancelled" },
];

const platformOptions = [
  { value: "", label: "All Platforms" },
  { value: "x", label: "X/Twitter" },
  { value: "telegram", label: "Telegram" },
];

export function DownloadsFilters({
  status,
  platform,
  search,
  onStatusChange,
  onPlatformChange,
  onSearchChange,
  onCleanup,
  cleanupLoading,
}: DownloadsFiltersProps) {
  return (
    <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
      <div className="flex flex-1 flex-col gap-4 md:flex-row md:items-center">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search by URL or ID..."
            value={search}
            onChange={(e) => onSearchChange(e.target.value)}
            className="pl-8"
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
      </div>
      <Button
        variant="outline"
        onClick={onCleanup}
        disabled={cleanupLoading}
        className="shrink-0"
      >
        <Trash2 className="h-4 w-4 mr-2" />
        Clean Up Completed
      </Button>
    </div>
  );
}

