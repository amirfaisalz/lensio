import type {
	APIKeyListItem,
	ApiErrorResponse,
	CreateKeyRequest,
	CreateKeyResponse,
	DailyUsage,
	EndpointUsage,
	KTPResponse,
	OrganizationDetails,
	PlanDetails,
	UsageRecordsResponse,
	UsageSummary,
	UserMember,
} from "../types/api";

const API_BASE = import.meta.env.VITE_API_URL || "";
const STORAGE_KEY_API_KEY = "nusaid_api_key";

class ApiClient {
	private activeApiKey: string | null = null;

	constructor() {
		if (typeof window !== "undefined") {
			this.activeApiKey = localStorage.getItem(STORAGE_KEY_API_KEY);
		}
	}

	public getApiKey(): string | null {
		return this.activeApiKey;
	}

	public setApiKey(key: string | null): void {
		this.activeApiKey = key;
		if (typeof window !== "undefined") {
			if (key) {
				localStorage.setItem(STORAGE_KEY_API_KEY, key);
			} else {
				localStorage.removeItem(STORAGE_KEY_API_KEY);
			}
		}
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

	private async handleResponse<T>(res: Response): Promise<T> {
		if (!res.ok) {
			let errorMessage = `Request failed with status ${res.status}`;
			try {
				const errorData = (await res.json()) as ApiErrorResponse;
				if (errorData?.error?.message) {
					errorMessage = errorData.error.message;
				}
			} catch {
				// use fallback status error
			}
			throw new Error(errorMessage);
		}
		return res.json() as Promise<T>;
	}

	public async checkHealth(): Promise<{ status: string; timestamp: string }> {
		const res = await fetch(`${API_BASE}/health`, {
			method: "GET",
			headers: this.getHeaders(),
		});
		return this.handleResponse<{ status: string; timestamp: string }>(res);
	}

	public async fetchUsageSummary(): Promise<UsageSummary> {
		const res = await fetch(`${API_BASE}/api/v1/usage`, {
			method: "GET",
			headers: this.getHeaders(),
		});
		return this.handleResponse<UsageSummary>(res);
	}

	public async fetchDailyUsage(): Promise<DailyUsage[]> {
		const res = await fetch(`${API_BASE}/api/v1/usage/daily`, {
			method: "GET",
			headers: this.getHeaders(),
		});
		const result = await this.handleResponse<{ data: DailyUsage[] }>(res);
		return result.data || [];
	}

	public async fetchEndpointUsage(): Promise<EndpointUsage[]> {
		const res = await fetch(`${API_BASE}/api/v1/usage/endpoints`, {
			method: "GET",
			headers: this.getHeaders(),
		});
		const result = await this.handleResponse<{ data: EndpointUsage[] }>(res);
		return result.data || [];
	}

	public async fetchUsageRecords(params?: {
		limit?: number;
		offset?: number;
		status_code?: number;
		endpoint?: string;
	}): Promise<UsageRecordsResponse> {
		const query = new URLSearchParams();
		if (params?.limit) query.set("limit", params.limit.toString());
		if (params?.offset !== undefined)
			query.set("offset", params.offset.toString());
		if (params?.status_code)
			query.set("status_code", params.status_code.toString());
		if (params?.endpoint) query.set("endpoint", params.endpoint);

		const qs = query.toString();
		const url = `${API_BASE}/api/v1/usage/records${qs ? `?${qs}` : ""}`;
		const res = await fetch(url, {
			method: "GET",
			headers: this.getHeaders(),
		});
		return this.handleResponse<UsageRecordsResponse>(res);
	}

	public async fetchAPIKeys(): Promise<APIKeyListItem[]> {
		const res = await fetch(`${API_BASE}/api/v1/auth/api-keys`, {
			method: "GET",
			headers: this.getHeaders(),
		});
		const result = await this.handleResponse<{ data: APIKeyListItem[] }>(res);
		return result.data || [];
	}

	public async createAPIKey(req: CreateKeyRequest): Promise<CreateKeyResponse> {
		const res = await fetch(`${API_BASE}/api/v1/auth/api-keys`, {
			method: "POST",
			headers: this.getHeaders(),
			body: JSON.stringify(req),
		});
		return this.handleResponse<CreateKeyResponse>(res);
	}

	public async revokeAPIKey(
		id: string,
	): Promise<{ message: string; id: string }> {
		const res = await fetch(`${API_BASE}/api/v1/auth/api-keys/${id}`, {
			method: "DELETE",
			headers: this.getHeaders(),
		});
		return this.handleResponse<{ message: string; id: string }>(res);
	}

	public async fetchAccount(): Promise<OrganizationDetails> {
		const res = await fetch(`${API_BASE}/api/v1/account`, {
			method: "GET",
			headers: this.getHeaders(),
		});
		return this.handleResponse<OrganizationDetails>(res);
	}

	public async fetchAccountPlan(): Promise<PlanDetails> {
		const res = await fetch(`${API_BASE}/api/v1/account/plan`, {
			method: "GET",
			headers: this.getHeaders(),
		});
		return this.handleResponse<PlanDetails>(res);
	}

	public async updateAccountPlan(
		planCode: string,
	): Promise<{ message: string; plan_code: string }> {
		const res = await fetch(`${API_BASE}/api/v1/account/plan`, {
			method: "PUT",
			headers: this.getHeaders(),
			body: JSON.stringify({ plan_code: planCode }),
		});
		return this.handleResponse<{ message: string; plan_code: string }>(res);
	}

	public async fetchAccountMembers(): Promise<UserMember[]> {
		const res = await fetch(`${API_BASE}/api/v1/account/members`, {
			method: "GET",
			headers: this.getHeaders(),
		});
		const result = await this.handleResponse<{ data: UserMember[] }>(res);
		return result.data || [];
	}

	public async executeKTPOCR(file: File | Blob): Promise<KTPResponse> {
		const formData = new FormData();
		formData.append("document", file);

		const res = await fetch(`${API_BASE}/api/v1/ocr/ktp`, {
			method: "POST",
			headers: this.getHeaders(true),
			body: formData,
		});
		return this.handleResponse<KTPResponse>(res);
	}
}

export const api = new ApiClient();
