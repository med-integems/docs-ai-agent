"use client";
import ChatRoom from "@/components/ChatRoom";
import { CookiesProvider } from "react-cookie";
export default function DashboardPage() {
  return (
    <CookiesProvider>
      <main className="max-w-7xl w-full mx-auto px-4 md:px-20">
        <ChatRoom />
      </main>
    </CookiesProvider>
  );
}
