"use client";

import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
// Import the xlsx library
import * as XLSX from "xlsx";
import { PiMicrosoftExcelLogoFill } from "react-icons/pi";

export interface DownloadExcelButtonProps {
  columnLabels: string[];
  rowLabels?: string[];
  data: any[];
}
export function DownloadExcelButton({
  columnLabels,
  data,
}: DownloadExcelButtonProps) {
  const downloadExcel = () => {
    console.log("Download Excel triggered");

    // Create a worksheet from the data
    const worksheetData = [
      columnLabels, // Column headers as the first row
      ...data.map((row) => row.map((cell: any) => cell.value)), // Data rows (accessing 'value' of each cell)
    ];

    // Create a worksheet object
    const ws = XLSX.utils.aoa_to_sheet(worksheetData);

    // Create a new workbook
    const wb = XLSX.utils.book_new();
    XLSX.utils.book_append_sheet(wb, ws, "Sheet1");

    // Generate a file name
    const fileName = "spreadsheet.xlsx";

    // Save the file to trigger the download
    XLSX.writeFile(wb, fileName);
  };

  return (
    <TooltipProvider delayDuration={0}>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            size="sm"
            className="bg-green-100"
            variant="outline"
            onClick={downloadExcel}
          >
            <PiMicrosoftExcelLogoFill
              className="text-green-800"
              size={16}
              strokeWidth={2}
            />
          </Button>
        </TooltipTrigger>
        <TooltipContent className="px-2 py-1 text-xs">
          Save excel file
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}
