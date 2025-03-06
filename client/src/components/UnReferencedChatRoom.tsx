"use client";

import { Button } from "@/components/ui/button";
import { useToast } from "@/hooks/use-toast";
import { cn } from "@/lib/utils";
import {
  File,
  FileIcon,
  FilePlus,
  Loader,
  LoaderPinwheel,
  Plus,
  PlusCircle,
  Printer,
  RefreshCw,
  Send,
  Upload,
  X,
} from "lucide-react";
import { useEffect, useOptimistic, useRef, useState } from "react";
import Markdown from "react-markdown";
import CopyComponent from "./CopyButton";
import PowerPointGenerator, { SlideContent } from "./PowerPointGenerator";
import remarkGfm from "remark-gfm";
import rehypeKatex from "rehype-katex";
import remarkMath from "remark-math";
import "katex/dist/katex.min.css";
import { SheetPreviewModal } from "./SheetPreviewModal";
import { DownloadExcelButton } from "./DownloadExcelButton";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { v4 as uuidv4 } from "uuid";
import { ExcelPreviewProps } from "./ExcelPreviewModal";
import Image from "next/image";

type Message = {
  role: string;
  content: string;
  createdAt: string;
  contentType: string;
};

export default function UnReferencedChatRoom() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState<string>("");
  const [file, setFile] = useState<File | null>(null);
  const [loadingAi, setLoadingAi] = useState<boolean>(false);
  const [errorMsg, setErrorMsg] = useState<string>("");
  const [fileData, setFileData] = useState<{
    uri: string;
    fileType: string;
  } | null>(null);
  const { toast } = useToast();
  const messageContainerRef = useRef<HTMLDivElement>(null);

  // Retrieve or generate sessionId
  useEffect(() => {
    let sessionId = localStorage.getItem("sessionId");
    console.log({ Session: sessionId });
    if (!sessionId) {
      sessionId = uuidv4();
      localStorage.setItem("sessionId", sessionId);
    }
    fetchMessages(sessionId);
  }, []);

  const handleFileUpload = (event: React.ChangeEvent<HTMLInputElement>) => {
    const uploadedFile = event.target.files?.[0];
    if (!uploadedFile) return;
    setFile(uploadedFile);

    const fileType = uploadedFile.type;
    const fileExtension = uploadedFile.name.split(".").pop()?.toLowerCase();

    if (fileExtension === "docx") {
      toast({ description: ".docx files are not supported." });
      return;
    }

    const fileUrl = URL.createObjectURL(uploadedFile);
    setFileData({ uri: fileUrl, fileType });
  };

  // Fetch messages for the current session
  const fetchMessages = async (sessionId: string | null) => {
    if (!sessionId) return;
    try {
      const response = await fetch(
        `http://127.0.0.1:5000/messages/sessions/${sessionId}`,
      );
      if (!response.ok) throw new Error("Failed to fetch messages");

      const data: Message[] = await response.json();
      setMessages(data);
    } catch (error) {
      console.error("Error fetching messages:", error);
    }
  };

  // Function to start a new session
  const handleNewSession = () => {
    const newSessionId = uuidv4();
    setErrorMsg("");
    localStorage.setItem("sessionId", newSessionId);
    setMessages([]); // Clear chat history
    toast({
      description: <p className="text-green-500">New chat session started.</p>,
    });
  };

  // Send a new message to the AI handler
  const sendMessage = async () => {
    setErrorMsg("");
    if (!input.trim() && !file) {
      toast({
        description: (
          <p className="text-orange-500">
            Please enter a message and optionally, select a file.
          </p>
        ),
      });
      return;
    }

    if (file && !input.trim) {
      toast({
        description: <p className="text-orange-500">Please enter a text message.</p>,
      });
      return;
    }

    setLoadingAi(true);
    const sessionId = localStorage.getItem("sessionId") || uuidv4();
    if (file) {
      const newMessage: Message = {
        role: "user",
        content: input,
        createdAt: new Date().toISOString(),
        contentType: "file",
      };

      setMessages((prev) => [...(prev || []), newMessage]);
    }
    const newMessage: Message = {
      role: "user",
      content: input,
      createdAt: new Date().toISOString(),
      contentType: "text",
    };

    setMessages((prev) => [...(prev || []), newMessage]);

    try {
      const formData = new FormData();
      formData.append("text", input);
      formData.append("sessionId", sessionId);
      if (file) {
        formData.append("file", file);
      }

      const response = await fetch("http://127.0.0.1:5000/ai-chat", {
        method: "POST",
        body: formData,
      });
      const reply: Message = await response.json();
      console.log({ reply });
      if (!response.ok) {
        throw new Error("Failed to send message.");
      }

      setInput("");
      setFile(null);

      setMessages((prev) => [...prev, reply]);
    } catch (error) {
      console.error(error);
      setMessages((prev) => prev.slice(0, -1)); // Remove unsent message
      setErrorMsg("Service unavailable. Please try again later.");
    } finally {
      setLoadingAi(false);
    }
  };

  useEffect(() => {
    if (messageContainerRef.current) {
      messageContainerRef.current.scrollTop =
        messageContainerRef.current.scrollHeight;
    }
  }, [messages]);

  return (
    <div className="relative flex flex-col w-full mx-auto md:w-[65vw]">
      {/* Header with New Session Button */}
      <div className="p-4 flex flex-row items-center justify-between">
        <h4>Chat Session</h4>
        <Button
          variant="outline"
          size="sm"
          onClick={handleNewSession}
          className="flex items-center outline-primary text-primary border-primary"
        >
          <Plus size={16} />
          New Session
        </Button>
      </div>

      {/* Chat Messages */}
      <div
        ref={messageContainerRef}
        className="overflow-y-auto bg-neutral-100 h-[72vh] p-4 w-full rounded-lg custom-scrollbar"
      >
     
        {
          messages?.map((message, index) => {
           let file = messages[messages.lastIndexOf(message) - 1]?.contentType == "file"
           return  (
            <MessageItem key={index} message={message} file={file} />
          )
          })
        }

        {errorMsg && (
          <div className="bg-orange-100 text-orange-600 text-center p-2 rounded-lg">
            <Markdown remarkPlugins={[remarkGfm, remarkMath]}>
              {errorMsg}
            </Markdown>
            <Button
              variant="outline"
              className="my-2 border-orange-500 hover:bg-orange-200"
              onClick={sendMessage}
            >
              Try again
            </Button>
          </div>
        )}

        {loadingAi && (
          <LoaderPinwheel className="animate-spin w-8 h-8 text-primary my-6 mx-2" />
        )}
      </div>

      {/* Input Area */}
         <div className="flex m-5 mt-2 items-start p-4 space-x-2 bg-white shadow-lg rounded-3xl border-2 border-transparent focus-within:border-primary">
           {file && fileData && fileData.fileType.startsWith("image") && (
                 <div className="relative w-8 h-8 rounded overflow-hidden z-50">
                   <Button
                     variant="ghost"
                     onClick={(e) => {
                       e.stopPropagation();
                       setFile(null);
                     }}
                     className="absolute inset-0 z-20 hover:bg-black/20 hover:text-white"
                   >
                     <X />
                   </Button>
                   <Image
                     src={fileData.uri}
                     fill
                     alt="Uploaded file preview"
                     className="absolute object-cover"
                   />
                 </div>
               )}
     
               {file && fileData && !fileData.fileType.startsWith("image") && (
                 <div className="relative w-8 h-8 rounded overflow-hidden z-50">
                   <Button
                     variant="ghost"
                     onClick={() => setFile(null)}
                     className="absolute inset-0 z-20 hover:bg-black/20 hover:text-white"
                   >
                     <X />
                   </Button>
                   <FileIcon strokeWidth={1} className="absolute inset-0 w-8 h-8" />
                 </div>
               )}
             <textarea
               value={input}
               onChange={(e) => setInput(e.target.value)}
               placeholder="Type your message..."
               className="relative flex-1 border-none outline-none resize-none p-2 rounded-md bg-white mb-6"
             />
            
             <input
               type="file"
               onChange={handleFileUpload}
               className="hidden"
               id="file-upload"
             />
             <label
               htmlFor="file-upload"
               className="cursor-pointer absolute bottom-8 left-8"
             >
          
               {file === null && (
                 <PlusCircle size={30} strokeWidth={1} className="text-primary" />
               )}
             </label>
       
         
             <Button size="icon" onClick={sendMessage} disabled={loadingAi}>
               <Send />
             </Button>
     
           </div>
    </div>
  );
}

// Sub-component for individual message
function MessageItem({ message,file }: { message: Message,file:boolean }) {
  const markdownRef = useRef<HTMLDivElement>(null); // Reference to capture rendered content

  const [slides, setSlides] = useState<SlideContent[]>([]);
  const [execlData, setExcelData] = useState<ExcelPreviewProps | null>(null);
  const [textMessage, setTextMessage] = useState<Message | null>(null);
  const [formatError, setFormatError] = useState<boolean>(false);

  useEffect(() => {
    try {
      if (message.role !== "user" && message.content.includes("&&json")) {
        let splitData = message.content.split("&&json");
        let replyJson = splitData?.[1];
        if (replyJson) {
          // let lastIndex = message.content.lastIndexOf("`")
          // let replyEnd = message.content.substring(lastIndex + 1,message.content.length);

          let start = replyJson.indexOf("{") - 1;
          let end = replyJson.lastIndexOf("}") + 1;
          let jsonString = replyJson.substring(start, end);
          let textReply =
            message.content.split("&&json")[0] + replyJson.substring(end + 1);

          let jsonReply = JSON.parse(jsonString || "{}");
          message.content = textReply;

          console.log("Data", jsonReply);

          setSlides(jsonReply?.slides);
          setExcelData(jsonReply?.excel);
        }
      }
      setTextMessage(message);
    } catch (err) {
      setFormatError(true);
    }
  }, [message]);

  if (message === null || formatError === true) {
    return null;
  }
  const handleCopy = () => {
    navigator.clipboard.writeText(message.content);
  };

  const handlePrint = () => {
    if (markdownRef.current) {
      const printWindow = window.open("", "_blank", "width=600,height=400");
      if (printWindow) {
        printWindow.document.open();
        printWindow.document.write(`
          <html>
            <head>
              <title>Print Message</title>
              <style>
                body { font-family: Arial, sans-serif; padding: 20px; }
              </style>
            </head>
            <body>
              ${markdownRef.current.innerHTML} <!-- Inject rendered content -->
            </body>
          </html>
        `);
        printWindow.document.close();
        printWindow.print();
      }
    }
  };

  if (message.contentType === "file") {
    return null
  }

  return (
    <div
      className={cn(
        "flex mb-6",
        message.role === "user" ? "justify-end" : "justify-start",
      )}
    >
      <div
        className={cn(
          "rounded-lg shadow-lg",
          message.role === "user"
            ? "max-w-2xl bg-primary text-white py-1 px-3"
            : "w-full max-w-full bg-white text-black p-4",
        )}
      >
        {/* Capture rendered content */}
        {
          file &&
          <div className={cn("w-fit p-2 flex flex-row items-center justify-center bg-white/10 rounded-lg text-sm")}>
          <FileIcon strokeWidth={3} size={15}/><span className="mx-1">file</span>
        </div>
        }
        <div ref={markdownRef}>
          <Markdown
            remarkPlugins={[remarkGfm, remarkMath]}
            rehypePlugins={[rehypeKatex]}
            className="prose max-w-none w-full text-[15px] md:text-[16px] leading-relaxed space-y-2"
          >
            {textMessage?.content}
          </Markdown>
        </div>
        <p
          className={cn(
            "text-xs mt-1",
            message.role === "user" ? "text-white/50" : "text-black/50",
          )}
        >
          {new Date(message.createdAt).toLocaleTimeString()}
        </p>
        {message.role !== "user" && (
          <div className="flex gap-2 mt-3">
            <CopyComponent onCopy={handleCopy} />
            <TooltipProvider delayDuration={0}>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    size="sm"
                    className="bg-blue-100"
                    variant="outline"
                    onClick={handlePrint}
                  >
                    <Printer className="text-blue-800" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent className="px-2 py-1 text-xs">
                  Print as pdf
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
            {!!slides?.length && <PowerPointGenerator slides={slides} />}
            {execlData && <DownloadExcelButton {...execlData} />}
            {/* {execlData && <ExcelPreviewModal {...execlData} />} */}
            {execlData && <SheetPreviewModal {...execlData} />}
          </div>
        )}
      </div>
    </div>
  );
}
