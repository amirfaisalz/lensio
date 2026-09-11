import React, { useState, useEffect } from 'react';
import {
  RefreshCw,
  ChevronLeft,
  ChevronRight,
  ShieldCheck,
  CheckCircle2,
  AlertTriangle,
  XCircle,
  Eye,
} from 'lucide-react';
import { api } from '../services/api';
import type { UsageRecord } from '../types/api';
import { Modal } from '../components/common/Modal';
import { TableSkeleton } from '../components/common/Skeleton';

export const RequestsPage: React.FC = () => {
  const [records, setRecords] = useState<UsageRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [limit] = useState(25);
  const [offset, setOffset] = useState(0);
  const [statusCodeFilter, setStatusCodeFilter] = useState<number | undefined>(undefined);
  const [endpointFilter, setEndpointFilter] = useState('');
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Detail Modal State
  const [selectedRecord, setSelectedRecord] = useState<UsageRecord | null>(null);

  const loadRecords = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const res = await api.fetchUsageRecords({
        limit,
        offset,
        status_code: statusCodeFilter,
        endpoint: endpointFilter || undefined,
      });
      setRecords(res.data || []);
      setTotal(res.total || 0);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load request logs');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    loadRecords();
  }, [limit, offset, statusCodeFilter, endpointFilter]);

  const getStatusBadge = (code: number) => {
    if (code >= 200 && code < 300) {
      return (
        <span className="inline-flex items-center gap-1 text-emerald-700 bg-emerald-50 border border-emerald-200 px-2 py-0.5 rounded text-xs font-semibold tabular-nums">
          <CheckCircle2 className="w-3 h-3 text-emerald-600" />
          {code} OK
        </span>
      );
    }
    if (code >= 400 && code < 500) {
      return (
        <span className="inline-flex items-center gap-1 text-amber-700 bg-amber-50 border border-amber-200 px-2 py-0.5 rounded text-xs font-semibold tabular-nums">
          <AlertTriangle className="w-3 h-3 text-amber-600" />
          {code}
        </span>
      );
    }
    return (
      <span className="inline-flex items-center gap-1 text-rose-700 bg-rose-50 border border-rose-200 px-2 py-0.5 rounded text-xs font-semibold tabular-nums">
        <XCircle className="w-3 h-3 text-rose-600" />
        {code}
      </span>
    );
  };

  const totalPages = Math.ceil(total / limit) || 1;
  const currentPage = Math.floor(offset / limit) + 1;

  return (
    <div className="space-y-6 animate-in fade-in duration-200">
      {/* Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h2 className="text-xl font-bold text-slate-900 tracking-tight">Requests Explorer</h2>
          <p className="text-xs text-slate-500">
            Audit trail of API requests with status codes, latency, and non-PII execution metadata.
          </p>
        </div>

        <button
          type="button"
          onClick={loadRecords}
          disabled={isLoading}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold text-slate-700 bg-white border border-slate-200 rounded-lg hover:bg-slate-50 transition-colors"
        >
          <RefreshCw className={`w-3.5 h-3.5 ${isLoading ? 'animate-spin' : ''}`} />
          <span>Refresh</span>
        </button>
      </div>

      {error && (
        <div className="p-4 bg-rose-50 border border-rose-200 rounded-xl text-xs text-rose-700 flex items-center justify-between">
          <span>{error}</span>
          <button type="button" onClick={loadRecords} className="font-semibold underline">Retry</button>
        </div>
      )}

      {/* Filter Bar */}
      <div className="bg-white p-4 rounded-xl border border-slate-200 shadow-xs flex flex-wrap items-center justify-between gap-4 text-xs">
        <div className="flex flex-wrap items-center gap-3">
          {/* Status Code Filter */}
          <div className="flex items-center gap-2">
            <span className="text-slate-500 font-medium">Status:</span>
            <select
              value={statusCodeFilter ?? ''}
              onChange={(e) => {
                const val = e.target.value ? Number(e.target.value) : undefined;
                setStatusCodeFilter(val);
                setOffset(0);
              }}
              className="px-2.5 py-1.5 border border-slate-300 rounded-lg text-xs bg-white text-slate-800 focus:outline-hidden focus:border-[#1877F2]"
            >
              <option value="">All Statuses</option>
              <option value="200">200 OK</option>
              <option value="400">400 Bad Request</option>
              <option value="401">401 Unauthorized</option>
              <option value="403">403 Forbidden</option>
              <option value="422">422 Unsupported</option>
              <option value="429">429 Rate Limited</option>
              <option value="500">500 Server Error</option>
              <option value="502">502 Provider Error</option>
            </select>
          </div>

          {/* Endpoint text filter */}
          <div className="flex items-center gap-2">
            <span className="text-slate-500 font-medium">Route:</span>
            <input
              type="text"
              placeholder="e.g. /api/v1/ocr/ktp"
              value={endpointFilter}
              onChange={(e) => {
                setEndpointFilter(e.target.value.trim());
                setOffset(0);
              }}
              className="px-2.5 py-1.5 border border-slate-300 rounded-lg text-xs font-mono bg-white text-slate-800 focus:outline-hidden focus:border-[#1877F2]"
            />
          </div>
        </div>

        <div className="text-xs text-slate-500 tabular-nums">
          Showing <strong className="text-slate-800">{records.length}</strong> of {total.toLocaleString()} records
        </div>
      </div>

      {/* Requests Table */}
      <div className="bg-white rounded-xl border border-slate-200 shadow-xs overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs text-slate-600">
            <thead className="bg-slate-50 border-b border-slate-200 text-[11px] font-semibold text-slate-500 uppercase tracking-wider">
              <tr>
                <th className="px-6 py-3.5">Status</th>
                <th className="px-6 py-3.5">Endpoint</th>
                <th className="px-6 py-3.5">Request ID</th>
                <th className="px-6 py-3.5">Latency</th>
                <th className="px-6 py-3.5">Timestamp</th>
                <th className="px-6 py-3.5 text-right">Details</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 font-medium">
              {isLoading ? (
                <tr>
                  <td colSpan={6} className="px-6 py-8">
                    <TableSkeleton rows={5} cols={6} />
                  </td>
                </tr>
              ) : records.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-6 py-12 text-center text-slate-400">
                    No matching requests found.
                  </td>
                </tr>
              ) : (
                records.map((r) => (
                  <tr
                    key={r.id}
                    onClick={() => setSelectedRecord(r)}
                    className="hover:bg-slate-50/80 cursor-pointer transition-colors"
                  >
                    <td className="px-6 py-3.5">{getStatusBadge(r.status_code)}</td>
                    <td className="px-6 py-3.5 font-mono font-semibold text-slate-900">{r.endpoint}</td>
                    <td className="px-6 py-3.5 font-mono text-[11px] text-slate-500">
                      {r.request_id || 'n/a'}
                    </td>
                    <td className="px-6 py-3.5 font-mono text-slate-700 tabular-nums">
                      {r.latency_ms} ms
                    </td>
                    <td className="px-6 py-3.5 text-slate-500 tabular-nums">
                      {new Date(r.timestamp).toLocaleDateString(undefined, {
                        month: 'short',
                        day: 'numeric',
                        hour: '2-digit',
                        minute: '2-digit',
                        second: '2-digit',
                      })}
                    </td>
                    <td className="px-6 py-3.5 text-right">
                      <button
                        type="button"
                        onClick={(e) => {
                          e.stopPropagation();
                          setSelectedRecord(r);
                        }}
                        className="text-slate-400 hover:text-[#1877F2] p-1 rounded hover:bg-slate-100 transition-colors"
                        title="View request details"
                      >
                        <Eye className="w-4 h-4" />
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination Footer */}
        <div className="px-6 py-3.5 border-t border-slate-100 bg-slate-50/50 flex items-center justify-between text-xs text-slate-500">
          <div>
            Page <strong className="text-slate-800">{currentPage}</strong> of {totalPages}
          </div>

          <div className="flex items-center gap-2">
            <button
              type="button"
              disabled={offset === 0 || isLoading}
              onClick={() => setOffset(Math.max(0, offset - limit))}
              className="p-1.5 border border-slate-200 bg-white rounded-md disabled:opacity-40 hover:bg-slate-50 transition-colors"
            >
              <ChevronLeft className="w-4 h-4" />
            </button>
            <button
              type="button"
              disabled={offset + limit >= total || isLoading}
              onClick={() => setOffset(offset + limit)}
              className="p-1.5 border border-slate-200 bg-white rounded-md disabled:opacity-40 hover:bg-slate-50 transition-colors"
            >
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>
        </div>
      </div>

      {/* Request Details Modal */}
      {selectedRecord && (
        <Modal
          isOpen={true}
          onClose={() => setSelectedRecord(null)}
          title="Request Audit Details"
          footer={
            <button
              type="button"
              onClick={() => setSelectedRecord(null)}
              className="px-4 py-2 text-xs font-semibold text-slate-700 bg-white border border-slate-200 rounded-lg hover:bg-slate-50"
            >
              Close
            </button>
          }
        >
          <div className="space-y-4">
            <div className="flex items-center justify-between p-3 bg-slate-50 rounded-lg border border-slate-200">
              <span className="text-xs text-slate-500">HTTP Status</span>
              {getStatusBadge(selectedRecord.status_code)}
            </div>

            <div className="space-y-2 text-xs">
              <div className="flex justify-between py-1 border-b border-slate-100">
                <span className="text-slate-500">Request ID</span>
                <span className="font-mono text-slate-900 font-semibold">{selectedRecord.request_id}</span>
              </div>
              <div className="flex justify-between py-1 border-b border-slate-100">
                <span className="text-slate-500">Target Endpoint</span>
                <span className="font-mono text-slate-900">{selectedRecord.endpoint}</span>
              </div>
              <div className="flex justify-between py-1 border-b border-slate-100">
                <span className="text-slate-500">Execution Latency</span>
                <span className="font-mono text-slate-900">{selectedRecord.latency_ms} ms</span>
              </div>
              <div className="flex justify-between py-1 border-b border-slate-100">
                <span className="text-slate-500">Timestamp</span>
                <span className="font-mono text-slate-900">{new Date(selectedRecord.timestamp).toISOString()}</span>
              </div>
              <div className="flex justify-between py-1 border-b border-slate-100">
                <span className="text-slate-500">API Key UUID</span>
                <span className="font-mono text-slate-700">{selectedRecord.api_key_id || 'System / Direct'}</span>
              </div>
            </div>

            {/* Privacy note */}
            <div className="p-3 bg-emerald-50 border border-emerald-200 rounded-lg flex items-start gap-2 text-xs text-emerald-800">
              <ShieldCheck className="w-4 h-4 text-emerald-600 shrink-0 mt-0.5" />
              <div>
                <strong>Strict Privacy & Data Minimization:</strong> Raw images and identity data (NIK, full names, addresses) are processed strictly in ephemeral memory and never persisted into queryable databases or logs.
              </div>
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
};
