"use client";

import * as React from "react";

import { Progress } from "@/components/ui/progress";

interface ProgressProps {
  value: number;
}

export function AnimatedProgress({ value }: ProgressProps) {
  const [progress, setProgress] = React.useState(1);

  React.useEffect(() => {
    const timer = setTimeout(() => setProgress(value), 500);
    return () => clearTimeout(timer);
  }, [value]);

  return <Progress value={progress} className="w-full" />;
}
