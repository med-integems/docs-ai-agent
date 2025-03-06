"use client";

import AddDocumentModal from "@/components/AddDocumentComponent";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogFooter,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";

import {
  Tooltip,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";

import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  useSidebar,
} from "@/components/ui/sidebar";

import { useCurrentUser } from "@/hooks/use-current-user";
import { useToast } from "@/hooks/use-toast";
import { cn } from "@/lib/utils";
import {
  Book,
  Grid,
  Home,
  HomeIcon,
  List,
  Loader2,
  LogOut,
  Table,
  Trash2,
} from "lucide-react";
import Link from "next/link";
import { use, useState } from "react";
import MenuBar from "./MenuBar";
import UserButton from "./UserButton";

import { useRouter } from "next/navigation";

export type Document = {
  documentId: string;
  url: string;
  title: string;
  type: string;
  createdAt: Date;
  updatedAt: Date;
};

interface SidebarProps {
  documentPromise: Promise<Document[]>;
}

export function ChatRoomSideBar({ documentPromise }: SidebarProps) {
  const resolvedDocuments = use(documentPromise);
  const [activeDocumentId, setActiveDocumentId] = useState<string | null>(null);
  const { open, setOpen, isMobile } = useSidebar();

  const handleLinkClick = (documentId: string) => {
    setActiveDocumentId(documentId);
    if (isMobile) {
      setOpen(false);
    }
  };

  if (isMobile) {
    return (
      <Sheet open={open} onOpenChange={setOpen}>
        <SheetTitle className="hidden"></SheetTitle>
        <SheetTrigger asChild>
          <div className="absolute top-2 left-[85%] z-50">
            <MenuBar />
          </div>
        </SheetTrigger>
        <SheetContent side="left" className="w-[80vw] h-full">
          <SidebarContentUI
            resolvedDocuments={resolvedDocuments}
            activeDocumentId={activeDocumentId}
            handleLinkClick={handleLinkClick}
          />
        </SheetContent>
      </Sheet>
    );
  }

  return (
    <>
      {/* Desktop Sidebar */}
      <Sidebar variant="sidebar">
        <SidebarContentUI
          resolvedDocuments={resolvedDocuments || []}
          activeDocumentId={activeDocumentId}
          handleLinkClick={handleLinkClick}
        />
      </Sidebar>
    </>
  );
}

interface SidebarContentUIProps {
  resolvedDocuments: Document[];
  activeDocumentId: string | null;
  handleLinkClick: (videoId: string) => void;
}

function SidebarContentUI({
  resolvedDocuments,
  activeDocumentId,
  handleLinkClick,
}: SidebarContentUIProps) {
  const user = useCurrentUser();
  const router = useRouter();

  return (
    <>
      <SidebarHeader>
        <div className="flex flex-row items-center gap-2 px-2">
          <UserButton />
          <h3 className="font-semibold">{user.user?.name}</h3>
        </div>

        <AddDocumentModal />
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>Ref Documents</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {resolvedDocuments?.map((document) => (
                <SideBarDocumentItem
                  key={document.documentId}
                  document={document}
                  active={document.documentId === activeDocumentId}
                  handleLinkClick={handleLinkClick}
                />
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>

      <SidebarFooter className="absolute bottom-5 w-[90%] mx-auto">
        <div className="flex flex-row items-center justify-evenly bg-primary/5 rounded-md p-1 text-primary">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="sm"
                className="font-bold"
                onClick={() => router.push("/")}
              >
                <HomeIcon strokeWidth={3} />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Home</TooltipContent>
          </Tooltip>

          {user.user?.role?.includes("admin") && (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="sm"
                  className="font-bold"
                  onClick={() => router.push("/dashboard")}
                >
                  <Table strokeWidth={3} />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Dashboard</TooltipContent>
            </Tooltip>
          )}

          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="sm"
                className="font-bold"
                onClick={() => user.signOut()}
              >
                <LogOut strokeWidth={3} />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Logout</TooltipContent>
          </Tooltip>
        </div>
      </SidebarFooter>
    </>
  );
}

interface SideBarDocumentItemProps {
  document: Document;
  handleLinkClick: (videoId: string) => void;
  active: boolean;
}

const SideBarDocumentItem = ({
  document,
  active,
  handleLinkClick,
}: SideBarDocumentItemProps) => {
  const { user } = useCurrentUser();
  const [deleted, setDeleted] = useState<boolean>(false);
  const [loading, setLoading] = useState<boolean>(false);
  const { toast } = useToast();

  const handleDelete = async () => {
    setLoading(true);
    try {
      const response = await fetch(
        `http://127.0.0.1:5000/documents/${document.documentId}`,
        {
          method: "DELETE",
        },
      );

      if (!response.ok) {
        throw new Error("Failed to delete document.");
      }

      setDeleted(true);
    } catch (error) {
      console.error(error);
      toast({
        description: `<p className="text-red-500">Couldn't delete video !!</p>`,
      });
    } finally {
      setLoading(false);
    }
  };

  if (deleted) return null;

  return (
    <SidebarMenuItem key={document.documentId}>
      <SidebarMenuButton asChild>
        <div
          className={cn(
            "flex items-center justify-between w-full",
            active ? "text-primary border-b-2 border-primary" : "",
          )}
        >
          <Book className="mr-1 h-10 w-10" />
          <Link
            href={`/chat/${user?.userId}/?documentId=${document.documentId}&type=${document.type}&url=${document.url}&title=${document.title}`}
            onClick={() => handleLinkClick(document.documentId)}
            className="hover:underline flex items-center w-full"
          >
            <span className="line-clamp-1">{document.title}</span>
          </Link>

          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="hover:bg-red-50 hover:text-red-700"
              >
                {loading ? <Loader2 className="animate-spin" /> : <Trash2 />}
              </Button>
            </AlertDialogTrigger>

            <AlertDialogContent>
              <AlertDialogTitle>Delete document</AlertDialogTitle>
              <p className="text-sm">
                Are you sure you want to delete this document?
              </p>
              <AlertDialogFooter>
                <AlertDialogCancel>Cancel</AlertDialogCancel>
                <AlertDialogAction onClick={handleDelete} disabled={loading}>
                  {loading ? (
                    <Loader2 className="animate-spin h-4 w-4" />
                  ) : (
                    "Delete"
                  )}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>
      </SidebarMenuButton>
    </SidebarMenuItem>
  );
};
