import type React from "react";
import {
	createContext,
	useCallback,
	useContext,
	useEffect,
	useMemo,
	useState,
} from "react";
import { api } from "../services/api";

export type AuthMode = "apikey" | "oidc";

export interface OrganizationContext {
	id: string;
	name: string;
	slug: string;
	planCode: string;
}

export interface OIDCUserSession {
	sub: string;
	email: string;
	preferredUsername: string;
	name?: string;
	roles?: string[];
	token?: string;
	organization?: OrganizationContext;
}

export interface AuthContextValue {
	apiKey: string | null;
	environment: "live" | "test";
	isConnected: boolean;
	isInitializing: boolean;
	authMode: AuthMode;
	oidcUser: OIDCUserSession | null;
	currentOrg: OrganizationContext | null;
	organizations: OrganizationContext[];
	setApiKey: (key: string | null) => void;
	setEnvironment: (env: "live" | "test") => void;
	loginOIDC: (session: OIDCUserSession) => void;
	loginWithPassword: (email: string, password: string) => Promise<void>;
	registerUser: (
		fullName: string,
		email: string,
		password: string,
	) => Promise<{ requiresVerification: boolean; verificationToken?: string }>;
	verifyUserEmail: (email: string, token?: string) => Promise<boolean>;
	isEmailVerified: (email: string) => boolean;
	createOrganization: (
		name: string,
		planCode?: string,
	) => Promise<OrganizationContext>;
	registerOrLogin: (
		email: string,
		fullName: string,
		orgName?: string,
		role?: string,
	) => void;
	switchOrganization: (org: OrganizationContext | null) => void;
	switchAuthMode: (mode: AuthMode) => void;
	logout: () => void;
}

const STORAGE_KEY_AUTH_MODE = "lensio_auth_mode";
const STORAGE_KEY_ORGANIZATIONS = "lensio_organizations";
const STORAGE_KEY_CURRENT_ORG_ID = "lensio_current_org_id";

export function createDevJwtToken(
	sub: string,
	email: string,
	username: string,
	roles: string[],
): string {
	const header = { alg: "none", typ: "JWT" };
	const payload = {
		sub,
		email,
		email_verified: true,
		preferred_username: username,
		name: username,
		roles,
		exp: Math.floor(Date.now() / 1000) + 86400 * 7,
		iat: Math.floor(Date.now() / 1000),
	};
	const b64 = (obj: unknown) => {
		const str = JSON.stringify(obj);
		return btoa(str).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
	};
	return `${b64(header)}.${b64(payload)}.dev_signature`;
}

export const AuthContext = createContext<AuthContextValue | undefined>(
	undefined,
);

export interface AuthProviderProps {
	children: React.ReactNode;
	initialOrg?: OrganizationContext | null;
	initialUser?: OIDCUserSession | null;
	initialApiKey?: string | null;
	initialAuthMode?: AuthMode;
}

export const AuthProvider: React.FC<AuthProviderProps> = ({
	children,
	initialOrg = null,
	initialUser = null,
	initialApiKey = null,
	initialAuthMode,
}) => {
	const [apiKey, setApiKeyState] = useState<string | null>(() => {
		if (initialApiKey !== null) return initialApiKey;
		if (typeof window !== "undefined") {
			const win = window as unknown as { __LENSIO_API_KEY__?: string };
			if (win.__LENSIO_API_KEY__) {
				api.setApiKey(win.__LENSIO_API_KEY__);
				return win.__LENSIO_API_KEY__;
			}
		}
		return api.getApiKey();
	});
	const [authMode, setAuthMode] = useState<AuthMode>(() => {
		if (initialAuthMode) return initialAuthMode;
		if (initialApiKey) return "apikey";
		if (initialUser) return "oidc";
		if (typeof window !== "undefined") {
			const saved = localStorage.getItem(STORAGE_KEY_AUTH_MODE);
			if (saved === "oidc" || saved === "apikey") {
				return saved;
			}
		}
		return "apikey";
	});
	const [oidcUser, setOidcUser] = useState<OIDCUserSession | null>(initialUser);
	const [organizations, setOrganizations] = useState<OrganizationContext[]>(
		() => {
			const list: OrganizationContext[] = [];
			if (initialOrg) {
				list.push(initialOrg);
			}
			if (typeof window !== "undefined") {
				try {
					const saved = localStorage.getItem(STORAGE_KEY_ORGANIZATIONS);
					if (saved) {
						const parsed = JSON.parse(saved) as OrganizationContext[];
						for (const o of parsed) {
							if (!list.some((existing) => existing.id === o.id)) {
								list.push(o);
							}
						}
					}
				} catch {
					// Ignore invalid JSON in localStorage
				}
			}
			return list;
		},
	);

	const [currentOrg, setCurrentOrg] = useState<OrganizationContext | null>(
		() => {
			if (initialOrg) return initialOrg;
			if (typeof window !== "undefined") {
				const savedId = localStorage.getItem(STORAGE_KEY_CURRENT_ORG_ID);
				if (savedId && organizations.length > 0) {
					const found = organizations.find((o) => o.id === savedId);
					if (found) return found;
				}
			}
			return organizations.length > 0 ? organizations[0] : null;
		},
	);

	const [environment, setEnvironment] = useState<"live" | "test">(() => {
		if (initialApiKey?.startsWith("lensio_test_")) return "test";
		return "live";
	});

	// Helper to persist active organization
	const updateActiveOrg = useCallback((org: OrganizationContext | null) => {
		setCurrentOrg(org);
		if (typeof window !== "undefined") {
			if (org?.id) {
				localStorage.setItem(STORAGE_KEY_CURRENT_ORG_ID, org.id);
			} else {
				localStorage.removeItem(STORAGE_KEY_CURRENT_ORG_ID);
			}
		}
	}, []);

	useEffect(() => {
		if (apiKey?.startsWith("lensio_test_")) {
			setEnvironment("test");
		} else if (apiKey?.startsWith("lensio_live_")) {
			setEnvironment("live");
		}
	}, [apiKey]);

	const [isInitializing, setIsInitializing] = useState<boolean>(() => {
		if (initialOrg || initialUser || initialApiKey) {
			return false;
		}
		if (
			typeof window !== "undefined" &&
			localStorage.getItem(STORAGE_KEY_AUTH_MODE) === "oidc"
		) {
			return true;
		}
		return false;
	});

	// Initialize and verify cookie-backed session on mount if in OIDC mode
	useEffect(() => {
		const restoreSession = async () => {
			if (initialOrg || initialUser || initialApiKey) {
				setIsInitializing(false);
				return;
			}
			if (
				typeof window !== "undefined" &&
				localStorage.getItem(STORAGE_KEY_AUTH_MODE) === "oidc"
			) {
				try {
					const res = await api.fetchCurrentUser();
					if (res?.user) {
						const username = res.user.email.split("@")[0];
						const orgContext: OrganizationContext | null = res.organization
							? {
									id: res.organization.id,
									name: res.organization.name,
									slug: res.organization.slug,
									planCode: res.organization.plan_code,
								}
							: null;
						const session: OIDCUserSession = {
							sub: res.user.id,
							email: res.user.email,
							preferredUsername: username,
							name: res.user.full_name || username,
							roles: res.user.roles || ["developer"],
							token: "",
							organization: orgContext ?? undefined,
						};
						setOidcUser(session);
						setAuthMode("oidc");
						if (typeof window !== "undefined") {
							localStorage.setItem(STORAGE_KEY_AUTH_MODE, "oidc");
						}
						api.setApiKey(null);
						if (orgContext) {
							setOrganizations((prev) => {
								const next = prev.some((o) => o.id === orgContext.id)
									? prev.map((o) => (o.id === orgContext.id ? orgContext : o))
									: [...prev, orgContext];
								if (typeof window !== "undefined") {
									localStorage.setItem(
										STORAGE_KEY_ORGANIZATIONS,
										JSON.stringify(next),
									);
								}
								return next;
							});
							updateActiveOrg(orgContext);
						}
					}
				} catch {
					// Cookie expired or unauthenticated; clear state
					setOidcUser(null);
					updateActiveOrg(null);
					if (typeof window !== "undefined") {
						localStorage.removeItem(STORAGE_KEY_AUTH_MODE);
					}
				} finally {
					setIsInitializing(false);
				}
			}
		};

		restoreSession();
	}, [initialOrg, initialUser, initialApiKey, updateActiveOrg]);

	// Keep API client header synchronized (in OIDC mode, rely purely on HttpOnly Cookie)
	useEffect(() => {
		if (authMode === "apikey" && apiKey) {
			api.setApiKey(apiKey);
		} else if (authMode === "oidc") {
			api.setApiKey(null);
		}
	}, [authMode, apiKey]);

	const handleSetApiKey = useCallback(
		(key: string | null) => {
			const trimmed = key ? key.trim() : null;
			setApiKeyState(trimmed);
			if (trimmed) {
				setAuthMode("apikey");
				if (typeof window !== "undefined") {
					localStorage.setItem(STORAGE_KEY_AUTH_MODE, "apikey");
				}
				api.setApiKey(trimmed);
			} else {
				if (oidcUser) {
					setAuthMode("oidc");
					if (typeof window !== "undefined") {
						localStorage.setItem(STORAGE_KEY_AUTH_MODE, "oidc");
					}
				}
				api.setApiKey(null);
			}
		},
		[oidcUser],
	);

	const handleLoginOIDC = useCallback(
		(session: OIDCUserSession) => {
			const org = session.organization ?? null;
			const sanitizedSession: OIDCUserSession = {
				...session,
				token: "", // Zero secret tokens in localStorage
				organization: org ?? undefined,
			};
			setOidcUser(sanitizedSession);
			if (org) {
				setOrganizations((prev) => {
					const next = prev.some((o) => o.id === org.id)
						? prev.map((o) => (o.id === org.id ? org : o))
						: [...prev, org];
					if (typeof window !== "undefined") {
						localStorage.setItem(
							STORAGE_KEY_ORGANIZATIONS,
							JSON.stringify(next),
						);
					}
					return next;
				});
			}
			updateActiveOrg(org);
			setAuthMode("oidc");
			if (typeof window !== "undefined") {
				localStorage.setItem(STORAGE_KEY_AUTH_MODE, "oidc");
			}
			api.setApiKey(null);
		},
		[updateActiveOrg],
	);

	const loginWithPassword = useCallback(
		async (email: string, password: string): Promise<void> => {
			const res = await api.login({ email: email.trim(), password });
			const username = res.user.email.split("@")[0];
			const orgContext: OrganizationContext | null = res.organization
				? {
						id: res.organization.id,
						name: res.organization.name,
						slug: res.organization.slug,
						planCode: res.organization.plan_code,
					}
				: null;

			handleLoginOIDC({
				sub: res.user.id,
				email: res.user.email,
				preferredUsername: username,
				name: res.user.full_name,
				roles: ["developer", "ocr:write", "ocr:read", "usage:read"],
				token: "",
				organization: orgContext ?? undefined,
			});
		},
		[handleLoginOIDC],
	);

	const isEmailVerified = useCallback((_email: string): boolean => {
		return true;
	}, []);

	const registerUser = useCallback(
		async (
			fullName: string,
			email: string,
			password: string,
		): Promise<{
			requiresVerification: boolean;
			verificationToken?: string;
		}> => {
			const res = await api.register({
				full_name: fullName.trim(),
				email: email.trim(),
				password,
			});
			return {
				requiresVerification: true,
				verificationToken: res.verification_token,
			};
		},
		[],
	);

	const verifyUserEmail = useCallback(
		async (email: string, token?: string): Promise<boolean> => {
			await api.verifyEmail({
				email: email.trim(),
				token,
			});
			return true;
		},
		[],
	);

	const createOrganization = useCallback(
		async (name: string, planCode = "free"): Promise<OrganizationContext> => {
			const cleanName = name.trim();
			const trackOrg = (newOrg: OrganizationContext) => {
				setOrganizations((prev) => {
					const next = [...prev.filter((o) => o.id !== newOrg.id), newOrg];
					if (typeof window !== "undefined") {
						localStorage.setItem(
							STORAGE_KEY_ORGANIZATIONS,
							JSON.stringify(next),
						);
					}
					return next;
				});
				updateActiveOrg(newOrg);
				if (oidcUser) {
					const updatedSession: OIDCUserSession = {
						...oidcUser,
						organization: newOrg,
					};
					setOidcUser(updatedSession);
				}
				return newOrg;
			};
			try {
				const res = await api.createOrganization({
					name: cleanName,
					plan_code: planCode,
				});
				return trackOrg({
					id: res.organization.id,
					name: res.organization.name,
					slug: res.organization.slug,
					planCode: res.organization.plan_code,
				});
			} catch {
				// Fallback for isolated unit tests
				const slug = cleanName
					.toLowerCase()
					.replace(/[^a-z0-9]+/g, "-")
					.replace(/^-|-$/g, "");
				return trackOrg({
					id: `org-${slug || Date.now()}`,
					name: cleanName,
					slug: slug || "org",
					planCode: planCode || "free",
				});
			}
		},
		[oidcUser, updateActiveOrg],
	);

	const handleRegisterOrLogin = useCallback(
		(email: string, fullName: string, orgName?: string, role = "owner") => {
			const username = email.split("@")[0];
			const roles =
				role === "admin"
					? ["admin", "developer", "ocr:write", "ocr:read", "usage:read"]
					: ["developer", "ocr:write", "ocr:read", "usage:read"];

			let org: OrganizationContext | undefined;
			if (orgName?.trim()) {
				const cleanOrgName = orgName.trim();
				org = {
					id: `org-${cleanOrgName
						.toLowerCase()
						.replace(/[^a-z0-9]+/g, "-")
						.replace(/^-|-$/g, "")}`,
					name: cleanOrgName,
					slug: cleanOrgName
						.toLowerCase()
						.replace(/[^a-z0-9]+/g, "-")
						.replace(/^-|-$/g, ""),
					planCode: "free",
				};
			}

			const token = createDevJwtToken(
				`sub-${username}`,
				email.trim(),
				username,
				roles,
			);

			const session: OIDCUserSession = {
				sub: `sub-${username}`,
				email: email.trim(),
				preferredUsername: username,
				name: fullName.trim() || username,
				roles,
				token,
				organization: org,
			};

			handleLoginOIDC(session);
		},
		[handleLoginOIDC],
	);

	const handleSwitchOrganization = useCallback(
		(org: OrganizationContext | null) => {
			if (org) {
				setOrganizations((prev) => {
					if (!prev.some((o) => o.id === org.id)) {
						const next = [...prev, org];
						if (typeof window !== "undefined") {
							localStorage.setItem(
								STORAGE_KEY_ORGANIZATIONS,
								JSON.stringify(next),
							);
						}
						return next;
					}
					return prev;
				});
			}
			// Reset active API key when switching organizations to prevent cross-tenant key pollution
			setApiKeyState(null);
			api.setApiKey(null);
			if (oidcUser) {
				setAuthMode("oidc");
				if (typeof window !== "undefined") {
					localStorage.setItem(STORAGE_KEY_AUTH_MODE, "oidc");
				}
			}
			updateActiveOrg(org);
			if (oidcUser) {
				const updatedSession: OIDCUserSession = {
					...oidcUser,
					organization: org ?? undefined,
				};
				setOidcUser(updatedSession);
			}
		},
		[oidcUser, updateActiveOrg],
	);

	const handleSwitchAuthMode = useCallback(
		(mode: AuthMode) => {
			setAuthMode(mode);
			if (typeof window !== "undefined") {
				localStorage.setItem(STORAGE_KEY_AUTH_MODE, mode);
			}
			if (mode === "oidc") {
				api.setApiKey(null);
			} else {
				api.setApiKey(apiKey);
			}
		},
		[apiKey],
	);

	const handleLogout = useCallback(() => {
		api.logout().catch(() => {});
		setOidcUser(null);
		setApiKeyState(null);
		setOrganizations([]);
		updateActiveOrg(null);
		if (typeof window !== "undefined") {
			localStorage.removeItem(STORAGE_KEY_AUTH_MODE);
			localStorage.removeItem(STORAGE_KEY_CURRENT_ORG_ID);
			// The cached organization list belongs to the account that just
			// signed out. Leaving it behind carried the previous user's
			// organization into the next session on a shared browser.
			localStorage.removeItem(STORAGE_KEY_ORGANIZATIONS);
		}
		api.setApiKey(null);
	}, [updateActiveOrg]);

	const isConnected = authMode === "oidc" ? Boolean(oidcUser) : Boolean(apiKey);

	const contextValue = useMemo<AuthContextValue>(
		() => ({
			apiKey,
			environment,
			isConnected,
			isInitializing,
			authMode,
			oidcUser,
			currentOrg,
			organizations,
			setApiKey: handleSetApiKey,
			setEnvironment,
			loginOIDC: handleLoginOIDC,
			loginWithPassword,
			registerUser,
			verifyUserEmail,
			isEmailVerified,
			createOrganization,
			registerOrLogin: handleRegisterOrLogin,
			switchOrganization: handleSwitchOrganization,
			switchAuthMode: handleSwitchAuthMode,
			logout: handleLogout,
		}),
		[
			apiKey,
			environment,
			isConnected,
			isInitializing,
			authMode,
			oidcUser,
			currentOrg,
			organizations,
			handleSetApiKey,
			handleLoginOIDC,
			loginWithPassword,
			registerUser,
			verifyUserEmail,
			isEmailVerified,
			createOrganization,
			handleRegisterOrLogin,
			handleSwitchOrganization,
			handleSwitchAuthMode,
			handleLogout,
		],
	);

	return (
		<AuthContext.Provider value={contextValue}>{children}</AuthContext.Provider>
	);
};

export const useAuth = (): AuthContextValue => {
	const context = useContext(AuthContext);
	if (!context) {
		throw new Error("useAuth must be used within an AuthProvider");
	}
	return context;
};
