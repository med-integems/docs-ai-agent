"use client";

import { Button } from "@/components/ui/button";
import { useToast } from "@/hooks/use-toast";
import { cn } from "@/lib/utils";
import {
  FileIcon,
  Loader,
  LoaderPinwheel,
  PlusCircle,
  Printer,
  Send,
  X,
} from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import Markdown from "react-markdown";
import CopyComponent from "./CopyButton";
import type { Document } from "./ChatRoomSideBar";
import PowerPointGenerator, { SlideContent } from "./PowerPointGenerator";
import remarkGfm from "remark-gfm";
import rehypeKatex from "rehype-katex";
import remarkMath from "remark-math";
import "katex/dist/katex.min.css";
import { ExcelPreviewModal, ExcelPreviewProps } from "./ExcelPreviewModal";
import { SheetPreviewModal } from "./SheetPreviewModal";
import { DownloadExcelButton } from "./DownloadExcelButton";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import UnReferencedChatRoom from "./UnReferencedChatRoom";
import Image from "next/image";

type Message = {
  role: string;
  content: string;
  createdAt: string;
  contentType: string;
};

export default function ChatRoom() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState<string>("");
  const [document, setDocument] = useState<Document | null>(null);
  const [loadingAi, setLoadingAi] = useState<boolean>(false);
  const [loadingMessages, setLoadingMessages] = useState<boolean>(false);
  const searchParams = useSearchParams();
  const [sessionChat, setSessionChat] = useState<boolean>(false);
  const [errorMsg, setErrorMsg] = useState<string>("");
  const [file, setFile] = useState<File | null>(null);
  const [fileData, setFileData] = useState<{
    uri: string;
    fileType: string;
  } | null>(null);
  const { toast } = useToast();

  const messageContainerRef = useRef<HTMLDivElement>(null);

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

  // useEffect(() => {
  //     if (!user.isAuthenticated) {
  //         router.push("/");
  //     }
  //   }, [user,router]);

  // Load video details from searchParams
  useEffect(() => {
    const queryParams = Object.fromEntries(searchParams);
    const { documentId, title, url, type } = queryParams;
    if (documentId && title) {
      const newDocument: Document = {
        documentId,
        url: url || "",
        title,
        createdAt: new Date(),
        updatedAt: new Date(),
        type,
      };
      setSessionChat(false);
      setDocument(newDocument);
      fetchMessages(documentId); // Fetch messages for this video on load
    }else{
      setSessionChat(true)
    }
  }, [searchParams]);

  // Fetch messages for a document
  const fetchMessages = async (sessionId: string) => {
    try {
      setLoadingMessages(true);
      const response = await fetch(
        `http://127.0.0.1:5000/messages/sessions/${sessionId}`,
      );
      if (!response.ok) {
        throw new Error("Failed to fetch messages");
      }
      let data: Message[] = await response.json();

      setMessages(data);
    } catch (error) {
      console.error("Error fetching messages:", error);
    } finally {
      setLoadingMessages(false);
    }
  };

  // Send a new message to the AI handler
  const sendMessage = async () => {
    setErrorMsg("");
    if (!input.trim()) {
      toast({
        description: (
          <p className="text-orange-500">
          Please enter a text message.
          </p>
        ),
      });
      return;
    }
    if (!document || !input.trim()) {
      toast({
        description: (
          <p className="text-orange-500">
            No document selected !!. Choose a document you want to converse on.
          </p>
        ),
      });
      return;
    }

    if (!input.trim) {
      toast({
        description: <p className="text-orange-500">Please enter a message.</p>,
      });
      return;
    }

    setLoadingAi(true);
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
      formData.append("sessionId", document?.documentId as string);
      if (file) {
        formData.append("file", file);
      }

      const response = await fetch("http://127.0.0.1:5000/ai-chat-docs", {
        method: "POST",
        body: formData,
      });
      const reply: Message = await response.json();
      // console.log({reply})
      if (!response.ok) {
        throw new Error("Failed to send message.");
      }

      setInput("");
      setFile(null);

      setMessages((prev) => [...prev, reply]);
    } catch (error) {
      console.error(error);
      setMessages((prev) => prev.slice(0, -1)); // Remove unsent message
      setErrorMsg(
        "Couldn't access service due to traffic issues or the document time limit is exceeded. Try again and re-upload the document again if the issue persists.",
      );
    } finally {
      setLoadingAi(false);
    }
  };


  const handleSessionChat = ()=>{
    if (typeof window !== 'undefined') {
      const url = new URL(window.location.href);
      url.search = '';
      window.history.replaceState({}, '', url);
    }
    setSessionChat(true)
    
  }

  // Send a new message to the AI handler
  // const sendMessage = async (text: string) => {
  //   if (!document || !text.trim()) {
  //     toast({
  //       description: (
  //         <p className="text-orange-500">
  //           No document selected !!. Choose a document you want to converse on.
  //         </p>
  //       ),
  //     });
  //     return;
  //   }

  //   setLoadingAi(true);
  //   const newMessage: Message = {
  //     role: "user",
  //     content: text,
  //     createdAt: new Date().toISOString(),
  //     contentType:"text"
  //   };

  //   setMessages((prev) => [...(prev || []), newMessage]); // Optimistically update the UI

  //   try {
  //     const response = await fetch("http://127.0.0.1:5000/ai-chat-docs", {
  //       method: "POST",
  //       headers: {
  //         "Content-Type": "application/json",
  //       },
  //       body: JSON.stringify({
  //         text,
  //         documentId: document.documentId,
  //       }),
  //     });

  //     if (!response.ok) {
  //       throw new Error("Failed to send message.");
  //     }
  //     setInput("");
  //     const reply: Message = await response.json();

  //     setMessages((prev) => [...(prev || []), reply]); // Add AI's reply to the chat
  //   } catch (error) {
  //     setMessages((prev) => {
  //       prev.pop();
  //       return prev;
  //     });
  //     setErrorMsg(
  //       "Couldn't access service due to traffic issues or the document time limit is exceeded. Try again and re-upload the document again if the issue persists.",
  //     );
  //     // console.error("Error sending message:", error);
  //   } finally {
  //     setLoadingAi(false);
  //   }
  // };

  const handleSendMessage = () => {
    setErrorMsg("");
    sendMessage(); // Call the sendMessage function
  };

  // Scroll to the most recent message when the message list changes
  useEffect(() => {
    if (messageContainerRef.current) {
      messageContainerRef.current.scrollTop =
        messageContainerRef.current.scrollHeight;
    }
  }, [messages]);

  if (sessionChat) {
    return <UnReferencedChatRoom />;
  }

  return (
    <div className="relative flex flex-col w-full mx-auto md:w-[65vw]">
      <div className=" p-4 rounded flex flex-row items-center gap-2 justify-between">
        {/* {document?.type == "pdf" && <File/>}
         {document?.type !== "pdf" && <Book/>} */}
        <h4>{document?.title}</h4>
        <Button
          size="sm"
          variant="outline"
          className="text-primary border border-primary"
          onClick={handleSessionChat}
        >
          Use unreferenced chat box
        </Button>
      </div>

      {/* Chat area */}

      <div
        ref={messageContainerRef}
        className="overflow-y-auto overflow-x-hidden bg-neutral-100 h-[72vh] p-4 w-full rounded-lg custom-scrollbar"
      >
        {loadingMessages && (
          <div className="flex-1 flex h-full items-center justify-center">
            <Loader className="text-primary/30 animate-spin h-8 w-8" />
          </div>
        )}

        {!loadingMessages &&
          messages?.map((message, index) => {
           let file = messages[messages.lastIndexOf(message) - 1]?.contentType == "file"
           return  (
            <MessageItem key={index} message={message} file={file} />
          )
          })
        }

        {errorMsg && (
          <div
            className={cn(
              "bg-orange-100 text-orange-600 text-center p-2 rounded-lg",
            )}
          >
            <Markdown
              remarkPlugins={[remarkGfm, remarkMath]}
              className="text-sm"
            >
              {errorMsg}
            </Markdown>
            <div>
              <Button
                variant="outline"
                className="my-2 border-orange-500 bg-transparent hover:bg-orange-200"
                onClick={handleSendMessage}
              >
                Try again
              </Button>
            </div>
          </div>
        )}
        {loadingAi && (
          <div>
            <LoaderPinwheel className="animate-spin w-8 h-8 text-primary my-6 mx-2" />
          </div>
        )}
      </div>
      {/* Input area */}
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

// const spreadsheetData = {
//   columnLabels: ["Order ID", "Customer", "Employee", "Ship Name", "City", "Address"],
//   rowLabels: ["Row 1", "Row 2", "Row 3", "Row 4", "Row 5", "Row 6", "Row 7", "Row 8", "Row 9", "Row 10"],
//   data: [
//     [{ value: 10248 }, { value: "VINET" }, { value: 5 }, { value: "Vins et alcools Chevalier" }, { value: "Reims" }, { value: "59 rue de lAbbaye" }],
//     [{ value: 10249 }, { value: "TOMSP" }, { value: 6 }, { value: "Toms Spezialitäten" }, { value: "Münster" }, { value: "Luisenstr. 48" }],
//     [{ value: 10250 }, { value: "HANAR" }, { value: 4 }, { value: "Hanari Carnes" }, { value: "Rio de Janeiro" }, { value: "Rua do Paço, 67" }],
//     [{ value: 10251 }, { value: "VICTE" }, { value: 3 }, { value: "Victuailles en stock" }, { value: "Lyon" }, { value: "2, rue du Commerce" }],
//     [{ value: 10252 }, { value: "SUPRD" }, { value: 4 }, { value: "Suprêmes délices" }, { value: "Charleroi" }, { value: "Boulevard Tirou, 255" }],
//     [{ value: 10253 }, { value: "ALFKI" }, { value: 7 }, { value: "Alfreds Futterkiste" }, { value: "Berlin" }, { value: "Obere Str. 57" }],
//     [{ value: 10254 }, { value: "FRANK" }, { value: 1 }, { value: "Frankenversand" }, { value: "Mannheim" }, { value: "Berliner Platz 43" }],
//     [{ value: 10255 }, { value: "BLONP" }, { value: 2 }, { value: "Blondel père et fils" }, { value: "Strasbourg" }, { value: "24, place Kléber" }],
//     [{ value: 10256 }, { value: "FOLKO" }, { value: 8 }, { value: "Folk och fä HB" }, { value: "Bräcke" }, { value: "Åkergatan 24" }],
//     [{ value: 10257 }, { value: "MEREP" }, { value: 9 }, { value: "Mère Paillarde" }, { value: "Montréal" }, { value: "43 rue St. Laurent" }],
//   ],
// };

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
