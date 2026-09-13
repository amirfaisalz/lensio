import { BookOpen, Check, Copy, ExternalLink, Terminal } from "lucide-react";
import type React from "react";
import { useState } from "react";
import { useAuth } from "../context/AuthContext";

const DOC_TYPES = [
	{
		id: "ktp",
		label: "KTP",
		endpoint: "/api/v1/ocr/ktp",
		sampleFile: "ktp.jpg",
	},
	{
		id: "sim",
		label: "SIM",
		endpoint: "/api/v1/ocr/sim",
		sampleFile: "sim.jpg",
	},
	{
		id: "passport",
		label: "Passport",
		endpoint: "/api/v1/ocr/passport",
		sampleFile: "passport.jpg",
	},
	{
		id: "npwp",
		label: "NPWP",
		endpoint: "/api/v1/ocr/npwp",
		sampleFile: "npwp.jpg",
	},
	{ id: "kk", label: "KK", endpoint: "/api/v1/ocr/kk", sampleFile: "kk.jpg" },
	{
		id: "invoice",
		label: "Invoice",
		endpoint: "/api/v1/ocr/invoice",
		sampleFile: "invoice.jpg",
	},
] as const;

type DocTypeId = (typeof DOC_TYPES)[number]["id"];

export const DocsPage: React.FC = () => {
	const { apiKey } = useAuth();
	const [docType, setDocType] = useState<DocTypeId>("ktp");
	const [activeTab, setActiveTab] = useState<
		"curl" | "go" | "python" | "nodejs"
	>("curl");
	const [copied, setCopied] = useState(false);

	const displayKey = apiKey || "lensio_live_sample_key_12345678";
	const activeDoc = DOC_TYPES.find((d) => d.id === docType) ?? DOC_TYPES[0];
	const endpoint = activeDoc.endpoint;
	const sampleFile = activeDoc.sampleFile;
	const docLabel = activeDoc.label;

	const snippets = {
		curl: `# Extract ${docLabel} data via cURL
curl -X POST \\
  http://localhost:8080${endpoint} \\
  -H "Authorization: Bearer ${displayKey}" \\
  -F "document=@/path/to/${sampleFile}"`,

		go: `package main

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
)

func main() {
	file, _ := os.Open("${sampleFile}")
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("document", "${sampleFile}")
	io.Copy(part, file)
	writer.Close()

	req, _ := http.NewRequest("POST", "http://localhost:8080${endpoint}", body)
	req.Header.Set("Authorization", "Bearer ${displayKey}")
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	fmt.Println("Status:", resp.Status)
}`,

		python: `import requests

url = "http://localhost:8080${endpoint}"
headers = {
    "Authorization": "Bearer ${displayKey}"
}
files = {
    "document": open("${sampleFile}", "rb")
}

response = requests.post(url, headers=headers, files=files)
print(response.json())`,

		nodejs: `import fs from 'fs';
import FormData from 'form-data';
import fetch from 'node-fetch';

const form = new FormData();
form.append('document', fs.createReadStream('${sampleFile}'));

const response = await fetch('http://localhost:8080${endpoint}', {
  method: 'POST',
  headers: {
    'Authorization': 'Bearer ${displayKey}',
    ...form.getHeaders()
  },
  body: form
});

const data = await response.json();
console.log(data);`,
	};

	const handleCopySnippet = () => {
		navigator.clipboard.writeText(snippets[activeTab]);
		setCopied(true);
		setTimeout(() => setCopied(false), 2000);
	};

	const errorCodes = [
		{
			code: "invalid_request",
			status: "400 Bad Request",
			description:
				"Malformed JSON, missing fields, or invalid query parameters.",
		},
		{
			code: "invalid_api_key",
			status: "401 Unauthorized",
			description: "Missing, malformed, revoked, or expired Bearer API token.",
		},
		{
			code: "insufficient_scope",
			status: "403 Forbidden",
			description:
				"The token lacks the required scope (e.g. ocr:write, usage:read).",
		},
		{
			code: "rate_limit_exceeded",
			status: "429 Too Many Requests",
			description: "Per-minute rate limit exhausted. Check Retry-After header.",
		},
		{
			code: "quota_exceeded",
			status: "429 Too Many Requests",
			description: "Monthly account request quota has been fully depleted.",
		},
		{
			code: "invalid_document",
			status: "400 Bad Request",
			description:
				"File exceeds 5MB or contains unreadable/corrupt image bytes.",
		},
		{
			code: "unsupported_document",
			status: "422 Unprocessable",
			description:
				"Uploaded image is not a recognized Indonesian identity document.",
		},
		{
			code: "ocr_failed",
			status: "502 Bad Gateway",
			description:
				"Underlying OCR vision provider timed out or reported internal error.",
		},
		{
			code: "internal_error",
			status: "500 Server Error",
			description:
				"Internal processing failure. Includes correlated request_id.",
		},
	];

	return (
		<div className="space-y-6">
			{/* Top Banner */}
			<div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 sm:gap-4">
				<div>
					<h2 className="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
						API Documentation & Quickstart
					</h2>
					<p className="text-xs text-slate-500 dark:text-slate-400">
						Developer integration guide, code samples in multiple languages, and
						standardized error schemas.
					</p>
				</div>

				<a
					href="/docs"
					target="_blank"
					rel="noopener noreferrer"
					className="inline-flex items-center justify-center gap-1.5 px-3.5 py-2 text-xs font-semibold text-[#1877F2] dark:text-[#7aa9f5] bg-[#E7F3FF] dark:bg-[#1877F2]/20 hover:bg-[#d5eaff] dark:hover:bg-[#1877F2]/30 rounded-lg transition-colors w-full sm:w-auto"
				>
					<BookOpen className="w-4 h-4" />
					<span>Interactive OpenAPI Docs (/docs)</span>
					<ExternalLink className="w-3 h-3 ml-0.5" />
				</a>
			</div>

			{/* Code Snippets Box */}
			<div className="bg-slate-900 rounded-xl overflow-hidden shadow-lg border border-slate-800">
				<div className="px-4 py-3 bg-slate-950/80 border-b border-slate-800 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
					<div className="flex items-center gap-3">
						<div className="flex items-center gap-2">
							<Terminal className="w-4 h-4 text-[#1877F2]" />
							<span className="text-xs font-semibold text-slate-300">
								Quickstart Integration Snippet
							</span>
						</div>

						{/* Document Selector */}
						<div className="flex flex-wrap bg-slate-900 p-0.5 rounded-lg border border-slate-800 text-[11px] font-mono">
							{DOC_TYPES.map((doc) => (
								<button
									key={doc.id}
									type="button"
									onClick={() => setDocType(doc.id)}
									className={`px-2.5 py-1 rounded transition-colors cursor-pointer uppercase font-semibold ${
										docType === doc.id
											? "bg-[#1877F2] text-white"
											: "text-slate-400 dark:text-slate-500 hover:text-slate-200"
									}`}
								>
									{doc.label}
								</button>
							))}
						</div>
					</div>

					<div className="flex items-center justify-between sm:justify-end gap-2 w-full sm:w-auto">
						<div className="flex bg-slate-900 p-0.5 rounded-lg border border-slate-800 text-[11px] font-mono overflow-x-auto">
							{(["curl", "go", "python", "nodejs"] as const).map((lang) => (
								<button
									key={lang}
									type="button"
									onClick={() => setActiveTab(lang)}
									className={`px-3 py-1 rounded transition-colors cursor-pointer ${
										activeTab === lang
											? "bg-slate-700 text-white font-semibold"
											: "text-slate-400 dark:text-slate-500 hover:text-slate-200"
									}`}
								>
									{lang}
								</button>
							))}
						</div>

						<button
							type="button"
							onClick={handleCopySnippet}
							className="p-1.5 text-slate-400 dark:text-slate-500 hover:text-white rounded hover:bg-slate-800 transition-colors flex items-center gap-1 text-xs cursor-pointer shrink-0"
							title="Copy snippet"
						>
							{copied ? (
								<Check className="w-3.5 h-3.5 text-emerald-400" />
							) : (
								<Copy className="w-3.5 h-3.5" />
							)}
							<span className="hidden sm:inline">
								{copied ? "Copied" : "Copy"}
							</span>
						</button>
					</div>
				</div>

				<div className="p-4 sm:p-5 font-mono text-[11px] sm:text-xs text-slate-200 overflow-x-auto leading-relaxed">
					<pre>{snippets[activeTab]}</pre>
				</div>
			</div>

			{/* Standardized Error Codes Reference */}
			<div className="bg-white dark:bg-slate-900 rounded-xl border border-slate-200 dark:border-white/10 overflow-hidden">
				<div className="px-4 sm:px-6 py-4 border-b border-slate-100 flex items-center justify-between">
					<div>
						<h3 className="text-sm font-semibold text-slate-900 dark:text-white">
							Standardized API Error Codes
						</h3>
						<p className="text-xs text-slate-500 dark:text-slate-400">
							Every error response conforms to the standard PRD Section 20 error
							envelope.
						</p>
					</div>
				</div>

				<div className="overflow-x-auto">
					<table className="w-full text-left text-xs text-slate-600 dark:text-slate-400 min-w-[500px]">
						<thead className="bg-slate-50 dark:bg-white/5 border-b border-slate-200 dark:border-white/10 text-[11px] font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
							<tr>
								<th className="px-6 py-3.5">Error Code</th>
								<th className="px-6 py-3.5">HTTP Status</th>
								<th className="px-6 py-3.5">Description & Recovery</th>
							</tr>
						</thead>
						<tbody className="divide-y divide-slate-100 dark:divide-white/10 font-medium">
							{errorCodes.map((err) => (
								<tr
									key={err.code}
									className="hover:bg-slate-50/80 dark:hover:bg-white/5 transition-colors"
								>
									<td className="px-6 py-3.5 font-mono font-bold text-slate-900 dark:text-white">
										<span className="bg-slate-100 dark:bg-white/10 px-2 py-0.5 rounded border border-slate-200 dark:border-white/10">
											{err.code}
										</span>
									</td>
									<td className="px-6 py-3.5 font-mono text-slate-700 dark:text-slate-300">
										{err.status}
									</td>
									<td className="px-6 py-3.5 text-slate-600 dark:text-slate-400">
										{err.description}
									</td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
			</div>

			{/* Standard Response Headers */}
			<div className="bg-white dark:bg-slate-900 p-6 rounded-xl border border-slate-200 dark:border-white/10">
				<h3 className="text-sm font-semibold text-slate-900 dark:text-white mb-2">
					Platform Headers
				</h3>
				<p className="text-xs text-slate-500 dark:text-slate-400 mb-4">
					Every API response returns standard metadata and rate limit tracking
					headers:
				</p>
				<div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs font-mono">
					<div className="p-3 bg-slate-50 dark:bg-white/5 rounded-lg border border-slate-200 dark:border-white/10">
						<strong className="text-[#1877F2]">X-Request-ID</strong>
						<p className="font-sans text-[11px] text-slate-600 dark:text-slate-400 mt-1">
							Unique request correlation identifier (e.g. req_01JABC...).
						</p>
					</div>
					<div className="p-3 bg-slate-50 dark:bg-white/5 rounded-lg border border-slate-200 dark:border-white/10">
						<strong className="text-[#1877F2]">X-RateLimit-Limit</strong>
						<p className="font-sans text-[11px] text-slate-600 dark:text-slate-400 mt-1">
							Maximum allowed requests per minute on your current plan.
						</p>
					</div>
					<div className="p-3 bg-slate-50 dark:bg-white/5 rounded-lg border border-slate-200 dark:border-white/10">
						<strong className="text-[#1877F2]">X-RateLimit-Remaining</strong>
						<p className="font-sans text-[11px] text-slate-600 dark:text-slate-400 mt-1">
							Remaining quota units in current 60-second window.
						</p>
					</div>
					<div className="p-3 bg-slate-50 dark:bg-white/5 rounded-lg border border-slate-200 dark:border-white/10">
						<strong className="text-[#1877F2]">Retry-After</strong>
						<p className="font-sans text-[11px] text-slate-600 dark:text-slate-400 mt-1">
							Seconds to wait before resending after a 429 response.
						</p>
					</div>
				</div>
			</div>
		</div>
	);
};
