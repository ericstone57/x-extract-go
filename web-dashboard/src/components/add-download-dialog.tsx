"use client";

import { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { api } from "@/lib/api";
import { useToast } from "@/components/ui/toast";
import { detectXURLType } from "@/lib/utils";
import type { Platform } from "@/lib/types";
import { Loader2, ChevronDown, ChevronUp } from "lucide-react";

interface AddDownloadDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSuccess: () => void;
}

// ─── URL analysis ────────────────────────────────────────────────────────────

type URLInfo =
  | { kind: "x-single" }
  | { kind: "x-timeline" }
  | { kind: "telegram" }
  | { kind: "gallery" }
  | { kind: "unknown" };

function analyzeURL(url: string): URLInfo {
  if (!url.startsWith("http://") && !url.startsWith("https://"))
    return { kind: "unknown" };
  if (url.includes("t.me")) return { kind: "telegram" };
  const xType = detectXURLType(url);
  if (xType === "single") return { kind: "x-single" };
  if (xType === "timeline") return { kind: "x-timeline" };
  return { kind: "gallery" };
}

// ─── URL info badge ───────────────────────────────────────────────────────────

function URLInfoBadge({ info }: { info: URLInfo }) {
  if (info.kind === "unknown") return null;

  const configs = {
    "x-single": {
      emoji: "🐦",
      label: "Single tweet",
      tool: "yt-dlp",
      color: "bg-sky-50 border-sky-200 text-sky-800 dark:bg-sky-950 dark:border-sky-800 dark:text-sky-300",
    },
    "x-timeline": {
      emoji: "📋",
      label: "Account timeline",
      tool: "gallery-dl",
      color: "bg-violet-50 border-violet-200 text-violet-800 dark:bg-violet-950 dark:border-violet-800 dark:text-violet-300",
    },
    telegram: {
      emoji: "✈️",
      label: "Telegram",
      tool: "tdl",
      color: "bg-blue-50 border-blue-200 text-blue-800 dark:bg-blue-950 dark:border-blue-800 dark:text-blue-300",
    },
    gallery: {
      emoji: "🖼️",
      label: "Gallery (100+ sites)",
      tool: "gallery-dl",
      color: "bg-emerald-50 border-emerald-200 text-emerald-800 dark:bg-emerald-950 dark:border-emerald-800 dark:text-emerald-300",
    },
  } as const;

  const c = configs[info.kind as keyof typeof configs];
  return (
    <div className={`flex items-center gap-2 rounded-md border px-3 py-2 text-sm font-medium ${c.color}`}>
      <span>{c.emoji}</span>
      <span>{c.label}</span>
      <span className="mx-1 opacity-40">→</span>
      <span className="font-mono text-xs opacity-70">{c.tool}</span>
    </div>
  );
}

// ─── Timeline filters panel ───────────────────────────────────────────────────

interface TimelineFiltersProps {
  skipReplies: boolean;
  skipRetweets: boolean;
  dateFrom: string;
  onSkipRepliesChange: (v: boolean) => void;
  onSkipRetweetsChange: (v: boolean) => void;
  onDateFromChange: (v: string) => void;
}

function TimelineFilters({
  skipReplies, skipRetweets, dateFrom,
  onSkipRepliesChange, onSkipRetweetsChange, onDateFromChange,
}: TimelineFiltersProps) {
  const [open, setOpen] = useState(false);
  const activeCount = [skipReplies, skipRetweets, dateFrom !== ""].filter(Boolean).length;

  return (
    <div className="rounded-md border border-dashed">
      <button
        type="button"
        className="flex w-full items-center justify-between px-3 py-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
        onClick={() => setOpen((o) => !o)}
      >
        <span>
          Timeline filters
          {activeCount > 0 && (
            <span className="ml-2 rounded-full bg-primary text-primary-foreground text-xs px-1.5 py-0.5">
              {activeCount}
            </span>
          )}
        </span>
        {open ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
      </button>

      {open && (
        <div className="border-t px-3 py-3 space-y-3">
          <label className="flex items-center gap-2 text-sm cursor-pointer select-none">
            <input
              type="checkbox"
              checked={skipReplies}
              onChange={(e) => onSkipRepliesChange(e.target.checked)}
              className="rounded border-input"
            />
            Skip replies
          </label>
          <label className="flex items-center gap-2 text-sm cursor-pointer select-none">
            <input
              type="checkbox"
              checked={skipRetweets}
              onChange={(e) => onSkipRetweetsChange(e.target.checked)}
              className="rounded border-input"
            />
            Skip retweets
          </label>
          <div className="space-y-1">
            <label className="text-sm text-muted-foreground">Download from date</label>
            <Input
              type="date"
              value={dateFrom}
              onChange={(e) => onDateFromChange(e.target.value)}
              className="h-8 text-sm"
            />
          </div>
        </div>
      )}
    </div>
  );
}

// ─── Main dialog ──────────────────────────────────────────────────────────────

export function AddDownloadDialog({ open, onOpenChange, onSuccess }: AddDownloadDialogProps) {
  const [url, setUrl] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  // Timeline filters
  const [skipReplies, setSkipReplies] = useState(false);
  const [skipRetweets, setSkipRetweets] = useState(false);
  const [dateFrom, setDateFrom] = useState("");
  const { addToast } = useToast();

  const info = analyzeURL(url.trim());
  const isTimeline = info.kind === "x-timeline";

  const buildFilters = (): string | undefined => {
    if (!isTimeline) return undefined;
    const parts: string[] = [];
    if (skipReplies) parts.push("replies=false");
    if (skipRetweets) parts.push("retweets=false");
    if (dateFrom) parts.push(`date-min=${dateFrom}`);
    return parts.length > 0 ? parts.join("|") : undefined;
  };

  const resolvePlatform = (): Platform | null => {
    switch (info.kind) {
      case "x-single": return "x";
      case "x-timeline": return "gallery";
      case "telegram": return "telegram";
      case "gallery": return "gallery";
      default: return null;
    }
  };

  const reset = () => {
    setUrl("");
    setError("");
    setSkipReplies(false);
    setSkipRetweets(false);
    setDateFrom("");
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    const trimmedUrl = url.trim();
    if (!trimmedUrl) { setError("URL is required"); return; }

    const platform = resolvePlatform();
    if (!platform) { setError("Enter a valid http/https URL."); return; }

    setLoading(true);
    try {
      await api.createDownload({ url: trimmedUrl, platform, filters: buildFilters() });
      addToast({ type: "success", title: "Queued", description: "Download added to queue." });
      reset();
      onOpenChange(false);
      onSuccess();
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Failed to add download";
      setError(msg);
    } finally {
      setLoading(false);
    }
  };

  const handleOpenChange = (v: boolean) => {
    if (!loading) { if (!v) reset(); onOpenChange(v); }
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>Add Download</DialogTitle>
          <DialogDescription>
            Paste a tweet, account, Telegram, or any supported URL.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4 py-2">
          {/* URL input */}
          <div className="space-y-2">
            <label htmlFor="url" className="text-sm font-medium">URL</label>
            <Input
              id="url"
              placeholder="https://x.com/username  or  /status/123…"
              value={url}
              onChange={(e) => { setUrl(e.target.value); setError(""); }}
              disabled={loading}
              autoFocus
            />
          </div>

          {/* Detection badge */}
          {url.trim() && <URLInfoBadge info={info} />}

          {/* Timeline filters */}
          {isTimeline && (
            <TimelineFilters
              skipReplies={skipReplies}
              skipRetweets={skipRetweets}
              dateFrom={dateFrom}
              onSkipRepliesChange={setSkipReplies}
              onSkipRetweetsChange={setSkipRetweets}
              onDateFromChange={setDateFrom}
            />
          )}

          {error && <p className="text-sm text-destructive">{error}</p>}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => handleOpenChange(false)} disabled={loading}>
              Cancel
            </Button>
            <Button type="submit" disabled={loading || info.kind === "unknown"}>
              {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Add Download
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
