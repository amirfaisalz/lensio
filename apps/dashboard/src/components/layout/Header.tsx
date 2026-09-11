import React, { useState } from 'react';
import { KeyRound, LogOut, Check } from 'lucide-react';
import { useAuth } from '../../context/AuthContext';
import { Modal } from '../common/Modal';

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
  const { apiKey, setApiKey, logout } = useAuth();
  const [isAuthModalOpen, setIsAuthModalOpen] = useState(false);
  const [inputKey, setInputKey] = useState('');

  const handleSaveKey = (e: React.FormEvent) => {
    e.preventDefault();
    if (inputKey.trim()) {
      setApiKey(inputKey.trim());
      setInputKey('');
      setIsAuthModalOpen(false);
    }
  };

  return (
    <header className="h-16 bg-white border-b border-slate-200 px-8 flex items-center justify-between sticky top-0 z-30">
      <div>
        <h1 className="text-lg font-bold text-slate-900 tracking-tight">{title}</h1>
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

        <button
          type="button"
          onClick={() => setIsAuthModalOpen(true)}
          className="inline-flex items-center gap-2 px-3 py-1.5 text-xs font-semibold rounded-lg border border-slate-200 bg-white text-slate-700 hover:bg-slate-50 transition-colors"
        >
          <KeyRound className="w-3.5 h-3.5 text-[#1877F2]" />
          <span>{apiKey ? 'Change API Key' : 'Connect Key'}</span>
        </button>

        {apiKey && (
          <button
            type="button"
            onClick={logout}
            title="Disconnect active key"
            className="p-1.5 text-slate-400 hover:text-rose-600 rounded-lg transition-colors hover:bg-slate-100"
          >
            <LogOut className="w-4 h-4" />
          </button>
        )}
      </div>

      {/* Connect API Key Modal */}
      <Modal
        isOpen={isAuthModalOpen}
        onClose={() => setIsAuthModalOpen(false)}
        title="Connect API Key"
        footer={
          <>
            <button
              type="button"
              onClick={() => setIsAuthModalOpen(false)}
              className="px-4 py-2 text-xs font-medium text-slate-700 bg-white border border-slate-200 rounded-lg hover:bg-slate-50"
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={handleSaveKey}
              className="px-4 py-2 text-xs font-semibold text-white bg-[#1877F2] hover:bg-[#166FE5] rounded-lg transition-colors"
            >
              Save & Authenticate
            </button>
          </>
        }
      >
        <form onSubmit={handleSaveKey} className="space-y-4">
          <p className="text-xs text-slate-600">
            Paste your active NusaID API key (<code className="font-mono text-[11px] bg-slate-100 px-1 py-0.5 rounded">nusa_live_...</code> or <code className="font-mono text-[11px] bg-slate-100 px-1 py-0.5 rounded">nusa_test_...</code>).
            It will be stored locally in your browser session.
          </p>
          <div>
            <label htmlFor="api-key-input" className="block text-xs font-medium text-slate-700 mb-1">
              API Key Token
            </label>
            <input
              id="api-key-input"
              type="password"
              placeholder="nusa_live_..."
              value={inputKey}
              onChange={(e) => setInputKey(e.target.value)}
              className="w-full px-3 py-2 text-sm font-mono border border-slate-300 rounded-lg focus:outline-hidden focus:border-[#1877F2] focus:ring-2 focus:ring-[#1877F2]/20"
              autoFocus
            />
          </div>
          {apiKey && (
            <div className="p-3 bg-slate-50 rounded-lg border border-slate-200 text-xs text-slate-600 flex items-center justify-between">
              <div>
                <p className="font-medium text-slate-800">Currently Connected</p>
                <p className="font-mono text-[11px] text-slate-500">{apiKey.slice(0, 16)}••••••••</p>
              </div>
              <span className="inline-flex items-center gap-1 text-emerald-600 font-semibold text-[11px]">
                <Check className="w-3.5 h-3.5" /> Active
              </span>
            </div>
          )}
        </form>
      </Modal>
    </header>
  );
};
