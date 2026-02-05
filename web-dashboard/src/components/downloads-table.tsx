"use client";

import { useState, useMemo } from "react";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Checkbox } from "@/components/ui/checkbox";
import { useToast } from "@/components/ui/toast";
import { api } from "@/lib/api";
import type { Download } from "@/lib/types";
import { STATUS_COLORS, PLATFORM_LABELS } from "@/lib/types";
import { formatDate, truncateUrl } from "@/lib/utils";
import {
  RefreshCw,
  XCircle,
  Trash2,
  ExternalLink,
  FolderOpen,
  Loader2,
  ChevronLeft,
  ChevronRight,
} from "lucide-react";

interface DownloadsTableProps {
  downloads: Download[];
  loading: boolean;
  onRefresh: () => void;
}

const ITEMS_PER_PAGE = 20;

export function DownloadsTable({ downloads, loading, onRefresh }: DownloadsTableProps) {
  const [page, setPage] = useState(0);
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [bulkActionLoading, setBulkActionLoading] = useState(false);
  const { addToast } = useToast();

  const paginatedDownloads = downloads.slice(
    page * ITEMS_PER_PAGE,
    (page + 1) * ITEMS_PER_PAGE
  );
  const totalPages = Math.ceil(downloads.length / ITEMS_PER_PAGE);

  // Check if all items on current page are selected
  const allPageSelected = useMemo(() => {
    if (paginatedDownloads.length === 0) return false;
    return paginatedDownloads.every((d) => selectedIds.has(d.id));
  }, [paginatedDownloads, selectedIds]);

  const somePageSelected = useMemo(() => {
    return paginatedDownloads.some((d) => selectedIds.has(d.id)) && !allPageSelected;
  }, [paginatedDownloads, selectedIds, allPageSelected]);

  // Get selected downloads for bulk actions
  const selectedDownloads = useMemo(() => {
    return downloads.filter((d) => selectedIds.has(d.id));
  }, [downloads, selectedIds]);

  const canBulkCancel = useMemo(() => {
    return selectedDownloads.some((d) => d.status === "queued" || d.status === "processing");
  }, [selectedDownloads]);

  const canBulkDelete = useMemo(() => {
    return selectedDownloads.some((d) => d.status !== "processing");
  }, [selectedDownloads]);

  const handleSelectAll = () => {
    if (allPageSelected) {
      // Deselect all on current page
      const newSelected = new Set(selectedIds);
      paginatedDownloads.forEach((d) => newSelected.delete(d.id));
      setSelectedIds(newSelected);
    } else {
      // Select all on current page
      const newSelected = new Set(selectedIds);
      paginatedDownloads.forEach((d) => newSelected.add(d.id));
      setSelectedIds(newSelected);
    }
  };

  const handleSelectOne = (id: string) => {
    const newSelected = new Set(selectedIds);
    if (newSelected.has(id)) {
      newSelected.delete(id);
    } else {
      newSelected.add(id);
    }
    setSelectedIds(newSelected);
  };

  const clearSelection = () => {
    setSelectedIds(new Set());
  };

  const handleBulkCancel = async () => {
    const toCancel = selectedDownloads.filter(
      (d) => d.status === "queued" || d.status === "processing"
    );
    if (toCancel.length === 0) return;

    setBulkActionLoading(true);
    let cancelled = 0;
    let failed = 0;

    for (const download of toCancel) {
      try {
        await api.cancelDownload(download.id);
        cancelled++;
      } catch {
        failed++;
      }
    }

    setBulkActionLoading(false);
    clearSelection();
    addToast({
      type: failed > 0 ? "error" : "success",
      title: "Bulk Cancel Complete",
      description: `Cancelled ${cancelled} downloads${failed > 0 ? `, ${failed} failed` : ""}`,
    });
    onRefresh();
  };

  const handleBulkDelete = async () => {
    const toDelete = selectedDownloads.filter((d) => d.status !== "processing");
    if (toDelete.length === 0) return;

    setBulkActionLoading(true);
    let deleted = 0;
    let failed = 0;

    for (const download of toDelete) {
      try {
        await api.deleteDownload(download.id);
        deleted++;
      } catch {
        failed++;
      }
    }

    setBulkActionLoading(false);
    clearSelection();
    addToast({
      type: failed > 0 ? "error" : "success",
      title: "Bulk Delete Complete",
      description: `Deleted ${deleted} downloads${failed > 0 ? `, ${failed} failed` : ""}`,
    });
    onRefresh();
  };

  const handleRetry = async (id: string) => {
    setActionLoading(id);
    try {
      await api.retryDownload(id);
      addToast({ type: "success", title: "Retry queued", description: "Download will be retried." });
      onRefresh();
    } catch (err) {
      addToast({ type: "error", title: "Error", description: err instanceof Error ? err.message : "Failed to retry" });
    } finally {
      setActionLoading(null);
    }
  };

  const handleCancel = async (id: string) => {
    setActionLoading(id);
    try {
      await api.cancelDownload(id);
      addToast({ type: "success", title: "Cancelled", description: "Download has been cancelled." });
      onRefresh();
    } catch (err) {
      addToast({ type: "error", title: "Error", description: err instanceof Error ? err.message : "Failed to cancel" });
    } finally {
      setActionLoading(null);
    }
  };

  const handleDelete = async (id: string) => {
    setActionLoading(id);
    try {
      await api.deleteDownload(id);
      addToast({ type: "success", title: "Deleted", description: "Download record has been deleted." });
      onRefresh();
    } catch (err) {
      addToast({ type: "error", title: "Error", description: err instanceof Error ? err.message : "Failed to delete" });
    } finally {
      setActionLoading(null);
    }
  };

  if (loading) {
    return (
      <div className="space-y-2">
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} className="h-12 w-full" />
        ))}
      </div>
    );
  }

  if (downloads.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
        <p>No downloads found</p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Bulk Action Bar */}
      {selectedIds.size > 0 && (
        <div className="flex items-center gap-4 p-3 bg-muted rounded-lg">
          <span className="text-sm font-medium">
            {selectedIds.size} item{selectedIds.size !== 1 ? "s" : ""} selected
          </span>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={handleBulkCancel}
              disabled={!canBulkCancel || bulkActionLoading}
            >
              {bulkActionLoading ? (
                <Loader2 className="h-4 w-4 mr-2 animate-spin" />
              ) : (
                <XCircle className="h-4 w-4 mr-2" />
              )}
              Cancel Selected
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={handleBulkDelete}
              disabled={!canBulkDelete || bulkActionLoading}
              className="text-destructive hover:text-destructive"
            >
              {bulkActionLoading ? (
                <Loader2 className="h-4 w-4 mr-2 animate-spin" />
              ) : (
                <Trash2 className="h-4 w-4 mr-2" />
              )}
              Delete Selected
            </Button>
          </div>
          <Button
            variant="ghost"
            size="sm"
            onClick={clearSelection}
            disabled={bulkActionLoading}
          >
            Clear Selection
          </Button>
        </div>
      )}
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-[40px]">
              <Checkbox
                checked={allPageSelected}
                indeterminate={somePageSelected}
                onChange={handleSelectAll}
                aria-label="Select all on page"
              />
            </TableHead>
            <TableHead className="w-[80px]">ID</TableHead>
            <TableHead>URL</TableHead>
            <TableHead className="w-[100px]">Platform</TableHead>
            <TableHead className="w-[100px]">Status</TableHead>
            <TableHead className="w-[160px]">Created</TableHead>
            <TableHead className="w-[160px]">Completed</TableHead>
            <TableHead className="w-[150px]">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {paginatedDownloads.map((download) => (
            <TableRow key={download.id} className={selectedIds.has(download.id) ? "bg-muted/50" : ""}>
              <TableCell>
                <Checkbox
                  checked={selectedIds.has(download.id)}
                  onChange={() => handleSelectOne(download.id)}
                  aria-label={`Select download ${download.id}`}
                />
              </TableCell>
              <TableCell className="font-mono text-xs">
                {download.id.substring(0, 8)}
              </TableCell>
              <TableCell>
                <div className="flex items-center gap-2">
                  <span className="truncate max-w-[300px]" title={download.url}>
                    {truncateUrl(download.url, 50)}
                  </span>
                  <a
                    href={download.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-muted-foreground hover:text-foreground"
                  >
                    <ExternalLink className="h-3 w-3" />
                  </a>
                </div>
                {download.error_message && (
                  <p className="text-xs text-destructive mt-1 truncate max-w-[300px]" title={download.error_message}>
                    {download.error_message}
                  </p>
                )}
              </TableCell>
              <TableCell>
                <Badge variant="outline">{PLATFORM_LABELS[download.platform]}</Badge>
              </TableCell>
              <TableCell>
                <Badge className={STATUS_COLORS[download.status]}>
                  {download.status}
                </Badge>
              </TableCell>
              <TableCell className="text-sm">{formatDate(download.created_at)}</TableCell>
              <TableCell className="text-sm">{formatDate(download.completed_at)}</TableCell>
              <TableCell>
                <DownloadActions
                  download={download}
                  loading={actionLoading === download.id}
                  onRetry={() => handleRetry(download.id)}
                  onCancel={() => handleCancel(download.id)}
                  onDelete={() => handleDelete(download.id)}
                />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      {totalPages > 1 && (
        <div className="flex items-center justify-between">
          <p className="text-sm text-muted-foreground">
            Showing {page * ITEMS_PER_PAGE + 1} to{" "}
            {Math.min((page + 1) * ITEMS_PER_PAGE, downloads.length)} of{" "}
            {downloads.length} downloads
          </p>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPage((p) => Math.max(0, p - 1))}
              disabled={page === 0}
            >
              <ChevronLeft className="h-4 w-4" />
              Previous
            </Button>
            <span className="text-sm">
              Page {page + 1} of {totalPages}
            </span>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
              disabled={page >= totalPages - 1}
            >
              Next
              <ChevronRight className="h-4 w-4" />
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}

interface DownloadActionsProps {
  download: Download;
  loading: boolean;
  onRetry: () => void;
  onCancel: () => void;
  onDelete: () => void;
}

function DownloadActions({ download, loading, onRetry, onCancel, onDelete }: DownloadActionsProps) {
  if (loading) {
    return <Loader2 className="h-4 w-4 animate-spin" />;
  }

  return (
    <div className="flex items-center gap-1">
      {download.status === "completed" && download.file_path && (
        <Button
          variant="ghost"
          size="icon"
          title="Open file location"
          onClick={() => {
            // In a real app, this would open the file location
            alert(`File location: ${download.file_path}`);
          }}
        >
          <FolderOpen className="h-4 w-4" />
        </Button>
      )}
      {download.status === "failed" && (
        <Button variant="ghost" size="icon" title="Retry" onClick={onRetry}>
          <RefreshCw className="h-4 w-4" />
        </Button>
      )}
      {(download.status === "queued" || download.status === "processing") && (
        <Button variant="ghost" size="icon" title="Cancel" onClick={onCancel}>
          <XCircle className="h-4 w-4" />
        </Button>
      )}
      <Button variant="ghost" size="icon" title="Delete" onClick={onDelete}>
        <Trash2 className="h-4 w-4 text-destructive" />
      </Button>
    </div>
  );
}

