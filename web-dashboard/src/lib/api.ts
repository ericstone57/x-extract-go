import type {
  Download,
  DownloadStats,
  CreateDownloadRequest,
  DownloadFilters,
  ApiError,
  ApiMessage,
} from "./types";

const API_BASE = "/api/v1";

class ApiClient {
  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const url = `${API_BASE}${endpoint}`;
    const response = await fetch(url, {
      ...options,
      headers: {
        "Content-Type": "application/json",
        ...options.headers,
      },
    });

    if (!response.ok) {
      const error: ApiError = await response.json().catch(() => ({
        error: `HTTP error ${response.status}`,
      }));
      throw new Error(error.error);
    }

    return response.json();
  }

  // Downloads
  async getDownloads(filters?: DownloadFilters): Promise<Download[]> {
    const params = new URLSearchParams();
    if (filters?.status) params.append("status", filters.status);
    if (filters?.platform) params.append("platform", filters.platform);
    const query = params.toString();
    return this.request<Download[]>(`/downloads${query ? `?${query}` : ""}`);
  }

  async getDownload(id: string): Promise<Download> {
    return this.request<Download>(`/downloads/${id}`);
  }

  async createDownload(data: CreateDownloadRequest): Promise<Download> {
    return this.request<Download>("/downloads", {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  async retryDownload(id: string): Promise<ApiMessage> {
    return this.request<ApiMessage>(`/downloads/${id}/retry`, {
      method: "POST",
    });
  }

  async cancelDownload(id: string): Promise<ApiMessage> {
    return this.request<ApiMessage>(`/downloads/${id}/cancel`, {
      method: "POST",
    });
  }

  async deleteDownload(id: string): Promise<ApiMessage> {
    return this.request<ApiMessage>(`/downloads/${id}`, {
      method: "DELETE",
    });
  }

  // Stats
  async getStats(): Promise<DownloadStats> {
    return this.request<DownloadStats>("/downloads/stats");
  }

  // Health check - check if the backend server is responding
  async checkHealth(): Promise<boolean> {
    try {
      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), 2000); // 2 second timeout

      // Use relative path so it works regardless of server port
      // The static dashboard is served by the Go server, so same origin
      await fetch("/api/v1/downloads?limit=1", {
        method: "GET",
        signal: controller.signal,
        cache: "no-store",
      });

      clearTimeout(timeoutId);
      // Server is online if we get any response (even error responses mean server is up)
      return true;
    } catch (error) {
      // Network error, timeout, or abort means server is offline
      if (error instanceof Error && error.name !== "AbortError") {
        console.debug("Server health check failed:", error.message);
      }
      return false;
    }
  }
}

export const api = new ApiClient();

