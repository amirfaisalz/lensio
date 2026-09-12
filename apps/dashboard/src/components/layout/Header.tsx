import { Check, KeyRound, LogOut, UserCheck, Users } from "lucide-react";
import type React from "react";
import { useState } from "react";
import { useAuth } from "../../context/AuthContext";
import { Modal } from "../common/Modal";

interface HeaderProps {
	title: string;
	subtitle?: string;
	onQuickTestClick?: () => void;
}

export const Header: React.FC<HeaderProps> = ({
	title,
	subtitle,
	onQuickTestClick,
}) => {
	const {
		apiKey,
		authMode,
		oidcUser,
		isConnected,
		setApiKey,
		loginOIDC,
		logout,
	} = useAuth();
	const [isAuthModalOpen, setIsAuthModalOpen] = useState(false);
	const [modalTab, setModalTab] = useState<"apikey" | "oidc">("apikey");
	const [inputKey, setInputKey] = useState("");
	const [inputOidcToken, setInputOidcToken] = useState("");

	const handleSaveKey = (e: React.FormEvent) => {
		e.preventDefault();
		if (inputKey.trim()) {
			setApiKey(inputKey.trim());
			setInputKey("");
			setIsAuthModalOpen(false);
		}
	};

	const handleSaveOidcToken = (e: React.FormEvent) => {
		e.preventDefault();
		if (inputOidcToken.trim()) {
			loginOIDC({
				sub: "manual-oidc-user",
				email: "operator@lensio.dev",
				preferredUsername: "operator",
				roles: ["admin"],
				token: inputOidcToken.trim(),
			});
			setInputOidcToken("");
			setIsAuthModalOpen(false);
		}
	};

	const handleSeedOIDCLogin = (
		email: string,
		preferredUsername: string,
		role: string,
	) => {
		loginOIDC({
			sub: `sub-${preferredUsername}`,
			email,
			preferredUsername,
			roles: [role],
			token: `mock_jwt_${preferredUsername}_${Date.now()}`,
		});
		setIsAuthModalOpen(false);
	};

	return (
		<header className="h-16 bg-white border-b border-slate-200 px-8 flex items-center justify-between sticky top-0 z-30">
			<div>
				<h1 className="text-lg font-bold text-slate-900 tracking-tight">
					{title}
				</h1>
				{subtitle && <p className="text-xs text-slate-500">{subtitle}</p>}
			</div>

			<div className="flex items-center gap-3">
				{onQuickTestClick && (
					<button
						type="button"
						onClick={onQuickTestClick}
						className="hidden sm:inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded-lg bg-[#E7F3FF] text-[#1877F2] hover:bg-[#dbeeff] transition-colors"
					>
						<span>Live KTP Test</span>
					</button>
				)}

				{authMode === "oidc" && oidcUser ? (
					<button
						type="button"
						onClick={() => {
							setModalTab("oidc");
							setIsAuthModalOpen(true);
						}}
						className="inline-flex items-center gap-2 px-3 py-1.5 text-xs font-semibold rounded-lg border border-purple-200 bg-purple-50 text-purple-700 hover:bg-purple-100 transition-colors"
					>
						<UserCheck className="w-3.5 h-3.5 text-purple-600" />
						<span>{oidcUser.email} (OIDC)</span>
					</button>
				) : (
					<button
						type="button"
						onClick={() => {
							setModalTab("apikey");
							setIsAuthModalOpen(true);
						}}
						className="inline-flex items-center gap-2 px-3 py-1.5 text-xs font-semibold rounded-lg border border-slate-200 bg-white text-slate-700 hover:bg-slate-50 transition-colors"
					>
						<KeyRound className="w-3.5 h-3.5 text-[#1877F2]" />
						<span>{apiKey ? "Change API Key" : "Connect Key"}</span>
					</button>
				)}

				{isConnected && (
					<button
						type="button"
						onClick={logout}
						title="Disconnect session"
						className="p-1.5 text-slate-400 hover:text-rose-600 rounded-lg transition-colors hover:bg-slate-100"
					>
						<LogOut className="w-4 h-4" />
					</button>
				)}
			</div>

			{/* Authentication Modal (Dual Auth: Machine API Key vs Keycloak OIDC) */}
			<Modal
				isOpen={isAuthModalOpen}
				onClose={() => setIsAuthModalOpen(false)}
				title="Lensio Authentication & Identity"
			>
				<div className="space-y-4">
					{/* Modal Tab Selector */}
					<div className="flex border-b border-slate-200 gap-4 text-xs font-medium">
						<button
							type="button"
							onClick={() => setModalTab("apikey")}
							className={`pb-2 border-b-2 transition-colors ${
								modalTab === "apikey"
									? "border-[#1877F2] text-[#1877F2] font-semibold"
									: "border-transparent text-slate-500 hover:text-slate-700"
							}`}
						>
							API Key (Machine Identity)
						</button>
						<button
							type="button"
							onClick={() => setModalTab("oidc")}
							className={`pb-2 border-b-2 transition-colors ${
								modalTab === "oidc"
									? "border-purple-600 text-purple-600 font-semibold"
									: "border-transparent text-slate-500 hover:text-slate-700"
							}`}
						>
							Keycloak OIDC (Human Operator)
						</button>
					</div>

					{modalTab === "apikey" ? (
						<form onSubmit={handleSaveKey} className="space-y-4">
							<p className="text-xs text-slate-600">
								Paste your active Lensio API key (
								<code className="font-mono text-[11px] bg-slate-100 px-1 py-0.5 rounded">
									lensio_live_...
								</code>{" "}
								or{" "}
								<code className="font-mono text-[11px] bg-slate-100 px-1 py-0.5 rounded">
									lensio_test_...
								</code>
								). It will be used for machine-level authenticated API calls.
							</p>
							<div>
								<label
									htmlFor="api-key-input"
									className="block text-xs font-medium text-slate-700 mb-1"
								>
									API Key Token
								</label>
								<input
									id="api-key-input"
									type="password"
									placeholder="lensio_live_..."
									value={inputKey}
									onChange={(e) => setInputKey(e.target.value)}
									className="w-full px-3 py-2 text-sm font-mono border border-slate-300 rounded-lg focus:outline-hidden focus:border-[#1877F2] focus:ring-2 focus:ring-[#1877F2]/20"
								/>
							</div>
							{apiKey && (
								<div className="p-3 bg-slate-50 rounded-lg border border-slate-200 text-xs text-slate-600 flex items-center justify-between">
									<div>
										<p className="font-medium text-slate-800">
											Currently Connected Key
										</p>
										<p className="font-mono text-[11px] text-slate-500">
											{apiKey.slice(0, 16)}••••••••
										</p>
									</div>
									<span className="inline-flex items-center gap-1 text-emerald-600 font-semibold text-[11px]">
										<Check className="w-3.5 h-3.5" /> Active
									</span>
								</div>
							)}
							<div className="flex justify-end gap-2 pt-2">
								<button
									type="button"
									onClick={() => setIsAuthModalOpen(false)}
									className="px-4 py-2 text-xs font-medium text-slate-700 bg-white border border-slate-200 rounded-lg hover:bg-slate-50"
								>
									Cancel
								</button>
								<button
									type="submit"
									className="px-4 py-2 text-xs font-semibold text-white bg-[#1877F2] hover:bg-[#166FE5] rounded-lg transition-colors"
								>
									Save & Authenticate
								</button>
							</div>
						</form>
					) : (
						<div className="space-y-4">
							<p className="text-xs text-slate-600">
								Authenticate as a human operator via Keycloak OpenID Connect
								(OIDC). Short-lived JWT bearer tokens grant role-scoped access
								to the dashboard.
							</p>

							{/* Quick seed user logins */}
							<div className="p-3 bg-slate-50 rounded-lg border border-slate-200 space-y-2">
								<p className="text-xs font-semibold text-slate-700 flex items-center gap-1.5">
									<Users className="w-3.5 h-3.5 text-purple-600" />
									Keycloak Seed Users:
								</p>
								<div className="flex flex-col sm:flex-row gap-2">
									<button
										type="button"
										onClick={() =>
											handleSeedOIDCLogin("admin@lensio.dev", "admin", "admin")
										}
										className="flex-1 px-3 py-2 text-xs font-semibold text-purple-700 bg-purple-100 hover:bg-purple-200 rounded-lg text-left transition-colors"
									>
										<div>admin@lensio.dev</div>
										<div className="text-[10px] text-purple-600 font-normal">
											Role: admin
										</div>
									</button>
									<button
										type="button"
										onClick={() =>
											handleSeedOIDCLogin(
												"developer@veriform.com",
												"dev",
												"developer",
											)
										}
										className="flex-1 px-3 py-2 text-xs font-semibold text-indigo-700 bg-indigo-100 hover:bg-indigo-200 rounded-lg text-left transition-colors"
									>
										<div>developer@veriform.com</div>
										<div className="text-[10px] text-indigo-600 font-normal">
											Role: developer
										</div>
									</button>
								</div>
							</div>

							<form onSubmit={handleSaveOidcToken} className="space-y-3">
								<label
									htmlFor="oidc-token-input"
									className="block text-xs font-medium text-slate-700"
								>
									Or paste Keycloak JWT bearer token:
								</label>
								<input
									id="oidc-token-input"
									type="password"
									placeholder="eyJhbGciOiJSUzI1NiIs..."
									value={inputOidcToken}
									onChange={(e) => setInputOidcToken(e.target.value)}
									className="w-full px-3 py-2 text-xs font-mono border border-slate-300 rounded-lg focus:outline-hidden focus:border-purple-600 focus:ring-2 focus:ring-purple-600/20"
								/>
								<div className="flex justify-end gap-2 pt-2">
									<button
										type="button"
										onClick={() => setIsAuthModalOpen(false)}
										className="px-4 py-2 text-xs font-medium text-slate-700 bg-white border border-slate-200 rounded-lg hover:bg-slate-50"
									>
										Cancel
									</button>
									<button
										type="submit"
										className="px-4 py-2 text-xs font-semibold text-white bg-purple-600 hover:bg-purple-700 rounded-lg transition-colors"
									>
										Authenticate OIDC
									</button>
								</div>
							</form>
						</div>
					)}
				</div>
			</Modal>
		</header>
	);
};
