import { SidebarProvider, SidebarTrigger } from "@/components/ui/sidebar";
import { ChatRoomSideBar } from "../../../components/ChatRoomSideBar";
import { Toaster } from "@/components/ui/toaster";
import "../../globals.css";

export const dynamic = "force-dynamic";

type Document = {
  documentId: string;
  url: string;
  title: string;
  type: string;
  createdAt: Date;
  updatedAt: Date;
};

// const API_URL =
//   typeof window === "undefined" ? "http://app-server:5000" : "http://localhost:5000";

const API_URL = "http://localhost:5000";

const getDocuments = async (userId: string): Promise<Document[]> => {
  try {
    const response = await fetch(`${API_URL}/documents/users/${userId}`, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
      },
      cache: "no-store",
    });

    if (!response.ok) {
      throw new Error("Failed to fetch videos");
    }

    const documents: Document[] = await response.json();
    return documents;
  } catch {
    // console.error("Error fetching videos:", error);
    return []; // Return an empty array on error
  }
};

export default async function Layout({
  children,
  params,
}: {
  children: React.ReactNode;
  params: Promise<{ userId: string }>;
}) {
  const { userId } = await params;
  const documentPromise = getDocuments(userId);
  return (
    <SidebarProvider>
      <ChatRoomSideBar documentPromise={documentPromise} />
      <SidebarTrigger className="m-4 hidden md:flex" />
      <main className="mx-auto w-full">
        {children}
        <Toaster />
      </main>
    </SidebarProvider>
  );
}
