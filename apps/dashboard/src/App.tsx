import type React from "react";
import { useEffect, useState } from "react";
import {
	BrowserRouter,
	Navigate,
	Route,
	Routes,
	useLocation,
	useNavigate,
	useParams,
} from "react-router-dom";
import { Header } from "./components/layout/Header";
import { type NavigationPage, Sidebar } from "./components/layout/Sidebar";
import { AuthProvider, useAuth } from "./context/AuthContext";
import { ThemeProvider } from "./context/ThemeContext";
import { AccountPage } from "./pages/AccountPage";
import { APIKeysPage } from "./pages/APIKeysPage";
import { DocsPage } from "./pages/DocsPage";
import { LoginPage } from "./pages/LoginPage";
import { OverviewPage } from "./pages/OverviewPage";
import { PlaygroundPage } from "./pages/PlaygroundPage";
import { RequestsPage } from "./pages/RequestsPage";
import { UsagePage } from "./pages/UsagePage";
import { api } from "./services/api";

const pageMeta: Record<NavigationPage, { title: string; subtitle: string }> = {
	overview: {
		title: "Overview",
		subtitle: "System overview, quota monitoring, and service metrics",
	},
	playground: {
		title: "OCR Playground",
		subtitle:
			"Interactive test harness for Indonesian KTP & SIM OCR extraction",
	},
	keys: {
		title: "API Keys",
		subtitle: "Generate and manage cryptographically secured API tokens",
	},
	usage: {
		title: "Usage & Analytics",
		subtitle: "Request rates, daily timeseries, and endpoint partitioning",
	},
	requests: {
		title: "Requests Explorer",
		subtitle: "Inspect paginated request audit logs and execution latencies",
	},
	docs: {
		title: "API Documentation",
		subtitle: "Quickstart code snippets and standardized error schemas",
	},
	account: {
		title: "Account & Settings",
		subtitle: "Organization details, subscription tier, and team members",
	},
};

const VALID_PAGES: NavigationPage[] = [
	"overview",
	"playground",
	"keys",
	"usage",
	"requests",
	"docs",
	"account",
];

export const DashboardLayout: React.FC = () => {
	const navigate = useNavigate();
	const params = useParams<{ page?: string }>();
	const [isHealthy, setIsHealthy] = useState(true);
	const [isMobileNavOpen, setIsMobileNavOpen] = useState(false);

	// Resolve active navigation page from route params or fallback to overview
	const currentPage: NavigationPage =
		params.page && VALID_PAGES.includes(params.page as NavigationPage)
			? (params.page as NavigationPage)
			: "overview";

	useEffect(() => {
		const checkSystemHealth = async () => {
			try {
				const res = await api.checkHealth();
				setIsHealthy(res.status === "ok");
			} catch {
				setIsHealthy(false);
			}
		};

		checkSystemHealth();
		const interval = setInterval(checkSystemHealth, 30000);
		return () => clearInterval(interval);
	}, []);

	const handleNavigate = (page: NavigationPage) => {
		setIsMobileNavOpen(false);
		navigate(`/dashboard/${page}`);
	};

	const renderPage = () => {
		switch (currentPage) {
			case "overview":
				return <OverviewPage onNavigate={handleNavigate} />;
			case "playground":
				return <PlaygroundPage onNavigate={handleNavigate} />;
			case "keys":
				return <APIKeysPage />;
			case "usage":
				return <UsagePage />;
			case "requests":
				return <RequestsPage />;
			case "docs":
				return <DocsPage />;
			case "account":
				return <AccountPage />;
			default:
				return <OverviewPage onNavigate={handleNavigate} />;
		}
	};

	const meta = pageMeta[currentPage];

	return (
		<div className="flex min-h-screen bg-[#F0F2F5] dark:bg-slate-950 text-slate-900 dark:text-slate-100 font-sans antialiased">
			{/* Sidebar navigation (responsive drawer on mobile, sticky on desktop) */}
			<Sidebar
				currentPage={currentPage}
				onNavigate={handleNavigate}
				isHealthy={isHealthy}
				isOpenMobile={isMobileNavOpen}
				onCloseMobile={() => setIsMobileNavOpen(false)}
			/>

			{/* Main Content Area */}
			<div className="flex-1 flex flex-col min-w-0">
				<Header
					title={meta.title}
					subtitle={meta.subtitle}
					onOpenMobileNav={() => setIsMobileNavOpen(true)}
				/>

				<main className="flex-1 p-4 sm:p-6 lg:p-8 max-w-7xl w-full mx-auto">
					{renderPage()}
				</main>
			</div>
		</div>
	);
};

// Protected Route Guard: If not authenticated, forcefully redirect to /login
export const ProtectedRoute: React.FC<{ children: React.ReactNode }> = ({
	children,
}) => {
	const { isConnected, isInitializing } = useAuth();
	const location = useLocation();

	if (isInitializing) {
		return (
			<div
				data-testid="auth-loading"
				className="flex min-h-screen items-center justify-center bg-[#F0F2F5] dark:bg-slate-950"
			>
				<div className="animate-spin rounded-full h-8 w-8 border-b-2 border-[#1877F2]" />
			</div>
		);
	}

	if (!isConnected) {
		return <Navigate to="/login" state={{ from: location }} replace />;
	}

	return <>{children}</>;
};

// Public Only Route Guard: If already authenticated, redirect to /dashboard
export const PublicOnlyRoute: React.FC<{
	children: React.ReactNode;
}> = ({ children }) => {
	const { isConnected, isInitializing } = useAuth();

	if (isInitializing) {
		return (
			<div
				data-testid="auth-loading"
				className="flex min-h-screen items-center justify-center bg-[#F0F2F5] dark:bg-slate-950"
			>
				<div className="animate-spin rounded-full h-8 w-8 border-b-2 border-[#1877F2]" />
			</div>
		);
	}

	if (isConnected) {
		return <Navigate to="/dashboard" replace />;
	}

	return <>{children}</>;
};

export const RootRedirect: React.FC = () => {
	const { isConnected, isInitializing } = useAuth();

	if (isInitializing) {
		return (
			<div
				data-testid="auth-loading"
				className="flex min-h-screen items-center justify-center bg-[#F0F2F5] dark:bg-slate-950"
			>
				<div className="animate-spin rounded-full h-8 w-8 border-b-2 border-[#1877F2]" />
			</div>
		);
	}

	return <Navigate to={isConnected ? "/dashboard" : "/login"} replace />;
};

export const AppRoutes: React.FC = () => {
	return (
		<Routes>
			{/* Public Authentication Routes */}
			<Route
				path="/login"
				element={
					<PublicOnlyRoute>
						<LoginPage defaultMode="login" />
					</PublicOnlyRoute>
				}
			/>
			<Route
				path="/register"
				element={
					<PublicOnlyRoute>
						<LoginPage defaultMode="register" />
					</PublicOnlyRoute>
				}
			/>

			{/* Protected Dashboard Routes */}
			<Route
				path="/dashboard"
				element={
					<ProtectedRoute>
						<DashboardLayout />
					</ProtectedRoute>
				}
			/>
			<Route
				path="/dashboard/:page"
				element={
					<ProtectedRoute>
						<DashboardLayout />
					</ProtectedRoute>
				}
			/>

			{/* Root Redirect */}
			<Route path="/" element={<RootRedirect />} />

			{/* Catch-all Fallback */}
			<Route path="*" element={<RootRedirect />} />
		</Routes>
	);
};

export const App: React.FC = () => {
	return (
		<ThemeProvider>
			<AuthProvider>
				<BrowserRouter>
					<AppRoutes />
				</BrowserRouter>
			</AuthProvider>
		</ThemeProvider>
	);
};

export default App;
