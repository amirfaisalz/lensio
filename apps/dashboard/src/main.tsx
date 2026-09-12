import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import "./index.css";
import { APP_VERSION } from "./version";

console.info(
	`%c Lensio Dashboard v${APP_VERSION} `,
	"background: #1877F2; color: #ffffff; font-weight: bold; border-radius: 4px; padding: 2px 6px;",
);

const rootElement = document.getElementById("root");
if (rootElement) {
	ReactDOM.createRoot(rootElement).render(
		<React.StrictMode>
			<App />
		</React.StrictMode>,
	);
}
