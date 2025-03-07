"use client";

import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Table } from "lucide-react";
import Spreadsheet from "react-spreadsheet";
import { Button } from "./ui/button";

export interface SheetPreviewModalProps {
  data: any[];
  columnLabels: string[];
}

export function SheetPreviewModal({
  data,
  columnLabels,
}: SheetPreviewModalProps) {
  return (
    <Dialog>
      <DialogTrigger asChild>
        {/* <TooltipProvider delayDuration={0}>
          <Tooltip>
            <TooltipTrigger asChild> */}
        <Button className="bg-green-100" variant="outline" size="sm">
          <Table className="text-green-800" />
        </Button>
        {/* </TooltipTrigger>
            <TooltipContent className="px-2 py-1 text-xs">
              View excel data
            </TooltipContent>
          </Tooltip>
        </TooltipProvider> */}
      </DialogTrigger>
      <DialogContent className="max-w-fit w-full h-[80vh] overflow-auto flex flex-col">
        <DialogHeader>
          <DialogTitle>Sheet View</DialogTitle>
        </DialogHeader>
        <Spreadsheet
          className="w-full"
          data={data}
          columnLabels={columnLabels}
        />
      </DialogContent>
    </Dialog>
  );
}
