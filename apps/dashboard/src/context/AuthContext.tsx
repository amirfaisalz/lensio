import type React from "react";
import { createContext, useContext, useEffect, useState } from "react";
import { api } from "../services/api";

export type AuthMode = "apikey" | "oidc";

export interface OIDCUserSession {
	sub: string;
	email: string;
	preferredUsername: string;
	name?: string;
	roles?: string[];
	token: string;
}

export interface AuthContextValue {
	apiKey: string | null;
	environment: "live" | "test";
	isConnected: boolean;
	authMode: AuthMode;
	oidcUser: OIDCUserSession | null;
	setApiKey: (key: string | null) => void;
	setEnvironment: (env: "live" | "test") => void;
	loginOIDC: (session: OIDCUserSession) => void;
	switchAuthMode: (mode: AuthMode) => void;
	logout: () => void;
}

const STORAGE_KEY_AUTH_MODE = "lensio_auth_mode";
const STORAGE_KEY_OIDC_USER = "lensio_oidc_user";

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({
	children,
}) => {
	const [apiKey, setApiKeyState] = useState<string | null>(() =>
		api.getApiKey(),
	);
	const [authMode, setAuthMode] = useState<AuthMode>(() => {
		if (typeof window !== "undefined") {
			const saved = localStorage.getItem(STORAGE_KEY_AUTH_MODE);
			if (saved === "oidc" || saved === "apikey") {
				return saved;
			}
		}
		return "apikey";
	});
	const [oidcUser, setOidcUser] = useState<OIDCUserSession | null>(() => {
		if (typeof window !== "undefined") {
			const saved = localStorage.getItem(STORAGE_KEY_OIDC_USER);
			if (saved) {
				try {
					return JSON.parse(saved);
				} catch {
					return null;
				}
			}
		}
		return null;
	});
	const [environment, setEnvironment] = useState<"live" | "test">("live");

	useEffect(() => {
		if (apiKey?.startsWith("lensio_test_")) {
			setEnvironment("test");
		} else if (apiKey?.startsWith("lensio_live_")) {
			setEnvironment("live");
		}
	}, [apiKey]);

	const handleSetApiKey = (key: string | null) => {
		const trimmed = key ? key.trim() : null;
		setApiKeyState(trimmed);
		setAuthMode("apikey");
		if (typeof window !== "undefined") {
			localStorage.setItem(STORAGE_KEY_AUTH_MODE, "apikey");
		}
		api.setApiKey(trimmed);
	};

	const handleLoginOIDC = (session: OIDCUserSession) => {
		setOidcUser(session);
		setAuthMode("oidc");
		if (typeof window !== "undefined") {
			localStorage.setItem(STORAGE_KEY_AUTH_MODE, "oidc");
			localStorage.setItem(STORAGE_KEY_OIDC_USER, JSON.stringify(session));
		}
		api.setApiKey(session.token);
	};

	const handleSwitchAuthMode = (mode: AuthMode) => {
		setAuthMode(mode);
		if (typeof window !== "undefined") {
			localStorage.setItem(STORAGE_KEY_AUTH_MODE, mode);
		}
		if (mode === "oidc") {
			api.setApiKey(oidcUser?.token ?? null);
		} else {
			api.setApiKey(apiKey);
		}
	};

	const handleLogout = () => {
		if (authMode === "oidc") {
			setOidcUser(null);
			if (typeof window !== "undefined") {
				localStorage.removeItem(STORAGE_KEY_OIDC_USER);
			}
		} else {
			setApiKeyState(null);
		}
		api.setApiKey(null);
	};

	const isConnected = authMode === "oidc" ? Boolean(oidcUser) : Boolean(apiKey);

	return (
		<AuthContext.Provider
			value={{
				apiKey,
				environment,
				isConnected,
				authMode,
				oidcUser,
				setApiKey: handleSetApiKey,
				setEnvironment,
				loginOIDC: handleLoginOIDC,
				switchAuthMode: handleSwitchAuthMode,
				logout: handleLogout,
			}}
		>
			{children}
		</AuthContext.Provider>
	);
};

export const useAuth = (): AuthContextValue => {
	const context = useContext(AuthContext);
	if (!context) {
		throw new Error("useAuth must be used within an AuthProvider");
	}
	return context;
};
