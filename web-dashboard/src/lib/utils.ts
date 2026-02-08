import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";
import type { DownloadMetadata } from "./types";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function formatDate(date: string | Date | null | undefined): string {
  if (!date) return "-";
  const d = typeof date === "string" ? new Date(date) : date;
  return d.toLocaleString();
}

export function truncateUrl(url: string, maxLength: number = 50): string {
  if (url.length <= maxLength) return url;
  return url.substring(0, maxLength) + "...";
}

// Format duration in seconds to human readable string
export function formatDuration(seconds: number | null | undefined): string {
  if (!seconds || seconds <= 0) return "-";

  if (seconds < 60) {
    return `${seconds}s`;
  }

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) {
    const secs = seconds % 60;
    return secs > 0 ? `${minutes}m ${secs}s` : `${minutes}m`;
  }

  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  if (hours < 24) {
    return remainingMinutes > 0 ? `${hours}h ${remainingMinutes}m` : `${hours}h`;
  }

  const days = Math.floor(hours / 24);
  const remainingHours = hours % 24;
  return remainingHours > 0 ? `${days}d ${remainingHours}h` : `${days}d`;
}

// Calculate duration between two dates in seconds
export function calculateDuration(
  startDate: string | Date | null | undefined,
  endDate: string | Date | null | undefined
): number | null {
  if (!startDate || !endDate) return null;
  const start = typeof startDate === "string" ? new Date(startDate) : startDate;
  const end = typeof endDate === "string" ? new Date(endDate) : endDate;
  return Math.floor((end.getTime() - start.getTime()) / 1000);
}

// Parse metadata JSON string
export function parseMetadata(metadataStr: string | null | undefined): DownloadMetadata | null {
  if (!metadataStr) return null;
  try {
    return JSON.parse(metadataStr) as DownloadMetadata;
  } catch {
    return null;
  }
}

// Get just the filename from a full path
export function getFileName(filePath: string): string {
  return filePath.split("/").pop() || filePath;
}

// Truncate text with ellipsis
export function truncateText(text: string, maxLength: number = 40): string {
  if (!text || text.length <= maxLength) return text;
  return text.substring(0, maxLength) + "...";
}

