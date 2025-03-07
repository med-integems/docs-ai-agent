"use client";

import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  ColumnDirective,
  ColumnsDirective,
  SpreadsheetComponent as ExcelSP,
  RangeDirective,
  RangesDirective,
  SheetDirective,
  SheetsDirective,
} from "@syncfusion/ej2-react-spreadsheet";
import { DialogTrigger } from "@radix-ui/react-dialog";
import { Button } from "./ui/button";
import { Table } from "lucide-react";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

export interface ExcelPreviewProps {
  columnLabels: string[];
  rowLabels: string[];
  data: any[];
}

export function ExcelPreviewModal({
  data,
  columnLabels,
  rowLabels,
}: ExcelPreviewProps) {
  return (
    <TooltipProvider delayDuration={0}>
      <Tooltip>
        <TooltipTrigger asChild>
          <Dialog>
            <DialogTrigger asChild>
              <Button variant="default" size="sm">
                <Table />
              </Button>
            </DialogTrigger>
            <DialogContent className="max-w-fit w-full h-[80vh] flex flex-col">
              <DialogHeader>
                <DialogTitle>Excel Preview</DialogTitle>
              </DialogHeader>
              <Excel
                data={data}
                columnLabels={columnLabels}
                rowLabels={rowLabels}
              />
            </DialogContent>
          </Dialog>
        </TooltipTrigger>
        <TooltipContent className="px-2 py-1 text-xs">
          View excel data
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

function Excel({ data, columnLabels, }: ExcelPreviewProps) {
  return (
    <ExcelSP>
      <SheetsDirective>
        <SheetDirective>
          <RangesDirective>
            <RangeDirective dataSource={data} fieldsOrder={columnLabels} />
          </RangesDirective>
          <ColumnsDirective>
            <ColumnDirective width={100} />
            <ColumnDirective width={110} />
            <ColumnDirective width={100} />
            <ColumnDirective width={180} />
            <ColumnDirective width={130} />
            <ColumnDirective width={130} />
          </ColumnsDirective>
        </SheetDirective>
      </SheetsDirective>
    </ExcelSP>
  );
}
