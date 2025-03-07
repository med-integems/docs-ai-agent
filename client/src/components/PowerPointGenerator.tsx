"use client";

import React from "react";
import PptxGenJS from "pptxgenjs";
import { Button } from "@/components/ui/button";
import { PiMicrosoftPowerpointLogoFill } from "react-icons/pi";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

type ContentType = "Image" | "Shape" | "Table" | "Text" | "Chart";

type TableRows = PptxGenJS.TableRow[];

type SlideTheme = {
  background?: string;
  fontSize?: number;
  color?: string;
};

type DataValue =
  | string
  | TableRows
  | PptxGenJS.SHAPE_NAME
  | PptxGenJS.CHART_NAME;

type DataOption =
  | PptxGenJS.TextPropsOptions
  | PptxGenJS.ImageProps
  | PptxGenJS.TableProps
  | PptxGenJS.ShapeProps
  | PptxGenJS.IChartOpts;

export type SlideData = {
  type: ContentType;
  value: DataValue;
  options?: DataOption;
  chatData?: { name: string; labels: any[]; values: any[] }[]; // for chart
};

export type SlideContent = {
  data: SlideData[];
};

type PowerPointGeneratorProps = {
  slides: SlideContent[];
  theme?: SlideTheme;
  fileName?: string;
  onGenerate?: (fileUrl: string) => void;
};

const PowerPointGenerator: React.FC<PowerPointGeneratorProps> = ({
  slides,
  fileName = "Presentation.pptx",
}) => {
  const generatePresentation = () => {
    const pptx = new PptxGenJS();
    slides.forEach((slideContent) => {
      const slide = pptx.addSlide();
      slideContent.data.forEach((data) => {
        switch (data.type) {
          case "Image":
            slide.addImage({
              path: "/powerpoint.jpg",
              ...data.options,
            } as PptxGenJS.ImageProps);
            break;
          case "Shape":
            slide.addShape(
              data.value as PptxGenJS.SHAPE_NAME,

              data.options as PptxGenJS.ShapeProps,
            );
            break;
          case "Table":
            slide.addTable(
              data.value as TableRows,
              data.options as PptxGenJS.TableProps,
            );
            break;
          case "Text":
            slide.addText(
              data.value as string,
              data.options as PptxGenJS.TextPropsOptions,
            );
            break;
          case "Chart":
            const chatData = data?.chatData?.map((data) => ({
              name: data?.name,
              values: data?.values || [],
              labels: data?.labels || [],
            }));

            console.log({ chatData });

            if (chatData) {
              slide.addChart(
                data.value as PptxGenJS.CHART_NAME,
                chatData,
                data.options as PptxGenJS.IChartOpts,
              );
            }
        }
      });

      // Generate the file
    });

    pptx.writeFile({ fileName });
  };

  //   const previewPresentation = () => {
  //     const pptx = new PptxGenJS();
  //     slides.forEach((slideContent) => {
  //       const slide = pptx.addSlide();
  //       slideContent.data.forEach((data) => {
  //         switch (data.type) {
  //           case "Image":
  //             slide.addImage({
  //               path: data?.value,
  //               data: data.value,
  //               ...data.options,
  //             } as PptxGenJS.ImageProps);
  //             break;
  //           case "Shape":
  //             slide.addShape(
  //               data.value as PptxGenJS.SHAPE_NAME,
  //               data.options as PptxGenJS.ShapeProps,
  //             );
  //             break;
  //           case "Table":
  //             slide.addTable(
  //               data.value as TableRows,
  //               data.options as PptxGenJS.TableProps,
  //             );
  //             break;
  //           case "Text":
  //             slide.addText(
  //               data.value as string,
  //               data.options as PptxGenJS.TextPropsOptions,
  //             );
  //             break;
  //           case "chart":
  //             slide.addChart(
  //               data.value as PptxGenJS.CHART_NAME,
  //               data.chatData as any[],
  //               data.options as PptxGenJS.IChartOpts,
  //             );
  //             break;
  //         }
  //       });

  //       // Generate the pptx file as a Blob
  //       pptx.write().then((file) => {
  //         // Now that we have the Blob, create an Object URL for it
  //         const pptxFile = new File([file], fileName, { type: "application/vnd.openxmlformats-officedocument.presentationml.presentation"});
  //         let url = URL.createObjectURL(pptxFile);
  //         console.log("Generated PowerPoint Blob URL:", url);
  //         console.log("File:", { pptxFile});

  //         // Pass this URL to react-doc-viewer for preview
  //         setPPTUrl(url); // Set the Blob URL in the state
  //         onGenerate?.(url); // Optionally call onGenerate with the URL
  //       });
  //     });
  //   };

  // const previewPresentation = () => {
  //     const pptx = new PptxGenJS();
  //     slides.forEach((slideContent) => {
  //       const slide = pptx.addSlide();
  //       slideContent.data.forEach((data) => {
  //         switch (data.type) {
  //           case "Image":
  //             slide.addImage({ path: data?.value, data: data.value, ...data.options } as PptxGenJS.ImageProps);
  //             break;
  //           case "Shape":
  //             slide.addShape(data.value as PptxGenJS.SHAPE_NAME, data.options as PptxGenJS.ShapeProps);
  //             break;
  //           case "Table":
  //             slide.addTable(data.value as TableRows, data.options as PptxGenJS.TableProps);
  //             break;
  //           case "Text":
  //             slide.addText(data.value as string, data.options as PptxGenJS.TextPropsOptions);
  //             break;
  //           case "chart":
  //             slide.addChart(data.value as PptxGenJS.CHART_NAME, data.chatData as any[], data.options as PptxGenJS.IChartOpts);
  //             break;
  //         }
  //       });

  //       pptx.write({ outputType: "blob", compression: false }).then((file) => {
  //         // Convert Blob to Data URL
  //         const reader = new FileReader();
  //         reader.onloadend = () => {
  //           const dataUrl = reader.result as string;
  //           console.log("Generated Data URL:", dataUrl);
  //           setPPTUrl(dataUrl);  // Update the URL with the data URL
  //           onGenerate?.(dataUrl);  // Pass the data URL to the parent
  //         };
  //         reader.readAsDataURL(file as Blob);
  //       });
  //     });
  //   };

  return (
    <TooltipProvider delayDuration={0}>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            size="sm"
            variant="outline"
            onClick={generatePresentation}
            className="bg-red-100"
          >
            <PiMicrosoftPowerpointLogoFill className="text-red-800" />
          </Button>
        </TooltipTrigger>
        <TooltipContent className="px-2 py-1 text-xs">
          Save powerpoint file
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
};

export default PowerPointGenerator;
