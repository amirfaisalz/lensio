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
	KTPResponse,
	LoginRequest,
	LoginResponse,
	OrganizationDetails,
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

const API_BASE =
	import.meta.env.VITE_API_URL ||
	(typeof window !== "undefined" &&
	window.location.hostname.includes("ca-dash-")
		? `${window.location.protocol}//${window.location.hostname.replace("ca-dash-", "ca-api-")}`
		: "");

export interface ApiClientError extends Error {
	code?: string;
	status?: number;
}

class ApiClient {
	private activeApiKey: string | null = null;

	public getApiKey(): string | null {
		return this.activeApiKey;
	}

	public setApiKey(key: string | null): void {
		this.activeApiKey = key;
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
		return this.handleResponse<{ status: string; message: string }>(res);
	}

	public async fetchUsageSummary(orgId?: string): Promise<UsageSummary> {
		const url = orgId
			? `${API_BASE}/api/v1/usage?org_id=${encodeURIComponent(orgId)}`
			: `${API_BASE}/api/v1/usage`;
		const res = await this.fetchWithAuth(url, {
			method: "GET",
		});
		return this.handleResponse<UsageSummary>(res);
	}

	public async fetchDailyUsage(orgId?: string): Promise<DailyUsage[]> {
		const url = orgId
			? `${API_BASE}/api/v1/usage/daily?org_id=${encodeURIComponent(orgId)}`
			: `${API_BASE}/api/v1/usage/daily`;
		const res = await this.fetchWithAuth(url, {
			method: "GET",
		});
		const result = await this.handleResponse<{ data: DailyUsage[] }>(res);
		return result.data || [];
	}

	public async fetchEndpointUsage(orgId?: string): Promise<EndpointUsage[]> {
		const url = orgId
			? `${API_BASE}/api/v1/usage/endpoints?org_id=${encodeURIComponent(orgId)}`
			: `${API_BASE}/api/v1/usage/endpoints`;
		const res = await this.fetchWithAuth(url, {
			method: "GET",
		});
		const result = await this.handleResponse<{ data: EndpointUsage[] }>(res);
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
		const res = await this.fetchWithAuth(url, {
			method: "GET",
		});
		return this.handleResponse<UsageRecordsResponse>(res);
	}

	public async fetchAPIKeys(orgId?: string): Promise<APIKeyListItem[]> {
		const url = orgId
			? `${API_BASE}/api/v1/auth/api-keys?org_id=${encodeURIComponent(orgId)}`
			: `${API_BASE}/api/v1/auth/api-keys`;
		const res = await this.fetchWithAuth(url, {
			method: "GET",
		});
		const result = await this.handleResponse<{ data: APIKeyListItem[] }>(res);
		return result.data || [];
	}

	public async createAPIKey(req: CreateKeyRequest): Promise<CreateKeyResponse> {
		const res = await this.fetchWithAuth(`${API_BASE}/api/v1/auth/api-keys`, {
			method: "POST",
			body: JSON.stringify(req),
		});
		return this.handleResponse<CreateKeyResponse>(res);
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
		return this.handleResponse<{ message: string; id: string }>(res);
	}

	public async fetchAccount(orgId?: string): Promise<OrganizationDetails> {
		const url = orgId
			? `${API_BASE}/api/v1/account?org_id=${encodeURIComponent(orgId)}`
			: `${API_BASE}/api/v1/account`;
		const res = await this.fetchWithAuth(url, {
			method: "GET",
		});
		return this.handleResponse<OrganizationDetails>(res);
	}

	public async fetchAccountPlan(orgId?: string): Promise<PlanDetails> {
		const url = orgId
			? `${API_BASE}/api/v1/account/plan?org_id=${encodeURIComponent(orgId)}`
			: `${API_BASE}/api/v1/account/plan`;
		const res = await this.fetchWithAuth(url, {
			method: "GET",
		});
		return this.handleResponse<PlanDetails>(res);
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
		return this.handleResponse<{ message: string; plan_code: string }>(res);
	}

	public async fetchAccountMembers(orgId?: string): Promise<UserMember[]> {
		const url = orgId
			? `${API_BASE}/api/v1/account/members?org_id=${encodeURIComponent(orgId)}`
			: `${API_BASE}/api/v1/account/members`;
		const res = await this.fetchWithAuth(url, {
			method: "GET",
		});
		const result = await this.handleResponse<{ data: UserMember[] }>(res);
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
		return this.handleResponse<CreateOrganizationResponse>(res);
	}
}

export const api = new ApiClient();
