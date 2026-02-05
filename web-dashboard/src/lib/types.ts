// Download status types
export type DownloadStatus = "queued" | "processing" | "completed" | "failed" | "cancelled";

// Platform types
export type Platform = "x" | "telegram";

// Download mode types
export type DownloadMode = "default" | "single" | "group";

// Download entity from API
export interface Download {
  id: string;
  url: string;
  platform: Platform;
  status: DownloadStatus;
  mode: DownloadMode;
  priority: number;
  retry_count: number;
  error_message?: string;
  file_path?: string;
  metadata?: string;
  process_log?: string;
  created_at: string;
  updated_at: string;
  started_at?: string;
  completed_at?: string;
}

// Statistics from API
export interface DownloadStats {
  total: number;
  queued: number;
  processing: number;
  completed: number;
  failed: number;
  cancelled: number;
}

// Request to create a download
export interface CreateDownloadRequest {
  url: string;
  platform?: Platform;
  mode?: DownloadMode;
}

// Filter parameters for listing downloads
export interface DownloadFilters {
  status?: DownloadStatus;
  platform?: Platform;
  search?: string;
  page?: number;
  limit?: number;
}

// API error response
export interface ApiError {
  error: string;
}

// API success message response
export interface ApiMessage {
  message: string;
}

// Status colors for UI
export const STATUS_COLORS: Record<DownloadStatus, string> = {
  queued: "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-300",
  processing: "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300",
  completed: "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300",
  failed: "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-300",
  cancelled: "bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300",
};

// Platform icons/labels
export const PLATFORM_LABELS: Record<Platform, string> = {
  x: "X/Twitter",
  telegram: "Telegram",
};

