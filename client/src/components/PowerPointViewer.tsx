"use client";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogTitle,
  DialogTrigger
} from "@/components/ui/dialog";
import { useState } from "react";
import DocViewer, { DocViewerRenderers } from "react-doc-viewer";
import PowerPointGenerator, {
  SlideContent
} from "./PowerPointGenerator";

export default function PowerPointViewer({ fileUrl }: { fileUrl?: string }) {
  const [open, setOpen] = useState(false);
  const [pptUrl, setPPTUrl] = useState<string | null>(fileUrl || null);

  // Example slides (this can be dynamic if needed)
  const slides: SlideContent[] = [
    {
      data: [
        {
          type: "Text",
          value: "Welcome to the PowerPoint Generator",
          options: { x: 1, y: 1, fontSize: 24, color: "FF5733" },
        },
        {
          type: "Text",
          value:
            "This is long text to test the text wrapping feature of the PowerPoint Generator,This is long text to test the text wrapping feature of the PowerPoint Generator",
          options: { x: 1, y: 1, fontSize: 18 },
        },
      ],
    },

    {
      data: [
        {
          type: "Table",
          value: [
            [
              {
                text: "Name",
                options: {
                  bold: true,
                  border: {
                    pt: 1,
                  },
                },
              },
              {
                text: "Age",
                options: {
                  bold: true,
                  border: {
                    pt: 1,
                  },
                },
              },
            ],
            [{ text: "John" }, { text: "21" }],
            [{ text: "Karim" }, { text: "25" }],
            [{ text: "Joseph" }, { text: "27" }],
            [{ text: "Mary" }, { text: "20" }],
            [{ text: "Alfred" }, { text: "22" }],
            [{ text: "John" }, { text: "21" }],
          ],
          //   options: { x: 1, y: 3, w: 5, h: 2 },
        },
      ],
    },
  ];

  return (
    <div>
      {/* Button to open dialog */}
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogTrigger asChild>
          <Button variant="outline">View PowerPoint</Button>
        </DialogTrigger>

        {/* Dialog content */}
        <DialogContent className="max-w-4xl w-full h-[80vh] flex flex-col">
          <DialogTitle />

          {pptUrl && (
            <DocViewer
              documents={[{ uri: "/Presentation.pptx" }]}
              config={{
                header: {
                  disableHeader: false,
                  disableFileName: true,
                  retainURLParams: false,
                },
              }}
              pluginRenderers={DocViewerRenderers}
              style={{ height: 500 }}
            />
          )}

          {/* PowerPoint viewer */}
          <div className="flex-1 overflow-auto">
            {/* Add the PowerPointGenerator component with sample slides for preview */}
          </div>
          <DialogFooter>
            <PowerPointGenerator
              slides={slides}
              fileName="Preview.pptx"
              onGenerate={setPPTUrl}
            />
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
