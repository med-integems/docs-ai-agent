"use client";

import { useState } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Slider } from "@/components/ui/slider";
import { Select, SelectItem } from "@/components/ui/select";
import { cn } from "@/lib/utils";

type SliceContent = {
  title: string;
  content: string;
  image?: string;
};

const fonts = [
  { name: "Sans-serif", value: "sans-serif" },
  { name: "Serif", value: "serif" },
  { name: "Monospace", value: "monospace" },
  { name: "Cursive", value: "cursive" },
  { name: "Fantasy", value: "fantasy" },
];

export default function FontControl({
  slicesContent,
}: {
  slicesContent: SliceContent[];
}) {
  const [fontSize, setFontSize] = useState(16);
  const [selectedFont, setSelectedFont] = useState("sans-serif");

  return (
    <div className="max-w-3xl mx-auto p-4">
      {/* Controls */}
      <div className="flex gap-4 items-center justify-between bg-gray-100 p-4 rounded-lg shadow">
        <div className="flex flex-col w-1/2">
          <label className="text-sm font-semibold">Font Size</label>
          <Slider
            defaultValue={[fontSize]}
            min={12}
            max={40}
            step={1}
            onValueChange={(value) => setFontSize(value[0])}
          />
          <span className="text-xs mt-1">{fontSize}px</span>
        </div>

        <div className="flex flex-col w-1/2">
          <label className="text-sm font-semibold">Font Style</label>
          <Select onValueChange={setSelectedFont} defaultValue="sans-serif">
            {fonts.map((font) => (
              <SelectItem key={font.value} value={font.value}>
                {font.name}
              </SelectItem>
            ))}
          </Select>
        </div>
      </div>

      {/* Display Content */}
      <div className="grid gap-4 mt-6">
        {slicesContent.map((slice, index) => (
          <Card key={index} className="p-4">
            <CardContent>
              {slice.image && (
                <img
                  src={slice.image}
                  alt={slice.title}
                  className="w-full h-48 object-cover rounded-md mb-2"
                />
              )}
              <h2
                className="font-bold text-lg"
                style={{ fontFamily: selectedFont, fontSize: `${fontSize}px` }}
              >
                {slice.title}
              </h2>
              <p
                className="text-gray-600"
                style={{
                  fontFamily: selectedFont,
                  fontSize: `${fontSize - 2}px`,
                }}
              >
                {slice.content}
              </p>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
}
