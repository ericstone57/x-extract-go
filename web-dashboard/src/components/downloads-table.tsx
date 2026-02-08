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
import {
  formatDate,
  truncateUrl,
  formatDuration,
  calculateDuration,
  parseMetadata,
  getFileName,
  truncateText,
} from "@/lib/utils";
import {
  RefreshCw,
  XCircle,
  Trash2,
  ExternalLink,
  Loader2,
  ChevronLeft,
  ChevronRight,
  ChevronDown,
  ChevronRight as ChevronRightIcon,
  FileText,
  Clock,
  User,
  List,
  Folder,
} from "lucide-react";

interface DownloadsTableProps {
  downloads: Download[];
  loading: boolean;
  onRefresh: () => void;
}

const ITEMS_PER_PAGE = 15;

export function DownloadsTable({ downloads, loading, onRefresh }: DownloadsTableProps) {
  const [page, setPage] = useState(0);
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [bulkActionLoading, setBulkActionLoading] = useState(false);
  const [expandedRows, setExpandedRows] = useState<Set<string>>(new Set());
  const { addToast } = useToast();

  const paginatedDownloads = downloads.slice(
    page * ITEMS_PER_PAGE,
    (page + 1) * ITEMS_PER_PAGE
  );
  const totalPages = Math.ceil(downloads.length / ITEMS_PER_PAGE);

  const allPageSelected = useMemo(() => {
    if (paginatedDownloads.length === 0) return false;
    return paginatedDownloads.every((d) => selectedIds.has(d.id));
  }, [paginatedDownloads, selectedIds]);

  const somePageSelected = useMemo(() => {
    return paginatedDownloads.some((d) => selectedIds.has(d.id)) && !allPageSelected;
  }, [paginatedDownloads, selectedIds, allPageSelected]);

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
      const newSelected = new Set(selectedIds);
      paginatedDownloads.forEach((d) => newSelected.delete(d.id));
      setSelectedIds(newSelected);
    } else {
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

  const toggleRow = (id: string) => {
    const newExpanded = new Set(expandedRows);
    if (newExpanded.has(id)) {
      newExpanded.delete(id);
    } else {
      newExpanded.add(id);
    }
    setExpandedRows(newExpanded);
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
          <Skeleton key={i} className="h-16 w-full" />
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

      <div className="rounded-md border">
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
              <TableHead>Info</TableHead>
              <TableHead className="w-[100px]">Platform</TableHead>
              <TableHead className="w-[100px]">Status</TableHead>
              <TableHead className="w-[100px]">Duration</TableHead>
              <TableHead className="w-[100px]">Files</TableHead>
              <TableHead className="w-[80px]">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {paginatedDownloads.map((download) => {
              const metadata = parseMetadata(download.metadata);
              const duration = calculateDuration(download.started_at, download.completed_at);
              const files = metadata?.files || (download.file_path ? [download.file_path] : []);
              const isExpanded = expandedRows.has(download.id);
              const isLoading = actionLoading === download.id;

              return (
                <>
                  <TableRow
                    key={download.id}
                    className={`${selectedIds.has(download.id) ? "bg-muted/50" : ""} ${isLoading ? "opacity-50" : ""}`}
                  >
                    <TableCell onClick={(e) => e.stopPropagation()}>
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
                      <div className="space-y-1">
                        {/* Title from metadata */}
                        {metadata?.title && (
                          <p className="font-medium text-sm truncate max-w-[250px]" title={metadata.title}>
                            {truncateText(metadata.title, 35)}
                          </p>
                        )}
                        {/* URL */}
                        <div className="flex items-center gap-2">
                          <span className="truncate max-w-[200px] text-muted-foreground" title={download.url}>
                            {truncateUrl(download.url, 30)}
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
                        {/* Uploader from metadata */}
                        {metadata?.uploader && (
                          <div className="flex items-center gap-1 text-xs text-muted-foreground">
                            <User className="h-3 w-3" />
                            <span>{truncateText(metadata.uploader, 25)}</span>
                          </div>
                        )}
                        {/* Description preview */}
                        {metadata?.description && (
                          <p className="text-xs text-muted-foreground truncate max-w-[250px]" title={metadata.description}>
                            {truncateText(metadata.description, 50)}
                          </p>
                        )}
                        {/* Error message */}
                        {download.error_message && (
                          <p className="text-xs text-destructive truncate max-w-[250px]" title={download.error_message}>
                            Error: {truncateText(download.error_message, 40)}
                          </p>
                        )}
                        {/* Timestamp info */}
                        <div className="flex items-center gap-3 text-xs text-muted-foreground">
                          <span>{formatDate(download.created_at)}</span>
                          {download.completed_at && (
                            <span className="flex items-center gap-1">
                              <Clock className="h-3 w-3" />
                              Completed {formatDate(download.completed_at)}
                            </span>
                          )}
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline">{PLATFORM_LABELS[download.platform]}</Badge>
                    </TableCell>
                    <TableCell>
                      <Badge className={STATUS_COLORS[download.status]}>
                        {download.status}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      {duration !== null ? (
                        <span className="text-sm font-medium">{formatDuration(duration)}</span>
                      ) : download.status === "processing" ? (
                        <span className="text-sm text-muted-foreground">-</span>
                      ) : (
                        <span className="text-sm text-muted-foreground">-</span>
                      )}
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1">
                        <List className="h-4 w-4 text-muted-foreground" />
                        <span className="text-sm">{files.length}</span>
                      </div>
                    </TableCell>
                    <TableCell onClick={(e) => e.stopPropagation()}>
                      {isLoading ? (
                        <Loader2 className="h-4 w-4 animate-spin" />
                      ) : (
                        <div className="flex items-center gap-1">
                          {files.length > 0 && (
                            <Button
                              variant="ghost"
                              size="icon"
                              className="h-8 w-8"
                              onClick={() => toggleRow(download.id)}
                              title="View details"
                            >
                              {isExpanded ? (
                                <ChevronDown className="h-4 w-4" />
                              ) : (
                                <ChevronRightIcon className="h-4 w-4" />
                              )}
                            </Button>
                          )}
                          {(download.status === "failed" || download.status === "cancelled") && (
                            <Button variant="ghost" size="icon" className="h-8 w-8" title="Restart" onClick={() => handleRetry(download.id)}>
                              <RefreshCw className="h-4 w-4" />
                            </Button>
                          )}
                          {(download.status === "queued" || download.status === "processing") && (
                            <Button variant="ghost" size="icon" className="h-8 w-8" title="Cancel" onClick={() => handleCancel(download.id)}>
                              <XCircle className="h-4 w-4" />
                            </Button>
                          )}
                          <Button variant="ghost" size="icon" className="h-8 w-8 text-destructive" title="Delete" onClick={() => handleDelete(download.id)}>
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </div>
                      )}
                    </TableCell>
                  </TableRow>
                  {/* Expanded details row */}
                  {isExpanded && files.length > 0 && (
                    <TableRow key={`${download.id}-details`} className="bg-muted/30">
                      <TableCell colSpan={8} className="p-4">
                        <div className="space-y-3">
                          <div className="flex items-center gap-2 font-medium text-sm">
                            <Folder className="h-4 w-4" />
                            <span>Downloaded Files ({files.length})</span>
                          </div>
                          <div className="pl-6 space-y-1">
                            {files.map((filePath, index) => (
                              <div key={index} className="flex items-center gap-2 text-sm">
                                <FileText className="h-3 w-3 text-muted-foreground" />
                                <span className="font-mono text-xs truncate max-w-[600px]" title={filePath}>
                                  {getFileName(filePath)}
                                </span>
                              </div>
                            ))}
                          </div>
                          {metadata?.description && (
                            <div className="pt-2 border-t">
                              <p className="text-xs font-medium text-muted-foreground mb-1">Description:</p>
                              <p className="text-sm whitespace-pre-wrap">{metadata.description}</p>
                            </div>
                          )}
                          {metadata?.tags && metadata.tags.length > 0 && (
                            <div className="flex items-center gap-1 flex-wrap">
                              <span className="text-xs font-medium text-muted-foreground">Tags:</span>
                              {metadata.tags.map((tag) => (
                                <Badge key={tag} variant="secondary" className="text-xs">
                                  {tag}
                                </Badge>
                              ))}
                            </div>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  )}
                </>
              );
            })}
          </TableBody>
        </Table>
      </div>

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
