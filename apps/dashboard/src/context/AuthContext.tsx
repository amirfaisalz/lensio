import type React from "react";
import { createContext, useContext, useEffect, useState } from "react";
import { api } from "../services/api";

interface AuthContextValue {
	apiKey: string | null;
	environment: "live" | "test";
	isConnected: boolean;
	setApiKey: (key: string | null) => void;
	setEnvironment: (env: "live" | "test") => void;
	logout: () => void;
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({
	children,
}) => {
	const [apiKey, setApiKeyState] = useState<string | null>(api.getApiKey());
	const [environment, setEnvironment] = useState<"live" | "test">("live");

	useEffect(() => {
		if (apiKey?.startsWith("nusa_test_")) {
			setEnvironment("test");
		} else if (apiKey?.startsWith("nusa_live_")) {
			setEnvironment("live");
		}
	}, [apiKey]);

	const handleSetApiKey = (key: string | null) => {
		const trimmed = key ? key.trim() : null;
		api.setApiKey(trimmed);
		setApiKeyState(trimmed);
	};

	const handleLogout = () => {
		handleSetApiKey(null);
	};

	return (
		<AuthContext.Provider
			value={{
				apiKey,
				environment,
				isConnected: Boolean(apiKey),
				setApiKey: handleSetApiKey,
				setEnvironment,
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
