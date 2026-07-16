// ============================================================
// nav.ax API Client — centralized HTTP client
// ============================================================

const API_BASE = '/api/v1';

interface RequestOptions {
  method?: string;
  body?: unknown;
  params?: Record<string, string | number | boolean | undefined>;
  headers?: Record<string, string>;
  signal?: AbortSignal;
}

class ApiError extends Error {
  status: number;
  code: string;
  detail: string;

  constructor(status: number, code: string, message: string, detail: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.detail = detail;
  }
}

async function request<T>(endpoint: string, options: RequestOptions = {}): Promise<T> {
  const { method = 'GET', body, params, headers: extraHeaders, signal } = options;

  let url = `${API_BASE}${endpoint}`;
  if (params) {
    const searchParams = new URLSearchParams();
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined) {
        searchParams.set(key, String(value));
      }
    });
    const qs = searchParams.toString();
    if (qs) url += `?${qs}`;
  }

  const headers: Record<string, string> = { ...extraHeaders };
  const isFormData = typeof FormData !== 'undefined' && body instanceof FormData;
  if (body && !isFormData) {
    headers['Content-Type'] = 'application/json';
  }

  const response = await fetch(url, {
    method,
    headers,
    body: body ? (isFormData ? body : JSON.stringify(body)) : undefined,
    credentials: 'include',
    signal,
  });

  if (!response.ok) {
    let code = 'ERROR';
    let message = 'Request failed';
    let detail = '';
    try {
      const errData = await response.json();
      code = errData.code || 'ERROR';
      message = errData.meta?.message || errData.message || 'Request failed';
      detail = errData.meta?.detail || '';
    } catch {
      // ignore parse error
    }
    throw new ApiError(response.status, code, message, detail);
  }

  const data = await response.json();
  return data as T;
}

export interface AttachmentResponse {
  blob: Blob;
  filename: string;
  contentType: string;
}

async function requestAttachment(
  endpoint: string,
  params?: Record<string, string | number | boolean | undefined>,
): Promise<AttachmentResponse> {
  const searchParams = new URLSearchParams();
  Object.entries(params ?? {}).forEach(([key, value]) => {
    if (value !== undefined) searchParams.set(key, String(value));
  });
  const query = searchParams.toString();
  const response = await fetch(`${API_BASE}${endpoint}${query ? `?${query}` : ''}`, {
    credentials: 'include',
  });
  if (!response.ok) {
    let code = 'ERROR';
    let message = 'Request failed';
    let detail = '';
    try {
      const error = await response.json();
      code = error.code || code;
      message = error.meta?.message || error.message || message;
      detail = error.meta?.detail || detail;
    } catch {
      // 附件接口错误响应可能没有 JSON body。
    }
    throw new ApiError(response.status, code, message, detail);
  }
  const disposition = response.headers.get('Content-Disposition') ?? '';
  const encodedName = disposition.match(/filename\*=UTF-8''([^;]+)/i)?.[1];
  const plainName = disposition.match(/filename="?([^";]+)"?/i)?.[1];
  const filename = encodedName ? decodeURIComponent(encodedName) : (plainName ?? 'nav.ax-export');
  return {
    blob: await response.blob(),
    filename,
    contentType: response.headers.get('Content-Type') ?? 'application/octet-stream',
  };
}

export { request, requestAttachment, ApiError, API_BASE };
export type { RequestOptions };
