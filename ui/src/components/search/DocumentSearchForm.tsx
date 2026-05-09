"use client";

import { useState } from "react";
import { Search, FileText, X, Loader2 } from "lucide-react";
import { FileUploadZone } from "./FileUploadZone";
import { BoostTermsInput } from "./BoostTermsInput";
import { cn } from "@/lib/utils";

interface DocumentSearchFormProps {
  onSearch: (file: File, boostTerms: string[]) => Promise<void>;
}

export function DocumentSearchForm({ onSearch }: DocumentSearchFormProps) {
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [boostTerms, setBoostTerms] = useState<string[]>([]);
  const [isSearching, setIsSearching] = useState(false);

  const handleFileSelect = (file: File) => {
    setSelectedFile(file);
  };

  const handleRemoveFile = () => {
    setSelectedFile(null);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedFile || isSearching) return;

    setIsSearching(true);
    try {
      await onSearch(selectedFile, boostTerms);
    } finally {
      setIsSearching(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="w-full space-y-4">
      {!selectedFile ? (
        <FileUploadZone onFileSelect={handleFileSelect} />
      ) : (
        <div className="space-y-4">
          {/* File Preview */}
          <div className="rounded-2xl border-2 border-sercha-silverline bg-white p-4">
            <div className="flex items-center justify-between gap-3">
              <div className="flex min-w-0 flex-1 items-center gap-3">
                <div className="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-lg bg-sercha-indigo-soft">
                  <FileText className="h-6 w-6 text-sercha-indigo" />
                </div>
                <div className="min-w-0 flex-1">
                  <p className="truncate font-medium text-sercha-ink-slate">
                    {selectedFile.name}
                  </p>
                  <p className="text-sm text-sercha-fog-grey">
                    {(selectedFile.size / 1024).toFixed(1)} KB
                  </p>
                </div>
              </div>
              <button
                type="button"
                onClick={handleRemoveFile}
                disabled={isSearching}
                className={cn(
                  "flex-shrink-0 rounded-full p-2 text-sercha-fog-grey transition-colors hover:bg-sercha-mist hover:text-red-600",
                  "disabled:cursor-not-allowed disabled:opacity-50"
                )}
              >
                <X size={20} />
              </button>
            </div>
          </div>

          {/* Boost Terms */}
          <BoostTermsInput terms={boostTerms} onChange={setBoostTerms} />

          {/* Submit Button */}
          <button
            type="submit"
            disabled={isSearching}
            className={cn(
              "flex w-full items-center justify-center gap-2 rounded-2xl bg-sercha-indigo px-6 py-4 text-lg font-semibold text-white transition-all",
              "hover:bg-sercha-indigo/90 focus:outline-none focus:ring-4 focus:ring-sercha-indigo-soft",
              "disabled:cursor-not-allowed disabled:opacity-50"
            )}
          >
            {isSearching ? (
              <>
                <Loader2 size={20} className="animate-spin" />
                Searching...
              </>
            ) : (
              <>
                <Search size={20} />
                Find Similar
              </>
            )}
          </button>
        </div>
      )}
    </form>
  );
}
