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
import { Select } from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import { api } from "@/lib/api";
import { useToast } from "@/components/ui/toast";
import type { Platform } from "@/lib/types";
import { Loader2 } from "lucide-react";

interface AddDownloadDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSuccess: () => void;
}

const platformOptions = [
  { value: "", label: "Auto-detect" },
  { value: "x", label: "X/Twitter" },
  { value: "telegram", label: "Telegram" },
];

function detectPlatform(url: string): Platform | null {
  if (url.includes("x.com") || url.includes("twitter.com")) {
    return "x";
  }
  if (url.includes("t.me")) {
    return "telegram";
  }
  return null;
}

export function AddDownloadDialog({ open, onOpenChange, onSuccess }: AddDownloadDialogProps) {
  const [url, setUrl] = useState("");
  const [platform, setPlatform] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const { addToast } = useToast();

  const detectedPlatform = detectPlatform(url);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    if (!url.trim()) {
      setError("URL is required");
      return;
    }

    const finalPlatform = platform || detectedPlatform;
    if (!finalPlatform) {
      setError("Could not detect platform. Please select manually.");
      return;
    }

    setLoading(true);
    try {
      await api.createDownload({
        url: url.trim(),
        platform: finalPlatform as Platform,
      });
      addToast({ type: "success", title: "Download added", description: "Your download has been queued." });
      setUrl("");
      setPlatform("");
      onOpenChange(false);
      onSuccess();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to add download";
      setError(message);
      addToast({ type: "error", title: "Error", description: message });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>Add New Download</DialogTitle>
          <DialogDescription>
            Enter the URL of the media you want to download from X/Twitter or Telegram.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit}>
          <div className="grid gap-4 py-4">
            <div className="space-y-2">
              <label htmlFor="url" className="text-sm font-medium">URL</label>
              <Input
                id="url"
                placeholder="https://x.com/user/status/123456789"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                disabled={loading}
              />
              {detectedPlatform && !platform && (
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  Detected platform:{" "}
                  <Badge variant="secondary">
                    {detectedPlatform === "x" ? "X/Twitter" : "Telegram"}
                  </Badge>
                </div>
              )}
            </div>
            <div className="space-y-2">
              <label htmlFor="platform" className="text-sm font-medium">Platform (optional)</label>
              <Select
                id="platform"
                options={platformOptions}
                value={platform}
                onChange={(e) => setPlatform(e.target.value)}
                disabled={loading}
              />
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={loading}>
              Cancel
            </Button>
            <Button type="submit" disabled={loading}>
              {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Add Download
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

