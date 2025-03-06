"use client";

import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { Check, Copy } from "lucide-react";
import { useState } from "react";

interface CopyComponentProps {
  onCopy: () => void;
}

export default function CopyComponent({ onCopy }: CopyComponentProps) {
  const [copied, setCopied] = useState<boolean>(false);

  const handleCopy = async () => {
    try {
      onCopy();
      // await navigator.clipboard.writeText("string to copy");
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch (err) {
      console.error("Failed to copy text: ", err);
    }
  };

  return (
    <TooltipProvider delayDuration={0}>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant="outline"
            size="sm"
            className="relative disabled:opacity-100 bg-primary/10"
            onClick={handleCopy}
            aria-label={copied ? "Copied" : "Copy to clipboard"}
            disabled={copied}
          >
            {copied && (
              <div
                className={cn(
                  "transition-all flex flex-row gap-1",
                  copied ? "scale-100 opacity-100" : "scale-0 opacity-0",
                )}
              >
                <Check
                  className="stroke-emerald-500"
                  size={16}
                  strokeWidth={2}
                  aria-hidden="true"
                />
              </div>
            )}
            {!copied && (
              <>
                <Copy
                  size={16}
                  strokeWidth={2}
                  aria-hidden="true"
                  className="text-primary"
                />
              </>
            )}
          </Button>
        </TooltipTrigger>
        <TooltipContent className="px-2 py-1 text-xs">
          Click to copy
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}
