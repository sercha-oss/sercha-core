"use client";

import { useState, KeyboardEvent } from "react";
import { X, Plus } from "lucide-react";
import { cn } from "@/lib/utils";

interface BoostTermsInputProps {
  terms: string[];
  onChange: (terms: string[]) => void;
  placeholder?: string;
}

export function BoostTermsInput({
  terms,
  onChange,
  placeholder = "Add keyword to boost...",
}: BoostTermsInputProps) {
  const [inputValue, setInputValue] = useState("");

  const handleAddTerm = () => {
    const trimmed = inputValue.trim();
    if (trimmed && !terms.includes(trimmed)) {
      onChange([...terms, trimmed]);
      setInputValue("");
    }
  };

  const handleRemoveTerm = (term: string) => {
    onChange(terms.filter((t) => t !== term));
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      handleAddTerm();
    }
  };

  return (
    <div className="w-full">
      <label className="mb-2 block text-sm font-medium text-sercha-ink-slate">
        Boost Keywords (optional)
      </label>
      <p className="mb-1 text-xs text-sercha-fog-grey">
        Prioritize results containing specific terms
      </p>
      <p className="mb-3 text-xs text-sercha-silverline leading-relaxed">
        Boost terms emphasize specific keywords when searching by a long document. Use them when
        the document covers multiple topics but you only care about matches on specific ones.
      </p>

      {/* Tags Display */}
      {terms.length > 0 && (
        <div className="mb-3 flex flex-wrap gap-2">
          {terms.map((term) => (
            <div
              key={term}
              className="flex items-center gap-1.5 rounded-full bg-sercha-indigo px-3 py-1.5 text-sm font-medium text-white"
            >
              <span>{term}</span>
              <button
                type="button"
                onClick={() => handleRemoveTerm(term)}
                className="rounded-full hover:bg-white/20 focus:outline-none focus:ring-2 focus:ring-white/50"
              >
                <X size={14} />
              </button>
            </div>
          ))}
        </div>
      )}

      {/* Input */}
      <div className="flex gap-2">
        <input
          type="text"
          value={inputValue}
          onChange={(e) => setInputValue(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          className={cn(
            "flex-1 rounded-lg border border-sercha-silverline bg-white px-3 py-2 text-sm text-sercha-ink-slate",
            "placeholder:text-sercha-fog-grey",
            "focus:border-sercha-indigo focus:outline-none focus:ring-2 focus:ring-sercha-indigo/20"
          )}
        />
        <button
          type="button"
          onClick={handleAddTerm}
          disabled={!inputValue.trim()}
          className={cn(
            "flex items-center gap-1.5 rounded-lg px-4 py-2 text-sm font-medium transition-all",
            "bg-sercha-indigo text-white hover:bg-sercha-indigo/90",
            "disabled:cursor-not-allowed disabled:opacity-50"
          )}
        >
          <Plus size={16} />
          Add
        </button>
      </div>
    </div>
  );
}
