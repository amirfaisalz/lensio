import type {
	APIKeyListItem,
	ApiErrorResponse,
	CreateKeyRequest,
	CreateKeyResponse,
	CreateOrganizationRequest,
	CreateOrganizationResponse,
	CurrentUserResponse,
	DailyUsage,
	EndpointUsage,
	InvoiceResponse,
	KKResponse,
	KTPResponse,
	ListOrganizationsResponse,
	LoginRequest,
	LoginResponse,
	NPWPResponse,
	OrganizationDetails,
	OrganizationMembership,
	PassportResponse,
	PlanDetails,
	RegisterRequest,
	RegisterResponse,
	SIMResponse,
	UsageRecordsResponse,
	UsageSummary,
	UserMember,
	VerifyEmailRequest,
	VerifyEmailResponse,
} from "../types/api";

declare global {
	interface Window {
		// Written by /config.js, which the dashboard container generates at start
		// from its API_URL env var, so one image serves every environment.
		__LENSIO_CONFIG__?: { apiUrl?: string };
	}
}

// resolveApiBase picks the API origin: the runtime config from the deployed
// container, then a build-time VITE_API_URL, then same-origin (the Vite dev
// server proxies /api to the local API).
export function resolveApiBase(
	runtimeApiUrl: string | undefined,
	buildApiUrl: string | undefined,
): string {
	return (runtimeApiUrl || buildApiUrl || "").replace(/\/+$/, "");
}

const API_BASE = resolveApiBase(
	typeof window !== "undefined" ? window.__LENSIO_CONFIG__?.apiUrl : undefined,
	import.meta.env.VITE_API_URL,
);

// Cache dashboard GETs briefly so fast sidebar navigation reuses data
// instead of bursting the per-org per-minute rate limit (free: 10 req/min).
const GET_CACHE_TTL_MS = 30_000;
// Single automatic retry on 429 keeps occasional bursts invisible to users.
const MAX_429_WAIT_MS = 5_000;

interface GetCacheEntry {
	expiresAt: number;
	data: unknown;
}

export interface ApiClientError extends Error {
	code?: string;
	status?: number;
}

class ApiClient {
	private activeApiKey: string | null = null;
	private getCache = new Map<string, GetCacheEntry>();
	private inflightGets = new Map<string, Promise<unknown>>();

	public getApiKey(): string | null {
		return this.activeApiKey;
	}

	public setApiKey(key: string | null): void {
		this.activeApiKey = key;
		this.clearCache();
	}

	public clearCache(): void {
		this.getCache.clear();
		this.inflightGets.clear();
	}

	private getHeaders(isMultipart = false): HeadersInit {
		const headers: Record<string, string> = {};
		if (!isMultipart) {
			headers["Content-Type"] = "application/json";
			headers.Accept = "application/json";
		}
		if (this.activeApiKey) {
			headers.Authorization = `Bearer ${this.activeApiKey}`;
		}
		return headers;
	}

	private async fetchWithAuth(
		url: string,
		init: RequestInit = {},
	): Promise<Response> {
		const isMultipart = init.body instanceof FormData;
		const defaultHeaders = this.getHeaders(isMultipart);
		const mergedHeaders = {
			...defaultHeaders,
			...(init.headers as Record<string, string> | undefined),
		};

		return fetch(url, {
			...init,
			credentials: "include",
			headers: mergedHeaders,
		});
	}

	private async fetchJSON<T>(
		url: string,
		init: RequestInit = {},
		useCache = false,
	): Promise<T> {
		if (useCache) {
			const cached = this.getCache.get(url);
			if (cached && cached.expiresAt > Date.now()) {
				return cached.data as T;
			}
			const pending = this.inflightGets.get(url);
			if (pending) {
				return pending as Promise<T>;
			}
		}

		const task = this.fetchWithRetry<T>(url, init);
		if (!useCache) {
			return task;
		}
		this.inflightGets.set(url, task);
		try {
			const data = await task;
			this.getCache.set(url, {
				expiresAt: Date.now() + GET_CACHE_TTL_MS,
				data,
			});
			return data;
		} finally {
			this.inflightGets.delete(url);
		}
	}

	private async fetchWithRetry<T>(url: string, init: RequestInit): Promise<T> {
		const res = await this.fetchWithAuth(url, {
			...init,
			method: init.method ?? "GET",
		});
		if (res.status === 429) {
			const headers = res.headers as Headers | undefined;
			const retryAfterSec = Number(headers?.get("Retry-After") ?? "0");
			const waitMs = Math.min(
				Math.max(Math.trunc(retryAfterSec * 1000) || 0, 0),
				MAX_429_WAIT_MS,
			);
			try {
				await res.json();
			} catch {
				// ignore drain errors before retry
			}
			if (waitMs > 0) {
				await new Promise((resolve) => setTimeout(resolve, waitMs));
			}
			const retryRes = await this.fetchWithAuth(url, {
				...init,
				method: init.method ?? "GET",
			});
			return this.handleResponse<T>(retryRes);
		}
		return this.handleResponse<T>(res);
	}

	private async handleResponse<T>(res: Response): Promise<T> {
		if (!res.ok) {
			let errorMessage = `Request failed with status ${res.status}`;
			let errorCode = "";
			try {
				const errorData = (await res.json()) as ApiErrorResponse;
				if (errorData?.error?.message) {
					errorMessage = errorData.error.message;
				}
				if (errorData?.error?.code) {
					errorCode = errorData.error.code;
				}
			} catch {
				// use fallback status error
			}
			if (res.status === 429) {
				const headers = res.headers as Headers | undefined;
				const retryAfter = headers?.get("Retry-After");
				errorMessage = retryAfter
					? `Terlalu banyak permintaan, coba lagi dalam ${retryAfter} detik`
					: "Terlalu banyak permintaan, coba lagi beberapa detik lagi";
				if (!errorCode) {
					errorCode = "rate_limit_exceeded";
				}
			}
			const err = new Error(errorMessage) as ApiClientError;
			err.code = errorCode;
			err.status = res.status;
			throw err;
		}
		return res.json() as Promise<T>;
	}

	public async checkHealth(): Promise<{ status: string; timestamp: string }> {
		const res = await this.fetchWithAuth(`${API_BASE}/health`, {
			method: "GET",
		});
		return this.handleResponse<{ status: string; timestamp: string }>(res);
	}

	public async fetchCurrentUser(): Promise<CurrentUserResponse> {
		const res = await this.fetchWithAuth(`${API_BASE}/api/v1/auth/me`, {
			method: "GET",
		});
		return this.handleResponse<CurrentUserResponse>(res);
	}

	public async logout(): Promise<{ status: string; message: string }> {
		const res = await this.fetchWithAuth(`${API_BASE}/api/v1/auth/logout`, {
			method: "POST",
		});
		const result = await this.handleResponse<{
			status: string;
			message: string;
		}>(res);
		this.clearCache();
		return result;
	}

	public async fetchUsageSummary(orgId?: string): Promise<UsageSummary> {
		const url = orgId
			? `${API_BASE}/api/v1/usage?org_id=${encodeURIComponent(orgId)}`
			: `${API_BASE}/api/v1/usage`;
		return this.fetchJSON<UsageSummary>(url, {}, true);
	}

	public async fetchDailyUsage(orgId?: string): Promise<DailyUsage[]> {
		const url = orgId
			? `${API_BASE}/api/v1/usage/daily?org_id=${encodeURIComponent(orgId)}`
			: `${API_BASE}/api/v1/usage/daily`;
		const result = await this.fetchJSON<{ data: DailyUsage[] }>(url, {}, true);
		return result.data || [];
	}

	public async fetchEndpointUsage(orgId?: string): Promise<EndpointUsage[]> {
		const url = orgId
			? `${API_BASE}/api/v1/usage/endpoints?org_id=${encodeURIComponent(orgId)}`
			: `${API_BASE}/api/v1/usage/endpoints`;
		const result = await this.fetchJSON<{ data: EndpointUsage[] }>(
			url,
			{},
			true,
		);
		return result.data || [];
	}

	public async fetchUsageRecords(params?: {
		org_id?: string;
		limit?: number;
		offset?: number;
		status_code?: number;
		endpoint?: string;
	}): Promise<UsageRecordsResponse> {
		const query = new URLSearchParams();
		if (params?.org_id) query.set("org_id", params.org_id);
		if (params?.limit) query.set("limit", params.limit.toString());
		if (params?.offset !== undefined)
			query.set("offset", params.offset.toString());
		if (params?.status_code)
			query.set("status_code", params.status_code.toString());
		if (params?.endpoint) query.set("endpoint", params.endpoint);

		const qs = query.toString();
		const url = `${API_BASE}/api/v1/usage/records${qs ? `?${qs}` : ""}`;
		return this.fetchJSON<UsageRecordsResponse>(url, {}, true);
	}

	public async fetchAPIKeys(orgId?: string): Promise<APIKeyListItem[]> {
		const url = orgId
			? `${API_BASE}/api/v1/auth/api-keys?org_id=${encodeURIComponent(orgId)}`
			: `${API_BASE}/api/v1/auth/api-keys`;
		const result = await this.fetchJSON<{ data: APIKeyListItem[] }>(
			url,
			{},
			true,
		);
		return result.data || [];
	}

	public async createAPIKey(req: CreateKeyRequest): Promise<CreateKeyResponse> {
		const res = await this.fetchWithAuth(`${API_BASE}/api/v1/auth/api-keys`, {
			method: "POST",
			body: JSON.stringify(req),
		});
		const created = await this.handleResponse<CreateKeyResponse>(res);
		this.clearCache();
		return created;
	}

	public async revokeAPIKey(
		id: string,
		orgId?: string,
	): Promise<{ message: string; id: string }> {
		const url = orgId
			? `${API_BASE}/api/v1/auth/api-keys/${id}?org_id=${encodeURIComponent(orgId)}`
			: `${API_BASE}/api/v1/auth/api-keys/${id}`;
		const res = await this.fetchWithAuth(url, {
			method: "DELETE",
		});
		const revoked = await this.handleResponse<{ message: string; id: string }>(
			res,
		);
		this.clearCache();
		return revoked;
	}

	public async fetchAccount(orgId?: string): Promise<OrganizationDetails> {
		const url = orgId
			? `${API_BASE}/api/v1/account?org_id=${encodeURIComponent(orgId)}`
			: `${API_BASE}/api/v1/account`;
		return this.fetchJSON<OrganizationDetails>(url, {}, true);
	}

	public async fetchAccountPlan(orgId?: string): Promise<PlanDetails> {
		const url = orgId
			? `${API_BASE}/api/v1/account/plan?org_id=${encodeURIComponent(orgId)}`
			: `${API_BASE}/api/v1/account/plan`;
		return this.fetchJSON<PlanDetails>(url, {}, true);
	}

	public async updateAccountPlan(
		planCode: string,
		orgId?: string,
	): Promise<{ message: string; plan_code: string }> {
		const url = orgId
			? `${API_BASE}/api/v1/account/plan?org_id=${encodeURIComponent(orgId)}`
			: `${API_BASE}/api/v1/account/plan`;
		const res = await this.fetchWithAuth(url, {
			method: "PUT",
			body: JSON.stringify({ plan_code: planCode }),
		});
		const updated = await this.handleResponse<{
			message: string;
			plan_code: string;
		}>(res);
		this.clearCache();
		return updated;
	}

	public async fetchAccountMembers(orgId?: string): Promise<UserMember[]> {
		const url = orgId
			? `${API_BASE}/api/v1/account/members?org_id=${encodeURIComponent(orgId)}`
			: `${API_BASE}/api/v1/account/members`;
		const result = await this.fetchJSON<{ data: UserMember[] }>(url, {}, true);
		return result.data || [];
	}

	public async executeKTPOCR(
		file: File | Blob,
		apiKeyOverride?: string,
	): Promise<KTPResponse> {
		const formData = new FormData();
		formData.append("document", file);

		const headers: Record<string, string> = {};
		const keyToUse = apiKeyOverride || this.activeApiKey;
		if (keyToUse) {
			headers.Authorization = `Bearer ${keyToUse}`;
		}

		const res = await this.fetchWithAuth(`${API_BASE}/api/v1/ocr/ktp`, {
			method: "POST",
			body: formData,
			headers,
		});
		return this.handleResponse<KTPResponse>(res);
	}

	public async executeSIMOCR(
		file: File | Blob,
		apiKeyOverride?: string,
	): Promise<SIMResponse> {
		const formData = new FormData();
		formData.append("document", file);

		const headers: Record<string, string> = {};
		const keyToUse = apiKeyOverride || this.activeApiKey;
		if (keyToUse) {
			headers.Authorization = `Bearer ${keyToUse}`;
		}

		const res = await this.fetchWithAuth(`${API_BASE}/api/v1/ocr/sim`, {
			method: "POST",
			body: formData,
			headers,
		});
		return this.handleResponse<SIMResponse>(res);
	}

	public async executePassportOCR(
		file: File | Blob,
		apiKeyOverride?: string,
	): Promise<PassportResponse> {
		const formData = new FormData();
		formData.append("document", file);

		const headers: Record<string, string> = {};
		const keyToUse = apiKeyOverride || this.activeApiKey;
		if (keyToUse) {
			headers.Authorization = `Bearer ${keyToUse}`;
		}

		const res = await this.fetchWithAuth(`${API_BASE}/api/v1/ocr/passport`, {
			method: "POST",
			body: formData,
			headers,
		});
		return this.handleResponse<PassportResponse>(res);
	}

	public async executeNPWPOCR(
		file: File | Blob,
		apiKeyOverride?: string,
	): Promise<NPWPResponse> {
		const formData = new FormData();
		formData.append("document", file);

		const headers: Record<string, string> = {};
		const keyToUse = apiKeyOverride || this.activeApiKey;
		if (keyToUse) {
			headers.Authorization = `Bearer ${keyToUse}`;
		}

		const res = await this.fetchWithAuth(`${API_BASE}/api/v1/ocr/npwp`, {
			method: "POST",
			body: formData,
			headers,
		});
		return this.handleResponse<NPWPResponse>(res);
	}

	public async executeKKOCR(
		file: File | Blob,
		apiKeyOverride?: string,
	): Promise<KKResponse> {
		const formData = new FormData();
		formData.append("document", file);

		const headers: Record<string, string> = {};
		const keyToUse = apiKeyOverride || this.activeApiKey;
		if (keyToUse) {
			headers.Authorization = `Bearer ${keyToUse}`;
		}

		const res = await this.fetchWithAuth(`${API_BASE}/api/v1/ocr/kk`, {
			method: "POST",
			body: formData,
			headers,
		});
		return this.handleResponse<KKResponse>(res);
	}

	public async executeInvoiceOCR(
		file: File | Blob,
		apiKeyOverride?: string,
	): Promise<InvoiceResponse> {
		const formData = new FormData();
		formData.append("document", file);

		const headers: Record<string, string> = {};
		const keyToUse = apiKeyOverride || this.activeApiKey;
		if (keyToUse) {
			headers.Authorization = `Bearer ${keyToUse}`;
		}

		const res = await this.fetchWithAuth(`${API_BASE}/api/v1/ocr/invoice`, {
			method: "POST",
			body: formData,
			headers,
		});
		return this.handleResponse<InvoiceResponse>(res);
	}

	public async register(req: RegisterRequest): Promise<RegisterResponse> {
		const res = await this.fetchWithAuth(`${API_BASE}/api/v1/auth/register`, {
			method: "POST",
			body: JSON.stringify(req),
		});
		return this.handleResponse<RegisterResponse>(res);
	}

	public async verifyEmail(
		req: VerifyEmailRequest,
	): Promise<VerifyEmailResponse> {
		const res = await this.fetchWithAuth(
			`${API_BASE}/api/v1/auth/verify-email`,
			{
				method: "POST",
				body: JSON.stringify(req),
			},
		);
		return this.handleResponse<VerifyEmailResponse>(res);
	}

	public async login(req: LoginRequest): Promise<LoginResponse> {
		const res = await this.fetchWithAuth(`${API_BASE}/api/v1/auth/login`, {
			method: "POST",
			body: JSON.stringify(req),
		});
		return this.handleResponse<LoginResponse>(res);
	}

	public async createOrganization(
		req: CreateOrganizationRequest,
	): Promise<CreateOrganizationResponse> {
		const res = await this.fetchWithAuth(
			`${API_BASE}/api/v1/account/organizations`,
			{
				method: "POST",
				body: JSON.stringify(req),
			},
		);
		const created = await this.handleResponse<CreateOrganizationResponse>(res);
		this.clearCache();
		return created;
	}

	/**
	 * Lists the organizations the signed-in user belongs to.
	 *
	 * The dashboard used to keep this list in localStorage, which the user can
	 * edit and which survived a logout into the next account's session. The
	 * server is the only thing that knows the real membership set.
	 */
	public async listOrganizations(): Promise<OrganizationMembership[]> {
		const res = await this.fetchWithAuth(
			`${API_BASE}/api/v1/account/organizations`,
		);
		const body = await this.handleResponse<ListOrganizationsResponse>(res);
		return body.data ?? [];
	}
}

export const api = new ApiClient();
