"use client";

import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTrigger,
} from "@/components/ui/dialog";
import React, { useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { useToast } from "@/hooks/use-toast";
import { DialogTitle } from "@radix-ui/react-dialog";
import { Loader2Icon, Plus, Upload } from "lucide-react";
import { useRouter } from "next/navigation";
import { useCurrentUser } from "@/hooks/use-current-user";

// primar-color #7C3AED

export default function AddDocumentModal() {
  const [documentType, setDocumentType] = useState<"docx" | "pdf">("pdf");
  const { user } = useCurrentUser();
  const [formData, setFormData] = useState({
    title: "",
    url: "",
    file: null as File | null,
  });

  const [loading, setLoading] = useState<boolean>(false);
  const router = useRouter();
  const { toast } = useToast();

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    setFormData((prev) => ({ ...prev, [name]: value }));
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (files && files[0]) {
      setFormData((prev) => ({ ...prev, file: files[0] }));
    }
  };

  const handleSubmit = async () => {
    setLoading(true);

    if (!formData.file) {
      toast({
        description: (
          <p className="text-red-500">{`A ${documentType} file is required`}</p>
        ),
      });
      setLoading(false);
      return;
    }

    try {
      const endpoint = "http://127.0.0.1:5000/documents";

      const payload = new FormData();

      if (user?.userId && formData.file) {
        payload.append("title", formData.title || formData.file.name);
        payload.append("userId", user?.userId);
        payload.append("file", formData.file);
        payload.append("fileType", documentType);
      } else {
        toast({
          description: (
            <p className="text-red-500">
              Incomplete or invalid data. Make sure you enter the correct
              values.
            </p>
          ),
        });
        setLoading(false);
        return;
      }

      const response = await fetch(endpoint, {
        method: "POST",
        body: payload,
      });

      const data = await response.json();

      if (!response.ok) {
        console.log("Response", data);
        throw new Error("Couldn't upload document.");
      }

      toast({
        description: (
          <p className="text-green-500">Document added successfully!!</p>
        ),
      });

      setFormData({ title: "", url: "", file: null });
      router.refresh();
    } catch (error) {
      console.error(error);
      toast({
        description: <p className="text-red-500">Couldn't add document.</p>,
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button variant="default" size="sm">
          <Plus />
          Upload
        </Button>
      </DialogTrigger>
      <DialogContent className="overflow-x-hidden">
        <DialogHeader>
          <DialogTitle className="text-lg font-semibold">
            Add Document
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          {/* Video Type Selection */}
          <RadioGroup
            value={documentType}
            onValueChange={(value) => setDocumentType(value as "docx" | "pdf")}
          >
            <div className="flex items-center space-x-2">
              <RadioGroupItem disabled={true} value="docx" id="docx" />
              <Label htmlFor="docx">
                Microsoft Word (docx) - Unavailable for now
              </Label>
            </div>
            <div className="flex items-center space-x-2">
              <RadioGroupItem value="pdf" id="pdf" />
              <Label htmlFor="pdf">Portable Document Format (pdf)</Label>
            </div>
          </RadioGroup>

          {/* Title Field */}
          <div>
            <Label htmlFor="title" className="mb-2">
              Title (name)
            </Label>
            <Input
              id="title"
              name="title"
              placeholder="Title of the document"
              value={formData.title}
              onChange={handleInputChange}
              className="my-2"
            />
          </div>
          {/* File Upload Field */}
          <div>
            <Label htmlFor="file" className="mb-2">
              Upload File
            </Label>
            <div className="flex items-center space-x-4 my-2">
              <Button variant="outline" asChild>
                <label className="cursor-pointer">
                  <Upload className="mr-2 h-5 w-5" />
                  Select File
                  <input
                    id="file"
                    name="file"
                    type="file"
                    accept={
                      documentType === "pdf"
                        ? "application/pdf"
                        : ".doc,.docx,application/msword,application/vnd.openxmlformats-officedocument.wordprocessingml.document"
                    }
                    className="hidden"
                    onChange={handleFileChange}
                  />
                </label>
              </Button>
              {formData.file && (
                <p className="line-clamp-1 text-ellipsis">
                  {formData.file.name.substring(0, 40)}
                </p>
              )}
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button size="sm" onClick={handleSubmit} disabled={loading}>
            {loading && <Loader2Icon className="animate-spin" />}
            Continue
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
