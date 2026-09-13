export interface KTPData {
	nik: string;
	nama: string;
	tempat_lahir: string;
	tanggal_lahir: string;
	jenis_kelamin: string;
	alamat: string;
	rt_rw: string;
	kelurahan: string;
	kecamatan: string;
	agama: string;
	status_perkawinan: string;
	pekerjaan: string;
	kewarganegaraan: string;
}

export type FieldConfidence = Record<string, number>;

export interface KTPResponse {
	id: string;
	status: "completed" | "failed" | "low_confidence" | string;
	confidence: number;
	latency_ms: number;
	data: KTPData;
	field_confidence: FieldConfidence;
	warnings?: string[];
}

export interface SIMData {
	nomor_sim: string;
	golongan: string;
	nama: string;
	tempat_lahir: string;
	tanggal_lahir: string;
	golongan_darah: string;
	jenis_kelamin: string;
	alamat: string;
	pekerjaan: string;
	polda: string;
	masa_berlaku: string;
}

export interface SIMResponse {
	id: string;
	document_type: "sim" | string;
	status?: "completed" | "failed" | "low_confidence" | string;
	confidence: number;
	processing: {
		latency_ms: number;
	};
	data: SIMData;
	field_confidence?: FieldConfidence;
	warnings?: string[];
}

export interface PassportData {
	passport_number: string;
	full_name: string;
	nationality: string;
	place_of_birth: string;
	date_of_birth: string;
	gender: string;
	issue_date: string;
	expiry_date: string;
	issuing_office: string;
	mrz_line1?: string;
	mrz_line2?: string;
}

export interface PassportResponse {
	id: string;
	document_type: "passport" | string;
	status?: "completed" | "failed" | "low_confidence" | string;
	confidence: number;
	processing: {
		latency_ms: number;
	};
	data: PassportData;
	field_confidence?: FieldConfidence;
	warnings?: string[];
}

export interface NPWPData {
	npwp: string;
	nama: string;
	nik?: string;
	alamat: string;
	kelurahan?: string;
	kecamatan?: string;
	kota_kabupaten?: string;
	provinsi?: string;
	kpp: string;
	tanggal_daftar?: string;
}

export interface NPWPResponse {
	id: string;
	document_type: "npwp" | string;
	status?: "completed" | "failed" | "low_confidence" | string;
	confidence: number;
	processing: {
		latency_ms: number;
	};
	data: NPWPData;
	field_confidence?: FieldConfidence;
	warnings?: string[];
}

export interface KKFamilyMember {
	nama: string;
	nik: string;
	jenis_kelamin: string;
	tempat_lahir: string;
	tanggal_lahir: string;
	agama: string;
	pendidikan?: string;
	jenis_pekerjaan?: string;
	golongan_darah?: string;
	status_perkawinan?: string;
	status_hubungan: string;
	kewarganegaraan?: string;
	nama_ayah?: string;
	nama_ibu?: string;
}

export interface KKData {
	nomor_kk: string;
	kepala_keluarga: string;
	alamat: string;
	rt_rw?: string;
	kode_pos?: string;
	kelurahan_desa?: string;
	kecamatan?: string;
	kabupaten_kota?: string;
	provinsi?: string;
	tanggal_dikeluarkan?: string;
	anggota_keluarga: KKFamilyMember[];
}

export interface KKResponse {
	id: string;
	document_type: "kk" | string;
	status?: "completed" | "failed" | "low_confidence" | string;
	confidence: number;
	processing: {
		latency_ms: number;
	};
	data: KKData;
	field_confidence?: FieldConfidence;
	warnings?: string[];
}

export interface InvoiceLineItem {
	description: string;
	quantity: number;
	unit_price: number;
	total_price: number;
}

export interface InvoiceData {
	invoice_number: string;
	invoice_date: string;
	due_date?: string;
	seller_name: string;
	seller_npwp?: string;
	seller_address?: string;
	buyer_name: string;
	buyer_npwp?: string;
	buyer_address?: string;
	currency: string;
	subtotal: number;
	discount?: number;
	dpp: number;
	ppn: number;
	grand_total: number;
	line_items: InvoiceLineItem[];
}

export interface InvoiceResponse {
	id: string;
	document_type: "invoice" | string;
	status?: "completed" | "failed" | "low_confidence" | string;
	confidence: number;
	processing: {
		latency_ms: number;
	};
	data: InvoiceData;
	field_confidence?: FieldConfidence;
	warnings?: string[];
}

export interface APIKeyListItem {
	id: string;
	name: string;
	prefix: string;
	masked_key: string;
	scopes: string[];
	environment: "live" | "test";
	last_used_at: string | null;
	expires_at: string | null;
	revoked_at: string | null;
	created_at: string;
}

export interface CreateKeyRequest {
	name: string;
	environment: "live" | "test";
	scopes: string[];
	expires_at?: string | null;
	org_id?: string;
}

export interface CreateKeyResponse {
	id: string;
	org_id: string;
	name: string;
	key: string;
	prefix: string;
	scopes: string[];
	environment: "live" | "test";
	expires_at: string | null;
	created_at: string;
}

export interface UsageSummary {
	total_requests: number;
	success_count: number;
	error_count: number;
	quota_limit: number;
	quota_remaining: number;
	p95_latency_ms: number;
	rate_limit_violations: number;
	billing_cycle_reset: string;
}

export interface DailyUsage {
	date: string;
	total_requests: number;
	success_count: number;
	error_count: number;
}

export interface EndpointUsage {
	endpoint: string;
	total_requests: number;
	avg_latency_ms: number;
}

export interface UsageRecord {
	id: string;
	org_id: string;
	api_key_id?: string | null;
	request_id: string;
	endpoint: string;
	status_code: number;
	latency_ms: number;
	timestamp: string;
}

export interface UsageRecordsResponse {
	data: UsageRecord[];
	total: number;
	limit: number;
	offset: number;
}

export interface OrganizationDetails {
	organization_id: string;
	organization_name: string;
	slug: string;
	plan_code: string;
	plan_name: string;
	active_keys_count: number;
	created_at: string;
}

export interface PlanDetails {
	plan_code: string;
	plan_name: string;
	monthly_quota: number;
	rate_limit_per_minute: number;
	billing_cycle_reset: string;
}

export interface UserMember {
	id: string;
	org_id: string;
	email: string;
	full_name: string;
	role: string;
	created_at: string;
}

export interface ApiErrorResponse {
	error: {
		code: string;
		message: string;
		request_id?: string;
	};
}

export interface RegisterRequest {
	full_name: string;
	email: string;
	password: string;
}

export interface RegisterResponse {
	status: string;
	email: string;
	verification_token?: string;
	message: string;
}

export interface VerifyEmailRequest {
	email: string;
	token?: string;
}

export interface VerifyEmailResponse {
	status: string;
	message: string;
}

export interface LoginRequest {
	email: string;
	password: string;
}

export interface LoginResponse {
	status?: string;
	access_token?: string;
	token_type?: string;
	expires_in?: number;
	user: {
		id: string;
		email: string;
		full_name: string;
		role: string;
	};
	organization?: {
		id: string;
		name: string;
		slug: string;
		plan_code: string;
	} | null;
}

export interface CreateOrganizationRequest {
	name: string;
	plan_code?: string;
}

export interface CreateOrganizationResponse {
	organization: {
		id: string;
		name: string;
		slug: string;
		plan_code: string;
		plan_name?: string;
		monthly_quota?: number;
		rate_limit_per_minute?: number;
	};
}

export interface CurrentUserResponse {
	user?: {
		id: string;
		email: string;
		full_name: string;
		roles?: string[];
	};
	api_key?: {
		id: string;
		name: string;
		prefix: string;
		environment: string;
		scopes: string[];
	};
	organization?: {
		id: string;
		name: string;
		slug: string;
		plan_code: string;
	} | null;
}
