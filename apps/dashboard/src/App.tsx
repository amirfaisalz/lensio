import type React from "react";
import { useEffect, useState } from "react";
import { Header } from "./components/layout/Header";
import { type NavigationPage, Sidebar } from "./components/layout/Sidebar";
import { AuthProvider } from "./context/AuthContext";
import { AccountPage } from "./pages/AccountPage";
import { APIKeysPage } from "./pages/APIKeysPage";
import { DocsPage } from "./pages/DocsPage";
import { OverviewPage } from "./pages/OverviewPage";
import { RequestsPage } from "./pages/RequestsPage";
import { UsagePage } from "./pages/UsagePage";
import { api } from "./services/api";

const pageMeta: Record<NavigationPage, { title: string; subtitle: string }> = {
	overview: {
		title: "Overview",
		subtitle: "Lensio KTP OCR developer analytics & quick test harness",
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

export const DashboardLayout: React.FC = () => {
	const [currentPage, setCurrentPage] = useState<NavigationPage>("overview");
	const [isHealthy, setIsHealthy] = useState(true);

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

	const renderPage = () => {
		switch (currentPage) {
			case "overview":
				return <OverviewPage />;
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
				return <OverviewPage />;
		}
	};

	const meta = pageMeta[currentPage];

	return (
		<div className="flex min-h-screen bg-[#F0F2F5] text-slate-900 font-sans antialiased">
			{/* Sidebar navigation */}
			<Sidebar
				currentPage={currentPage}
				onNavigate={setCurrentPage}
				isHealthy={isHealthy}
			/>

			{/* Main Content Area */}
			<div className="flex-1 flex flex-col min-w-0">
				<Header
					title={meta.title}
					subtitle={meta.subtitle}
					onQuickTestClick={() => setCurrentPage("overview")}
				/>

				<main className="flex-1 p-8 max-w-7xl w-full mx-auto">
					{renderPage()}
				</main>
			</div>
		</div>
	);
};

export const App: React.FC = () => {
	return (
		<AuthProvider>
			<DashboardLayout />
		</AuthProvider>
	);
};

export default App;
