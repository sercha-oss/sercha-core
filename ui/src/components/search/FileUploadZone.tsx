"use client";

import { useRef, useState } from "react";
import { Upload, FileText } from "lucide-react";
import { cn } from "@/lib/utils";

interface FileUploadZoneProps {
  onFileSelect: (file: File) => void;
  accept?: string;
  maxSizeMB?: number;
}

const ACCEPTED_TYPES = [
  "application/pdf",
  "text/markdown",
  "text/html",
  "text/plain",
  "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  "application/vnd.openxmlformats-officedocument.presentationml.presentation",
  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
];

const ACCEPTED_EXTENSIONS = ".pdf,.md,.html,.htm,.txt,.docx,.pptx,.xlsx";

export function FileUploadZone({
  onFileSelect,
  accept = ACCEPTED_EXTENSIONS,
  maxSizeMB = 10,
}: FileUploadZoneProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [isDragging, setIsDragging] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const validateFile = (file: File): string | null => {
    // Check file size
    const maxBytes = maxSizeMB * 1024 * 1024;
    if (file.size > maxBytes) {
      return `File size must be less than ${maxSizeMB}MB`;
    }

    // Check file type
    const isValidType = ACCEPTED_TYPES.some((type) => {
      if (type.endsWith("/*")) {
        return file.type.startsWith(type.replace("/*", ""));
      }
      return file.type === type;
    });

    if (!isValidType) {
      return "File type not supported. Please upload PDF, Word (DOCX), PowerPoint (PPTX), Excel (XLSX), Markdown, HTML, or plain text files.";
    }

    return null;
  };

  const handleFile = (file: File) => {
    setError(null);
    const validationError = validateFile(file);
    if (validationError) {
      setError(validationError);
      return;
    }
    onFileSelect(file);
  };

  const handleDrop = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setIsDragging(false);

    const file = e.dataTransfer.files[0];
    if (file) {
      handleFile(file);
    }
  };

  const handleDragOver = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setIsDragging(true);
  };

  const handleDragLeave = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setIsDragging(false);
  };

  const handleClick = () => {
    inputRef.current?.click();
  };

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      handleFile(file);
    }
  };

  return (
    <div className="w-full">
      <div
        onClick={handleClick}
        onDrop={handleDrop}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        className={cn(
          "flex cursor-pointer flex-col items-center justify-center rounded-2xl border-2 border-dashed px-6 py-8 transition-all",
          isDragging
            ? "border-sercha-indigo bg-sercha-indigo-soft"
            : error
              ? "border-red-300 bg-red-50"
              : "border-sercha-silverline bg-white hover:border-sercha-indigo hover:bg-sercha-mist/50"
        )}
      >
        <input
          ref={inputRef}
          type="file"
          accept={accept}
          onChange={handleChange}
          className="hidden"
        />

        {error ? (
          <FileText className="mb-4 h-12 w-12 text-red-400" />
        ) : (
          <Upload className="mb-4 h-12 w-12 text-sercha-fog-grey" />
        )}

        <h3 className="mb-2 text-lg font-semibold text-sercha-ink-slate">
          {isDragging ? "Drop file here" : "Upload a document"}
        </h3>

        <p className="mb-1 text-sm text-sercha-fog-grey">
          Drag and drop or click to browse
        </p>

        <p className="text-xs text-sercha-silverline">
          Accepts PDF, Word, PowerPoint, Excel, Markdown, HTML, and plain text (max {maxSizeMB}MB)
        </p>

        {error && (
          <p className="mt-3 text-sm text-red-600">{error}</p>
        )}
      </div>
    </div>
  );
}
