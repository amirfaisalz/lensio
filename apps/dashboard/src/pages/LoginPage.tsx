import {
	AlertCircle,
	ArrowRight,
	BadgeCheck,
	CheckCircle2,
	Eye,
	EyeOff,
	KeyRound,
	Loader2,
	Lock,
	Mail,
	MailCheck,
	ShieldCheck,
	User,
	Zap,
} from "lucide-react";
import type React from "react";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import type { ApiClientError } from "../services/api";

export interface LoginPageProps {
	defaultMode?: "login" | "register";
}

export const LoginPage: React.FC<LoginPageProps> = ({
	defaultMode = "login",
}) => {
	const navigate = useNavigate();
	const { loginWithPassword, registerUser, verifyUserEmail } = useAuth();

	const [activeTab, setActiveTab] = useState<"login" | "register">(defaultMode);

	// Login states
	const [loginEmail, setLoginEmail] = useState("");
	const [loginPassword, setLoginPassword] = useState("");
	const [showLoginPassword, setShowLoginPassword] = useState(false);
	const [rememberMe, setRememberMe] = useState(true);

	// Register states (User Identity Only)
	const [regName, setRegName] = useState("");
	const [regEmail, setRegEmail] = useState("");
	const [regPassword, setRegPassword] = useState("");
	const [regConfirmPassword, setRegConfirmPassword] = useState("");
	const [showRegPassword, setShowRegPassword] = useState(false);
	const [agreeTerms, setAgreeTerms] = useState(false);

	// Verification state
	const [verificationPendingEmail, setVerificationPendingEmail] = useState<
		string | null
	>(null);
	const [verificationSuccessMsg, setVerificationSuccessMsg] = useState<
		string | null
	>(null);

	// Common UI states
	const [isLoading, setIsLoading] = useState(false);
	const [errorMessage, setErrorMessage] = useState<string | null>(null);
	const [unverifiedAttemptEmail, setUnverifiedAttemptEmail] = useState<
		string | null
	>(null);

	// Calculate password strength
	const getPasswordStrength = (
		pass: string,
	): { score: number; label: string; color: string } => {
		if (!pass) return { score: 0, label: "Kosong", color: "bg-slate-200" };
		let score = 0;
		if (pass.length >= 8) score++;
		if (/[A-Z]/.test(pass) && /[a-z]/.test(pass)) score++;
		if (/[0-9]/.test(pass)) score++;
		if (/[^A-Za-z0-9]/.test(pass)) score++;

		if (score <= 1) return { score: 1, label: "Lemah", color: "bg-rose-500" };
		if (score <= 3) return { score: 2, label: "Sedang", color: "bg-amber-500" };
		return { score: 3, label: "Kuat", color: "bg-emerald-500" };
	};

	const strength = getPasswordStrength(regPassword);

	// Handle standard normal login (PostgreSQL API or Keycloak)
	const handlePasswordLogin = async (e: React.FormEvent) => {
		e.preventDefault();
		setErrorMessage(null);
		setUnverifiedAttemptEmail(null);
		setVerificationSuccessMsg(null);

		const email = loginEmail.trim();
		const password = loginPassword.trim();

		if (!email) {
			setErrorMessage("Silakan masukkan email kerja atau username Anda.");
			return;
		}
		if (!password) {
			setErrorMessage("Silakan masukkan kata sandi Anda.");
			return;
		}

		setIsLoading(true);

		try {
			// Authenticate against PostgreSQL backend API
			await loginWithPassword(email, password);
			navigate("/dashboard");
		} catch (err: unknown) {
			setIsLoading(false);
			const apiErr = err as ApiClientError;
			if (apiErr.code === "EMAIL_NOT_VERIFIED" || apiErr.status === 403) {
				setUnverifiedAttemptEmail(email);
				setErrorMessage(
					"Akun Anda belum diverifikasi. Silakan periksa email Anda dan lakukan verifikasi sebelum masuk.",
				);
				return;
			}
			if (apiErr.status === 401 || apiErr.code === "invalid_api_key") {
				setErrorMessage(
					"Email atau kata sandi tidak valid. Silakan periksa kembali kredensial Anda.",
				);
				return;
			}
			setErrorMessage(
				apiErr.message ||
					"Email atau kata sandi tidak valid. Silakan periksa kembali kredensial Anda.",
			);
		}
	};

	// Handle User Registration (User Identity Only - No Workspace Creation)
	const handleRegister = async (e: React.FormEvent) => {
		e.preventDefault();
		setErrorMessage(null);

		const name = regName.trim();
		const email = regEmail.trim();

		if (!name) {
			setErrorMessage("Silakan masukkan nama lengkap Anda.");
			return;
		}

		if (!email || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
			setErrorMessage("Silakan masukkan alamat email kerja yang valid.");
			return;
		}

		if (regPassword.length < 8) {
			setErrorMessage("Kata sandi harus memiliki panjang minimal 8 karakter.");
			return;
		}

		if (regPassword !== regConfirmPassword) {
			setErrorMessage("Konfirmasi kata sandi tidak cocok dengan kata sandi.");
			return;
		}

		if (!agreeTerms) {
			setErrorMessage(
				"Anda harus menyetujui Ketentuan Layanan & Kebijakan Privasi.",
			);
			return;
		}

		setIsLoading(true);

		try {
			// Register user identity into the system with verification requirement
			await registerUser(name, email, regPassword);
			setIsLoading(false);
			// Transition to Email Verification Notice screen
			setVerificationPendingEmail(email);
		} catch (err) {
			setIsLoading(false);
			setErrorMessage(
				err instanceof Error
					? err.message
					: "Gagal mendaftarkan akun. Silakan coba lagi.",
			);
		}
	};

	// Helper to complete verification (calls backend PostgreSQL API)
	const handleSimulateVerification = async (emailToVerify: string) => {
		try {
			setIsLoading(true);
			await verifyUserEmail(emailToVerify);
			setIsLoading(false);
			setVerificationPendingEmail(null);
			setUnverifiedAttemptEmail(null);
			setLoginEmail(emailToVerify);
			setActiveTab("login");
			setVerificationSuccessMsg(
				`Email ${emailToVerify} berhasil diverifikasi! Silakan masuk dengan kata sandi Anda.`,
			);
			setErrorMessage(null);
		} catch (err: unknown) {
			setIsLoading(false);
			setErrorMessage(
				err instanceof Error
					? err.message
					: "Gagal memverifikasi email. Silakan periksa kembali tautan verifikasi.",
			);
		}
	};

	return (
		<div className="min-h-screen bg-[#F0F2F5] flex flex-col lg:flex-row text-slate-900 font-sans antialiased">
			{/* Left Column: Brand Hero & Platform Highlights (Desktop) */}
			<div className="hidden lg:flex lg:w-1/2 bg-slate-950 text-white px-12 py-10 flex-col justify-between relative overflow-hidden">
				{/* Static gradient wash: one paint, zero blur filters */}
				<div
					aria-hidden="true"
					className="absolute inset-0 pointer-events-none"
					style={{
						backgroundImage:
							"radial-gradient(60% 45% at 18% 0%, rgba(24,119,242,0.16), transparent 70%), radial-gradient(50% 40% at 92% 100%, rgba(16,185,129,0.10), transparent 70%)",
					}}
				/>

				{/* Top Brand */}
				<div className="relative z-10">
					<div className="flex items-center gap-3 mb-6">
						<div className="w-10 h-10 rounded-xl bg-[#1877F2] flex items-center justify-center text-white shadow-lg shadow-[#1877F2]/30">
							<Zap className="w-5 h-5" />
						</div>
						<div>
							<span className="text-xl font-bold tracking-tight text-white">
								Lensio
							</span>
							<span className="ml-2 text-[11px] px-2 py-0.5 rounded-full bg-white/10 text-slate-300 font-medium tracking-wide uppercase">
								Identity OCR API
							</span>
						</div>
					</div>
					<h1 className="text-4xl font-extrabold tracking-tight text-white leading-[1.1]">
						Infrastruktur OCR{" "}
						<span className="text-[#7aa9f5]">Identitas Indonesia</span>{" "}
						Berperforma Tinggi
					</h1>
					<p className="mt-3 text-sm text-slate-400 max-w-md leading-relaxed">
						Layanan API ekstraksi dokumen identitas (KTP & SIM) otomatis
						berbasis Vision AI untuk industri perbankan, fintech, dan verifikasi
						identitas di Indonesia.
					</p>
				</div>

				{/* Center: live scan visual — pure CSS, one transform-only animation */}
				<div className="relative z-10 my-10" aria-hidden="true">
					<div className="relative overflow-hidden rounded-2xl border border-white/15 bg-white/[0.04]">
						<span className="absolute left-3 top-3 h-5 w-5 rounded-tl-md border-t-2 border-l-2 border-[#60a5fa]" />
						<span className="absolute right-3 top-3 h-5 w-5 rounded-tr-md border-t-2 border-r-2 border-[#60a5fa]" />
						<span className="absolute left-3 bottom-3 h-5 w-5 rounded-bl-md border-b-2 border-l-2 border-[#60a5fa]" />
						<span className="absolute right-3 bottom-3 h-5 w-5 rounded-br-md border-b-2 border-r-2 border-[#60a5fa]" />
						<div className="absolute inset-x-6 top-0 bottom-0 overflow-hidden">
							<div className="scan-beam h-1/4 w-full bg-linear-to-b from-transparent via-[#60a5fa]/35 to-transparent opacity-0" />
						</div>
						<div className="p-7">
							<div className="flex items-center justify-between">
								<span className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-400">
									KTP • Ekstraksi
								</span>
								<span className="flex items-center gap-1.5 text-[11px] font-medium text-emerald-400">
									<span className="h-1.5 w-1.5 rounded-full bg-emerald-400" />
									Live
								</span>
							</div>
							<p className="mt-4 font-mono text-2xl font-semibold tracking-[0.08em] text-white tabular-nums">
								3171 •••• •••• 0001
							</p>
							<dl className="mt-5 space-y-3">
								<div className="flex items-center justify-between gap-4">
									<dt className="text-[11px] uppercase tracking-[0.14em] text-slate-500">
										Nama
									</dt>
									<dd className="flex items-center gap-2 text-sm text-slate-200">
										BUDI S••••••
										<span className="rounded-md bg-emerald-500/15 px-1.5 py-0.5 font-mono text-[11px] font-semibold text-emerald-300 tabular-nums">
											0.98
										</span>
									</dd>
								</div>
								<div className="flex items-center justify-between gap-4">
									<dt className="text-[11px] uppercase tracking-[0.14em] text-slate-500">
										Lahir
									</dt>
									<dd className="flex items-center gap-2 text-sm text-slate-200">
										17-08-1992
										<span className="rounded-md bg-emerald-500/15 px-1.5 py-0.5 font-mono text-[11px] font-semibold text-emerald-300 tabular-nums">
											0.97
										</span>
									</dd>
								</div>
								<div className="flex items-center justify-between gap-4">
									<dt className="text-[11px] uppercase tracking-[0.14em] text-slate-500">
										Status
									</dt>
									<dd className="flex items-center gap-2 text-sm text-slate-200">
										KAWIN
										<span className="rounded-md bg-emerald-500/15 px-1.5 py-0.5 font-mono text-[11px] font-semibold text-emerald-300 tabular-nums">
											0.99
										</span>
									</dd>
								</div>
							</dl>
						</div>
					</div>
					<ul className="mt-6 space-y-2.5">
						<li className="flex items-center gap-2.5 text-xs text-slate-300">
							<EyeOff className="h-4 w-4 shrink-0 text-emerald-400" />
							Zero PII retained — citra in-memory, langsung dihapus
						</li>
						<li className="flex items-center gap-2.5 text-xs text-slate-300">
							<BadgeCheck className="h-4 w-4 shrink-0 text-[#60a5fa]" />
							Validasi deterministik 16-digit NIK, tanpa halusinasi LLM
						</li>
						<li className="flex items-center gap-2.5 text-xs text-slate-300">
							<KeyRound className="h-4 w-4 shrink-0 text-purple-400" />
							Keycloak OIDC + ReBAC SpiceDB standar enterprise
						</li>
					</ul>
				</div>

				{/* Bottom Trust & Status */}
				<div className="relative z-10 pt-6 border-t border-white/10 flex items-center justify-between text-xs text-slate-400 tabular-nums">
					<div className="flex items-center gap-2">
						<span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse" />
						<span className="text-slate-300 font-medium">
							Sistem Berjalan Normal (99.99%)
						</span>
					</div>
					<span>P95 Latency &lt; 800ms</span>
				</div>
			</div>

			{/* Right Column: Clean Interactive Auth Experience */}
			<div className="flex-1 flex flex-col justify-center items-center p-4 sm:p-8 lg:p-12 overflow-y-auto">
				{/* Mobile Brand Header */}
				<div className="w-full max-w-md text-center lg:hidden mb-6">
					<div className="inline-flex items-center justify-center w-12 h-12 rounded-xl bg-[#1877F2] text-white shadow-md shadow-[#1877F2]/25 mb-3">
						<Zap className="w-6 h-6" />
					</div>
					<h1 className="text-2xl font-bold text-slate-900 tracking-tight">
						Lensio Developer Portal
					</h1>
					<p className="text-xs text-slate-500 mt-1">
						High-Performance Indonesian Identity OCR API as a Service
					</p>
				</div>

				{/* Verification Pending Screen View */}
				{verificationPendingEmail ? (
					<div className="w-full max-w-md bg-white rounded-2xl shadow-[0_1px_2px_rgba(15,23,42,0.06),0_16px_40px_-24px_rgba(15,23,42,0.25)] p-6 sm:p-8 text-center">
						<div className="w-14 h-14 rounded-2xl bg-[#E7F3FF] text-[#1877F2] flex items-center justify-center mx-auto mb-5">
							<MailCheck className="w-7 h-7" />
						</div>

						<h2 className="text-xl font-bold text-slate-900 tracking-tight">
							Verifikasi Email Anda
						</h2>
						<p className="text-xs text-slate-600 mt-2 leading-relaxed">
							Kami telah mengirimkan tautan konfirmasi ke alamat email:
						</p>
						<div className="my-3 px-3 py-2 bg-slate-50 border border-slate-200 rounded-lg text-xs font-mono font-semibold text-slate-800 break-all">
							{verificationPendingEmail}
						</div>

						<div className="p-3.5 bg-amber-50 border border-amber-200/80 rounded-xl text-left text-xs text-amber-800 mb-6 flex items-start gap-2.5">
							<AlertCircle className="w-4 h-4 shrink-0 mt-0.5 text-amber-600" />
							<div>
								<p className="font-semibold">Verifikasi Diperlukan</p>
								<p className="text-xs text-amber-700 mt-0.5">
									Sesuai kebijakan keamanan platform, Anda belum dapat masuk ke
									dashboard sebelum melakukan verifikasi email.
								</p>
							</div>
						</div>

						<div className="space-y-3">
							{/* Dev Simulation button */}
							<button
								type="button"
								onClick={() =>
									handleSimulateVerification(verificationPendingEmail)
								}
								className="w-full py-2.5 px-4 text-xs font-semibold text-white bg-[#1877F2] hover:bg-[#166FE5] active:bg-[#0e5ec8] rounded-xl transition-colors flex items-center justify-center gap-1.5 cursor-pointer"
							>
								<CheckCircle2 className="w-4 h-4" />
								<span>Verifikasi Sekarang (Simulasi Dev)</span>
							</button>

							<button
								type="button"
								onClick={() => {
									setVerificationPendingEmail(null);
									setActiveTab("login");
									setErrorMessage(null);
								}}
								className="w-full py-2.5 px-4 text-xs font-medium text-slate-600 hover:text-slate-900 bg-white border border-slate-200 hover:bg-slate-50 rounded-xl transition-colors cursor-pointer"
							>
								Kembali ke Halaman Masuk
							</button>
						</div>
					</div>
				) : (
					/* Main Auth Container */
					<div className="w-full max-w-md bg-white rounded-2xl shadow-[0_1px_2px_rgba(15,23,42,0.06),0_16px_40px_-24px_rgba(15,23,42,0.25)] overflow-hidden">
						{/* Tab Switcher: Masuk vs Daftar */}
						<div
							role="tablist"
							aria-label="Pilih mode autentikasi"
							className="flex border-b border-slate-200 bg-slate-50/50"
						>
							<button
								type="button"
								role="tab"
								aria-selected={activeTab === "login"}
								onClick={() => {
									setActiveTab("login");
									setErrorMessage(null);
									setUnverifiedAttemptEmail(null);
								}}
								className={`flex-1 py-3.5 text-xs font-semibold border-b-2 transition-all flex items-center justify-center gap-1.5 ${
									activeTab === "login"
										? "border-[#1877F2] text-[#1877F2] bg-white font-bold"
										: "border-transparent text-slate-500 hover:text-slate-700 hover:bg-slate-100/50"
								}`}
							>
								<Lock className="w-4 h-4" />
								Masuk (Sign In)
							</button>
							<button
								type="button"
								role="tab"
								aria-selected={activeTab === "register"}
								onClick={() => {
									setActiveTab("register");
									setErrorMessage(null);
									setUnverifiedAttemptEmail(null);
								}}
								className={`flex-1 py-3.5 text-xs font-semibold border-b-2 transition-all flex items-center justify-center gap-1.5 ${
									activeTab === "register"
										? "border-[#1877F2] text-[#1877F2] bg-white font-bold"
										: "border-transparent text-slate-500 hover:text-slate-700 hover:bg-slate-100/50"
								}`}
							>
								<User className="w-4 h-4" />
								Daftar Akun (Register)
							</button>
						</div>

						<div className="p-6 sm:p-7">
							{/* Error Alert */}
							{errorMessage && (
								<div
									role="alert"
									aria-live="assertive"
									className="mb-5 p-3.5 bg-rose-50 border border-rose-200 text-rose-700 rounded-xl text-xs flex items-start gap-2.5"
								>
									<AlertCircle className="w-4 h-4 shrink-0 mt-0.5 text-rose-600" />
									<div className="flex-1">
										<p className="font-semibold">Autentikasi Gagal</p>
										<p className="text-xs text-rose-600 mt-0.5">
											{errorMessage}
										</p>
										{unverifiedAttemptEmail && (
											<button
												type="button"
												onClick={() =>
													handleSimulateVerification(unverifiedAttemptEmail)
												}
												className="mt-2 text-xs font-semibold text-[#1877F2] hover:underline flex items-center gap-1 rounded px-0.5 py-0.5"
											>
												<CheckCircle2 className="w-3.5 h-3.5" />
												<span>Lakukan verifikasi email sekarang</span>
											</button>
										)}
									</div>
								</div>
							)}

							{/* Verification Success Toast */}
							{verificationSuccessMsg && (
								<div
									role="status"
									aria-live="polite"
									className="mb-5 p-3 bg-emerald-50 border border-emerald-200 text-emerald-800 rounded-xl text-xs flex items-center gap-2"
								>
									<CheckCircle2 className="w-4 h-4 shrink-0 text-emerald-600" />
									<span>{verificationSuccessMsg}</span>
								</div>
							)}

							{/* SIGN IN VIEW */}
							{activeTab === "login" ? (
								<form onSubmit={handlePasswordLogin} className="space-y-4">
									<div>
										<label
											htmlFor="login-email"
											className="block text-xs font-semibold text-slate-700 mb-1.5"
										>
											Email Kerja / Username
										</label>
										<div className="relative">
											<Mail className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
											<input
												id="login-email"
												type="text"
												inputMode="email"
												autoComplete="username"
												placeholder="nama@perusahaan.com"
												value={loginEmail}
												aria-invalid={Boolean(errorMessage)}
												onChange={(e) => {
													setLoginEmail(e.target.value);
													if (errorMessage) setErrorMessage(null);
												}}
												className="w-full pl-9 pr-3 py-2.5 text-sm text-slate-900 border border-slate-300 rounded-lg placeholder:text-slate-400 focus:outline-none focus:border-[#1877F2] focus:ring-2 focus:ring-[#1877F2]/25 transition-all"
											/>
										</div>
									</div>

									<div>
										<div className="flex items-center justify-between mb-1.5">
											<label
												htmlFor="login-password"
												className="block text-xs font-semibold text-slate-700"
											>
												Kata Sandi
											</label>
											<button
												type="button"
												className="text-xs font-medium text-[#1877F2] hover:underline rounded px-1 py-0.5"
											>
												Lupa sandi?
											</button>
										</div>
										<div className="relative">
											<Lock className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
											<input
												id="login-password"
												type={showLoginPassword ? "text" : "password"}
												autoComplete="current-password"
												placeholder="••••••••"
												value={loginPassword}
												aria-invalid={Boolean(errorMessage)}
												onChange={(e) => {
													setLoginPassword(e.target.value);
													if (errorMessage) setErrorMessage(null);
												}}
												className="w-full pl-9 pr-10 py-2.5 text-sm text-slate-900 border border-slate-300 rounded-lg placeholder:text-slate-400 focus:outline-none focus:border-[#1877F2] focus:ring-2 focus:ring-[#1877F2]/25 transition-all"
											/>
											<button
												type="button"
												onClick={() => setShowLoginPassword(!showLoginPassword)}
												aria-label={
													showLoginPassword
														? "Sembunyikan kata sandi"
														: "Tampilkan kata sandi"
												}
												className="absolute right-2 top-1/2 -translate-y-1/2 p-1 rounded-md text-slate-400 hover:text-slate-600 cursor-pointer"
											>
												{showLoginPassword ? (
													<EyeOff className="w-4 h-4" />
												) : (
													<Eye className="w-4 h-4" />
												)}
											</button>
										</div>
									</div>

									<div className="flex items-center justify-between text-xs text-slate-600">
										<label className="flex items-center gap-2 cursor-pointer select-none py-1">
											<input
												type="checkbox"
												checked={rememberMe}
												onChange={(e) => setRememberMe(e.target.checked)}
												className="w-4 h-4 rounded border-slate-300 bg-white text-[#1877F2] accent-[#1877F2] scheme-light focus:ring-[#1877F2]"
											/>
											<span className="text-xs">Ingat sesi saya</span>
										</label>
									</div>

									<button
										type="submit"
										disabled={isLoading}
										aria-busy={isLoading}
										className="w-full py-2.5 px-4 text-sm font-semibold text-white bg-[#1877F2] hover:bg-[#166FE5] active:bg-[#0e5ec8] disabled:opacity-50 disabled:cursor-not-allowed rounded-lg shadow-[0_1px_2px_rgba(24,119,242,0.4)] transition-colors flex items-center justify-center gap-2 cursor-pointer"
									>
										{isLoading ? (
											<>
												<Loader2 className="w-3.5 h-3.5 animate-spin" />
												<span>Memverifikasi Kredensial...</span>
											</>
										) : (
											<>
												<span>Masuk ke Dashboard</span>
												<ArrowRight className="w-3.5 h-3.5" />
											</>
										)}
									</button>
								</form>
							) : (
								/* REGISTER VIEW (USER IDENTITY ONLY - NO WORKSPACE FIELDS) */
								<form onSubmit={handleRegister} className="space-y-3.5">
									<div>
										<label
											htmlFor="reg-name"
											className="block text-xs font-semibold text-slate-700 mb-1"
										>
											Nama Lengkap
										</label>
										<div className="relative">
											<User className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
											<input
												id="reg-name"
												type="text"
												autoComplete="name"
												placeholder="misal: Budi Santoso"
												value={regName}
												onChange={(e) => {
													setRegName(e.target.value);
													if (errorMessage) setErrorMessage(null);
												}}
												className="w-full pl-9 pr-3 py-2.5 text-sm text-slate-900 border border-slate-300 rounded-lg placeholder:text-slate-400 focus:outline-none focus:border-[#1877F2] focus:ring-2 focus:ring-[#1877F2]/25"
											/>
										</div>
									</div>

									<div>
										<label
											htmlFor="reg-email"
											className="block text-xs font-semibold text-slate-700 mb-1"
										>
											Email Kerja
										</label>
										<div className="relative">
											<Mail className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
											<input
												id="reg-email"
												type="email"
												autoComplete="email"
												placeholder="budi@perusahaan.id"
												value={regEmail}
												onChange={(e) => {
													setRegEmail(e.target.value);
													if (errorMessage) setErrorMessage(null);
												}}
												className="w-full pl-9 pr-3 py-2.5 text-sm text-slate-900 border border-slate-300 rounded-lg placeholder:text-slate-400 focus:outline-none focus:border-[#1877F2] focus:ring-2 focus:ring-[#1877F2]/25"
											/>
										</div>
									</div>

									<div>
										<label
											htmlFor="reg-password"
											className="block text-xs font-semibold text-slate-700 mb-1"
										>
											Kata Sandi
										</label>
										<div className="relative">
											<Lock className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
											<input
												id="reg-password"
												type={showRegPassword ? "text" : "password"}
												autoComplete="new-password"
												placeholder="Minimal 8 karakter"
												value={regPassword}
												aria-describedby="reg-password-strength"
												onChange={(e) => {
													setRegPassword(e.target.value);
													if (errorMessage) setErrorMessage(null);
												}}
												className="w-full pl-9 pr-10 py-2.5 text-sm text-slate-900 border border-slate-300 rounded-lg placeholder:text-slate-400 focus:outline-none focus:border-[#1877F2] focus:ring-2 focus:ring-[#1877F2]/25"
											/>
											<button
												type="button"
												onClick={() => setShowRegPassword(!showRegPassword)}
												aria-label={
													showRegPassword
														? "Sembunyikan kata sandi"
														: "Tampilkan kata sandi"
												}
												className="absolute right-2 top-1/2 -translate-y-1/2 p-1 rounded-md text-slate-400 hover:text-slate-600 cursor-pointer"
											>
												{showRegPassword ? (
													<EyeOff className="w-4 h-4" />
												) : (
													<Eye className="w-4 h-4" />
												)}
											</button>
										</div>

										{/* Password strength bar */}
										{regPassword && (
											<div
												className="mt-1.5"
												id="reg-password-strength"
												role="status"
												aria-live="polite"
											>
												{" "}
												<div className="flex items-center justify-between text-[10px] text-slate-500 mb-0.5">
													<span>Kekuatan sandi:</span>
													<span className="font-semibold text-slate-700">
														{strength.label}
													</span>
												</div>
												<div className="w-full h-1 bg-slate-100 rounded-full overflow-hidden flex gap-1">
													<div
														className={`h-full flex-1 rounded-full ${
															strength.score >= 1
																? strength.color
																: "bg-slate-200"
														}`}
													/>
													<div
														className={`h-full flex-1 rounded-full ${
															strength.score >= 2
																? strength.color
																: "bg-slate-200"
														}`}
													/>
													<div
														className={`h-full flex-1 rounded-full ${
															strength.score >= 3
																? strength.color
																: "bg-slate-200"
														}`}
													/>
												</div>
											</div>
										)}
									</div>

									<div>
										<label
											htmlFor="reg-confirm"
											className="block text-xs font-semibold text-slate-700 mb-1"
										>
											Konfirmasi Kata Sandi
										</label>
										<div className="relative">
											<Lock className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
											<input
												id="reg-confirm"
												type={showRegPassword ? "text" : "password"}
												autoComplete="new-password"
												placeholder="Ulangi kata sandi"
												value={regConfirmPassword}
												onChange={(e) => {
													setRegConfirmPassword(e.target.value);
													if (errorMessage) setErrorMessage(null);
												}}
												className="w-full pl-9 pr-3 py-2.5 text-sm text-slate-900 border border-slate-300 rounded-lg placeholder:text-slate-400 focus:outline-none focus:border-[#1877F2] focus:ring-2 focus:ring-[#1877F2]/25"
											/>
										</div>
									</div>

									<div className="pt-1">
										<label className="flex items-start gap-2 text-xs text-slate-600 cursor-pointer select-none">
											<input
												type="checkbox"
												checked={agreeTerms}
												onChange={(e) => setAgreeTerms(e.target.checked)}
												className="w-4 h-4 rounded border-slate-300 bg-white text-[#1877F2] accent-[#1877F2] scheme-light focus:ring-[#1877F2] mt-0.5"
											/>
											<span>
												Saya menyetujui{" "}
												<button
													type="button"
													className="text-[#1877F2] underline rounded px-0.5"
												>
													Ketentuan Layanan
												</button>{" "}
												dan{" "}
												<button
													type="button"
													className="text-[#1877F2] underline rounded px-0.5"
												>
													Kebijakan Privasi
												</button>{" "}
												Lensio.
											</span>
										</label>
									</div>

									<button
										type="submit"
										disabled={isLoading}
										aria-busy={isLoading}
										className="w-full py-2.5 px-4 text-sm font-semibold text-white bg-[#1877F2] hover:bg-[#166FE5] active:bg-[#0e5ec8] disabled:opacity-50 disabled:cursor-not-allowed rounded-lg shadow-[0_1px_2px_rgba(24,119,242,0.4)] transition-colors flex items-center justify-center gap-2 cursor-pointer mt-2"
									>
										{isLoading ? (
											<>
												<Loader2 className="w-3.5 h-3.5 animate-spin" />
												<span>Mendaftarkan Akun...</span>
											</>
										) : (
											<>
												<span>Daftar Sekarang</span>
												<ArrowRight className="w-3.5 h-3.5" />
											</>
										)}
									</button>
								</form>
							)}
						</div>

						{/* Footer Security Badges */}
						<div className="px-6 py-3.5 bg-slate-50 border-t border-slate-200 flex items-center justify-between text-xs text-slate-500">
							<span className="flex items-center gap-1">
								<ShieldCheck className="w-3.5 h-3.5 text-emerald-600" />
								<span>Keycloak OIDC Standar</span>
							</span>
							<span>256-bit TLS Encrypted</span>
						</div>
					</div>
				)}

				{/* Portal Footnote */}
				<div className="mt-6 text-center text-xs text-slate-500">
					Lensio Platform • Indonesian Identity Document OCR API
				</div>
			</div>
		</div>
	);
};
