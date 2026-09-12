import {
	AlertOctagon,
	Building2,
	Check,
	Copy,
	KeyRound,
	Plus,
	RefreshCw,
	ShieldAlert,
	Trash2,
} from "lucide-react";
import type React from "react";
import { useCallback, useEffect, useState } from "react";
import { Badge } from "../components/common/Badge";
import { Modal } from "../components/common/Modal";
import { TableSkeleton } from "../components/common/Skeleton";
import { useAuth } from "../context/AuthContext";
import { api } from "../services/api";
import type { APIKeyListItem, CreateKeyResponse } from "../types/api";

export const APIKeysPage: React.FC = () => {
	const { currentOrg, setApiKey, createOrganization } = useAuth();
	const [keys, setKeys] = useState<APIKeyListItem[]>([]);
	const [isLoading, setIsLoading] = useState(false);
	const [error, setError] = useState<string | null>(null);

	// Org Creation Modal State (from Empty State)
	const [isOrgModalOpen, setIsOrgModalOpen] = useState(false);
	const [newOrgName, setNewOrgName] = useState("");
	const [newOrgPlan, setNewOrgPlan] = useState("free");

	// Create Key Modal State
	const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
	const [newKeyName, setNewKeyName] = useState("");
	const [newKeyEnv, setNewKeyEnv] = useState<"live" | "test">("live");
	const [newKeyScopes, setNewKeyScopes] = useState<string[]>([
		"ocr:read",
		"ocr:write",
		"usage:read",
	]);
	const [isCreating, setIsCreating] = useState(false);

	// Plaintext Reveal Modal
	const [createdKeyData, setCreatedKeyData] =
		useState<CreateKeyResponse | null>(null);
	const [copied, setCopied] = useState(false);

	// Revoke Modal State
	const [keyToRevoke, setKeyToRevoke] = useState<APIKeyListItem | null>(null);
	const [isRevoking, setIsRevoking] = useState(false);

	const loadKeys = useCallback(async () => {
		if (!currentOrg) {
			setKeys([]);
			setIsLoading(false);
			return;
		}

		try {
			setIsLoading(true);
			setError(null);
			const data = await api.fetchAPIKeys(currentOrg.id);
			setKeys(data);
		} catch (err) {
			setError(err instanceof Error ? err.message : "Failed to load API keys");
		} finally {
			setIsLoading(false);
		}
	}, [currentOrg]);

	useEffect(() => {
		loadKeys();
	}, [loadKeys]);

	const handleCreateOrg = (e: React.FormEvent) => {
		e.preventDefault();
		if (!newOrgName.trim()) return;
		createOrganization(newOrgName.trim(), newOrgPlan);
		setNewOrgName("");
		setIsOrgModalOpen(false);
	};

	const handleCreateKey = async (e: React.FormEvent) => {
		e.preventDefault();
		if (!newKeyName.trim()) return;

		try {
			setIsCreating(true);
			const created = await api.createAPIKey({
				name: newKeyName.trim(),
				environment: newKeyEnv,
				scopes: newKeyScopes,
				org_id: currentOrg?.id,
			});

			setIsCreateModalOpen(false);
			setNewKeyName("");
			setCreatedKeyData(created);
			await loadKeys();
		} catch (err) {
			alert(err instanceof Error ? err.message : "Failed to create key");
		} finally {
			setIsCreating(false);
		}
	};

	const handleCopyKey = () => {
		if (!createdKeyData) return;
		if (navigator.clipboard?.writeText) {
			navigator.clipboard.writeText(createdKeyData.key);
		}
		setCopied(true);
		setTimeout(() => setCopied(false), 2000);
	};

	const handleUseCreatedKey = () => {
		if (!createdKeyData) return;
		setApiKey(createdKeyData.key);
		setCreatedKeyData(null);
	};

	const handleRevokeKey = async () => {
		if (!keyToRevoke) return;

		try {
			setIsRevoking(true);
			await api.revokeAPIKey(keyToRevoke.id, currentOrg?.id);
			setKeyToRevoke(null);
			await loadKeys();
		} catch (err) {
			alert(err instanceof Error ? err.message : "Failed to revoke key");
		} finally {
			setIsRevoking(false);
		}
	};

	// REQUIREMENT 4: EMPTY STATE WHEN NO ORGANIZATION EXISTS
	if (!currentOrg) {
		return (
			<div className="space-y-6 animate-in fade-in duration-200">
				<div>
					<h2 className="text-xl font-bold text-slate-900 tracking-tight">
						API Key Management
					</h2>
					<p className="text-xs text-slate-500">
						Securely generate, scope, and manage authentication credentials.
					</p>
				</div>

				<div className="bg-white rounded-2xl border border-slate-200 p-8 sm:p-14 text-center max-w-xl mx-auto shadow-xs my-8">
					<div className="w-16 h-16 rounded-2xl bg-[#E7F3FF] text-[#1877F2] flex items-center justify-center mx-auto mb-4 shadow-xs">
						<Building2 className="w-8 h-8" />
					</div>
					<h3 className="text-lg font-bold text-slate-900 tracking-tight">
						Organisasi Diperlukan
					</h3>
					<p className="text-xs text-slate-600 mt-2 leading-relaxed max-w-md mx-auto">
						Anda harus membuat atau memilih organisasi terlebih dahulu sebelum
						dapat membuat API Key. Setiap API Key terikat pada kuota bulanan dan
						kebijakan keamanan organisasi Anda.
					</p>
					<button
						type="button"
						onClick={() => setIsOrgModalOpen(true)}
						className="mt-6 inline-flex items-center gap-2 px-4 py-2.5 text-xs font-semibold text-white bg-[#1877F2] hover:bg-[#166FE5] rounded-xl shadow-xs transition-colors cursor-pointer"
					>
						<Plus className="w-4 h-4" />
						<span>Buat Organisasi Sekarang</span>
					</button>
				</div>

				{/* Create Org Modal */}
				<Modal
					isOpen={isOrgModalOpen}
					onClose={() => setIsOrgModalOpen(false)}
					title="Buat Organisasi Baru"
				>
					<form onSubmit={handleCreateOrg} className="space-y-4">
						<p className="text-xs text-slate-600">
							Tentukan nama organisasi untuk mengaktifkan kuota API dan
							pembuatan API Key.
						</p>

						<div>
							<label
								htmlFor="apikey-new-org-name"
								className="block text-xs font-semibold text-slate-700 mb-1"
							>
								Nama Organisasi / Perusahaan
							</label>
							<div className="relative">
								<Building2 className="w-4 h-4 text-slate-400 absolute left-3 top-2.5" />
								<input
									id="apikey-new-org-name"
									type="text"
									placeholder="misal: PT Fintech Nusantara"
									value={newOrgName}
									onChange={(e) => setNewOrgName(e.target.value)}
									className="w-full pl-9 pr-3 py-2 text-xs border border-slate-300 rounded-lg focus:outline-hidden focus:border-[#1877F2] focus:ring-2 focus:ring-[#1877F2]/20"
								/>
							</div>
						</div>

						<div>
							<label
								htmlFor="apikey-org-plan"
								className="block text-xs font-semibold text-slate-700 mb-1"
							>
								Paket Langganan
							</label>
							<select
								id="apikey-org-plan"
								value={newOrgPlan}
								onChange={(e) => setNewOrgPlan(e.target.value)}
								className="w-full px-3 py-2 text-xs border border-slate-300 rounded-lg bg-white focus:outline-hidden focus:border-[#1877F2] focus:ring-2 focus:ring-[#1877F2]/20"
							>
								<option value="free">
									Free Tier (100 req/bulan, 10 req/min) - Gratis
								</option>
								<option value="starter">
									Starter Tier (1,000 req/bulan, 30 req/min)
								</option>
								<option value="pro">
									Pro Tier (10,000 req/bulan, 100 req/min)
								</option>
							</select>
						</div>

						<div className="flex justify-end gap-2 pt-2">
							<button
								type="button"
								onClick={() => setIsOrgModalOpen(false)}
								className="px-3 py-2 text-xs font-medium text-slate-600 hover:text-slate-800 bg-white border border-slate-200 rounded-lg hover:bg-slate-50 transition-colors"
							>
								Batal
							</button>
							<button
								type="submit"
								disabled={!newOrgName.trim()}
								className="px-4 py-2 text-xs font-semibold text-white bg-[#1877F2] hover:bg-[#166FE5] disabled:opacity-50 rounded-lg transition-colors cursor-pointer"
							>
								Buat Organisasi & Lanjutkan
							</button>
						</div>
					</form>
				</Modal>
			</div>
		);
	}

	return (
		<div className="space-y-6 animate-in fade-in duration-200">
			{/* Top Banner */}
			<div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 sm:gap-4">
				<div>
					<h2 className="text-xl font-bold text-slate-900 tracking-tight">
						API Key Management
					</h2>
					<p className="text-xs text-slate-500">
						Organisasi aktif:{" "}
						<strong className="text-slate-800">{currentOrg.name}</strong> •
						Kelola token otentikasi aplikasi Anda.
					</p>
				</div>

				<div className="flex items-center gap-2 w-full sm:w-auto justify-end">
					<button
						type="button"
						onClick={loadKeys}
						disabled={isLoading}
						className="p-2 text-slate-600 bg-white border border-slate-200 rounded-lg hover:bg-slate-50 transition-colors cursor-pointer"
						title="Refresh keys list"
					>
						<RefreshCw
							className={`w-4 h-4 ${isLoading ? "animate-spin" : ""}`}
						/>
					</button>

					<button
						type="button"
						onClick={() => setIsCreateModalOpen(true)}
						className="inline-flex items-center justify-center gap-1.5 px-3.5 py-2 text-xs font-semibold text-white bg-[#1877F2] hover:bg-[#166FE5] rounded-lg shadow-xs transition-colors cursor-pointer flex-1 sm:flex-initial"
					>
						<Plus className="w-4 h-4" />
						<span>Create New Key</span>
					</button>
				</div>
			</div>

			{error && (
				<div className="p-4 bg-rose-50 border border-rose-200 rounded-xl text-xs text-rose-700 flex items-center justify-between">
					<span>{error}</span>
					<button
						type="button"
						onClick={loadKeys}
						className="font-semibold underline cursor-pointer"
					>
						Retry
					</button>
				</div>
			)}

			{/* Keys Table Container */}
			<div className="bg-white rounded-xl border border-slate-200 shadow-xs overflow-hidden">
				<div className="overflow-x-auto">
					<table className="w-full text-left text-xs text-slate-600 min-w-[680px]">
						<thead className="bg-slate-50 border-b border-slate-200 text-[11px] font-semibold text-slate-500 uppercase tracking-wider">
							<tr>
								<th className="px-6 py-3.5">Name</th>
								<th className="px-6 py-3.5">Key Prefix</th>
								<th className="px-6 py-3.5">Environment</th>
								<th className="px-6 py-3.5">Scopes</th>
								<th className="px-6 py-3.5">Status</th>
								<th className="px-6 py-3.5">Last Used</th>
								<th className="px-6 py-3.5 text-right">Actions</th>
							</tr>
						</thead>
						<tbody className="divide-y divide-slate-100 font-medium">
							{isLoading ? (
								<tr>
									<td colSpan={7} className="px-6 py-8">
										<TableSkeleton rows={4} cols={7} />
									</td>
								</tr>
							) : keys.length === 0 ? (
								<tr>
									<td
										colSpan={7}
										className="px-6 py-12 text-center text-slate-500"
									>
										<KeyRound className="w-8 h-8 text-slate-300 mx-auto mb-2" />
										<p className="font-semibold text-slate-700">
											Belum Ada API Key
										</p>
										<p className="text-xs text-slate-400 mt-1">
											Organisasi &apos;{currentOrg.name}&apos; belum memiliki
											API Key aktif.
										</p>
										<button
											type="button"
											onClick={() => setIsCreateModalOpen(true)}
											className="mt-4 inline-flex items-center gap-1.5 px-3.5 py-1.5 text-xs font-semibold text-white bg-[#1877F2] hover:bg-[#166FE5] rounded-lg shadow-xs transition-colors cursor-pointer"
										>
											<Plus className="w-3.5 h-3.5" />
											<span>Buat API Key Pertama</span>
										</button>
									</td>
								</tr>
							) : (
								keys.map((k) => {
									const isRevoked = Boolean(k.revoked_at);
									const isExpired = k.expires_at
										? new Date(k.expires_at) < new Date()
										: false;

									return (
										<tr
											key={k.id}
											className="hover:bg-slate-50/80 transition-colors"
										>
											<td className="px-6 py-4">
												<span className="font-semibold text-slate-900 block">
													{k.name}
												</span>
												<span className="text-[10px] text-slate-400 font-mono">
													ID: {k.id.slice(0, 8)}...
												</span>
											</td>
											<td className="px-6 py-4 font-mono font-medium text-slate-700">
												{k.masked_key || `${k.prefix}••••••••`}
											</td>
											<td className="px-6 py-4">
												<Badge
													variant={
														k.environment === "live" ? "success" : "neutral"
													}
												>
													{k.environment}
												</Badge>
											</td>
											<td className="px-6 py-4">
												<div className="flex flex-wrap gap-1">
													{k.scopes.map((s) => (
														<span
															key={s}
															className="px-1.5 py-0.5 rounded text-[10px] bg-slate-100 text-slate-600 font-mono"
														>
															{s}
														</span>
													))}
												</div>
											</td>
											<td className="px-6 py-4">
												{isRevoked ? (
													<Badge variant="danger">Revoked</Badge>
												) : isExpired ? (
													<Badge variant="warning">Expired</Badge>
												) : (
													<Badge variant="success">Active</Badge>
												)}
											</td>
											<td className="px-6 py-4 text-slate-500 tabular-nums">
												{k.last_used_at
													? new Date(k.last_used_at).toLocaleDateString(
															undefined,
															{
																month: "short",
																day: "numeric",
																hour: "2-digit",
																minute: "2-digit",
															},
														)
													: "Never"}
											</td>
											<td className="px-6 py-4 text-right">
												{!isRevoked && (
													<button
														type="button"
														onClick={() => setKeyToRevoke(k)}
														className="text-rose-600 hover:text-rose-800 p-1 rounded hover:bg-rose-50 transition-colors cursor-pointer"
														title="Revoke this API Key"
													>
														<Trash2 className="w-4 h-4" />
													</button>
												)}
											</td>
										</tr>
									);
								})
							)}
						</tbody>
					</table>
				</div>
			</div>

			{/* Create Key Modal */}
			<Modal
				isOpen={isCreateModalOpen}
				onClose={() => setIsCreateModalOpen(false)}
				title="Create New API Key"
				footer={
					<>
						<button
							type="button"
							onClick={() => setIsCreateModalOpen(false)}
							className="px-4 py-2 text-xs font-medium text-slate-700 bg-white border border-slate-200 rounded-lg hover:bg-slate-50 cursor-pointer"
						>
							Cancel
						</button>
						<button
							type="button"
							onClick={handleCreateKey}
							disabled={isCreating || !newKeyName.trim()}
							className="px-4 py-2 text-xs font-semibold text-white bg-[#1877F2] hover:bg-[#166FE5] disabled:opacity-50 rounded-lg transition-colors cursor-pointer"
						>
							{isCreating ? "Generating..." : "Create Key"}
						</button>
					</>
				}
			>
				<form onSubmit={handleCreateKey} className="space-y-4">
					<div>
						<label
							htmlFor="key-name"
							className="block text-xs font-medium text-slate-700 mb-1"
						>
							Key Description / Label
						</label>
						<input
							id="key-name"
							type="text"
							required
							placeholder="e.g. Production Backend Service"
							value={newKeyName}
							onChange={(e) => setNewKeyName(e.target.value)}
							className="w-full px-3 py-2 text-xs border border-slate-300 rounded-lg focus:outline-hidden focus:border-[#1877F2] focus:ring-2 focus:ring-[#1877F2]/20"
						/>
					</div>

					<div>
						<span className="block text-xs font-medium text-slate-700 mb-1">
							Environment
						</span>
						<div className="grid grid-cols-2 gap-3">
							<label
								className={`flex items-center justify-between p-3 rounded-lg border cursor-pointer text-xs ${newKeyEnv === "live" ? "border-[#1877F2] bg-[#E7F3FF]/40 text-[#1877F2] font-semibold" : "border-slate-200 text-slate-700"}`}
							>
								<span>Live (Production)</span>
								<input
									type="radio"
									name="env"
									value="live"
									checked={newKeyEnv === "live"}
									onChange={() => setNewKeyEnv("live")}
									className="accent-[#1877F2]"
								/>
							</label>

							<label
								className={`flex items-center justify-between p-3 rounded-lg border cursor-pointer text-xs ${newKeyEnv === "test" ? "border-[#1877F2] bg-[#E7F3FF]/40 text-[#1877F2] font-semibold" : "border-slate-200 text-slate-700"}`}
							>
								<span>Test (Sandbox)</span>
								<input
									type="radio"
									name="env"
									value="test"
									checked={newKeyEnv === "test"}
									onChange={() => setNewKeyEnv("test")}
									className="accent-[#1877F2]"
								/>
							</label>
						</div>
					</div>

					<div>
						<span className="block text-xs font-medium text-slate-700 mb-2">
							Assigned Scopes
						</span>
						<div className="space-y-2">
							{[
								{
									id: "ocr:read",
									label: "ocr:read",
									desc: "Query previous OCR results and metadata",
								},
								{
									id: "ocr:write",
									label: "ocr:write",
									desc: "Submit KTP documents for OCR extraction",
								},
								{
									id: "usage:read",
									label: "usage:read",
									desc: "Read analytics, metrics, and logs",
								},
							].map((scope) => (
								<label
									key={scope.id}
									className="flex items-start gap-2.5 p-2 rounded-md hover:bg-slate-50 cursor-pointer text-xs"
								>
									<input
										type="checkbox"
										checked={newKeyScopes.includes(scope.id)}
										onChange={(e) => {
											if (e.target.checked) {
												setNewKeyScopes([...newKeyScopes, scope.id]);
											} else {
												setNewKeyScopes(
													newKeyScopes.filter((s) => s !== scope.id),
												);
											}
										}}
										className="mt-0.5 accent-[#1877F2] rounded"
									/>
									<div>
										<span className="font-mono font-semibold text-slate-900">
											{scope.label}
										</span>
										<p className="text-[11px] text-slate-500">{scope.desc}</p>
									</div>
								</label>
							))}
						</div>
					</div>
				</form>
			</Modal>

			{/* One-Time Plaintext Token Reveal Modal */}
			{createdKeyData && (
				<Modal
					isOpen={true}
					onClose={() => setCreatedKeyData(null)}
					title="Save Your API Key"
					footer={
						<div className="flex flex-col-reverse sm:flex-row sm:items-center sm:justify-between gap-2 w-full">
							<button
								type="button"
								onClick={handleUseCreatedKey}
								className="px-3.5 py-2 text-xs font-semibold text-[#1877F2] bg-[#E7F3FF] hover:bg-[#d5eaff] rounded-lg transition-colors cursor-pointer text-center"
							>
								Connect in Dashboard
							</button>
							<button
								type="button"
								onClick={() => setCreatedKeyData(null)}
								className="px-4 py-2 text-xs font-semibold text-white bg-slate-900 hover:bg-slate-800 rounded-lg transition-colors cursor-pointer text-center"
							>
								I Have Saved It
							</button>
						</div>
					}
				>
					<div className="space-y-4">
						<div className="p-3 bg-amber-50 border border-amber-200 rounded-lg flex items-start gap-2 text-xs text-amber-800">
							<ShieldAlert className="w-4 h-4 text-amber-600 shrink-0 mt-0.5" />
							<div>
								<strong>Important:</strong> Please copy this API key now. For
								your security, this plaintext token will{" "}
								<strong>never be shown again</strong>.
							</div>
						</div>

						<div>
							<label
								htmlFor="created-key-token"
								className="block text-xs font-medium text-slate-700 mb-1"
							>
								API Key Secret Token
							</label>
							<div className="flex items-center gap-2">
								<input
									id="created-key-token"
									type="text"
									readOnly
									value={createdKeyData.key}
									className="w-full px-3 py-2 text-xs font-mono bg-slate-50 border border-slate-300 rounded-lg select-all"
								/>
								<button
									type="button"
									onClick={handleCopyKey}
									className="px-3 py-2 text-xs font-semibold text-white bg-[#1877F2] hover:bg-[#166FE5] rounded-lg transition-colors flex items-center gap-1 shrink-0 cursor-pointer"
								>
									{copied ? (
										<Check className="w-3.5 h-3.5" />
									) : (
										<Copy className="w-3.5 h-3.5" />
									)}
									<span>{copied ? "Copied" : "Copy"}</span>
								</button>
							</div>
						</div>

						<div className="text-xs text-slate-500 space-y-1 bg-slate-50 p-3 rounded-lg border border-slate-100 font-mono text-[11px]">
							<div>Name: {createdKeyData.name}</div>
							<div>Environment: {createdKeyData.environment}</div>
							<div>Scopes: {createdKeyData.scopes.join(", ")}</div>
						</div>
					</div>
				</Modal>
			)}

			{/* Revoke Key Confirmation Modal */}
			{keyToRevoke && (
				<Modal
					isOpen={true}
					onClose={() => setKeyToRevoke(null)}
					title="Revoke API Key"
					footer={
						<>
							<button
								type="button"
								onClick={() => setKeyToRevoke(null)}
								className="px-4 py-2 text-xs font-medium text-slate-700 bg-white border border-slate-200 rounded-lg hover:bg-slate-50 cursor-pointer"
							>
								Cancel
							</button>
							<button
								type="button"
								onClick={handleRevokeKey}
								disabled={isRevoking}
								className="px-4 py-2 text-xs font-semibold text-white bg-rose-600 hover:bg-rose-700 rounded-lg transition-colors cursor-pointer"
							>
								{isRevoking ? "Revoking..." : "Confirm Revocation"}
							</button>
						</>
					}
				>
					<div className="space-y-3">
						<div className="flex items-center gap-2 text-rose-700">
							<AlertOctagon className="w-5 h-5 text-rose-600" />
							<h4 className="font-semibold text-sm">
								Are you sure you want to revoke this key?
							</h4>
						</div>
						<p className="text-xs text-slate-600 leading-relaxed">
							Revoking key{" "}
							<strong className="text-slate-900">{keyToRevoke.name}</strong> (
							<code className="font-mono text-[11px]">
								{keyToRevoke.prefix}••••
							</code>
							) will immediately reject all future requests using it with HTTP
							401 Unauthorized. This action cannot be undone.
						</p>
					</div>
				</Modal>
			)}
		</div>
	);
};
