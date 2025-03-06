"use client";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useCurrentUser } from "@/hooks/use-current-user";
import { LogOut } from "lucide-react";

import { useRouter } from "next/navigation";

export default function UserButton() {
  const { user, isAuthenticated, signOut } = useCurrentUser();
  const router = useRouter();

  if (!isAuthenticated) {
    return (
      <Button
        variant="outline"
        className="text-primary"
        onClick={() => router.push("/signin")}
      >
        Sign in
      </Button>
    );
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" className="p-1 rounded-full">
          <Avatar className="w-9 h-9">
            <AvatarImage src={user?.image} alt={user?.name} />
            <AvatarFallback>{user?.name?.charAt(0) || "U"}</AvatarFallback>
          </Avatar>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-48">
        <div className="w-full flex flex-col items-center justify-center p-5">
          <Avatar className="w-20 h-20">
            <AvatarImage src={user?.image} alt={user?.name} />
            <AvatarFallback className="text-2xl font-semibold">
              {user?.name?.charAt(0) || "U"}
            </AvatarFallback>
          </Avatar>
          <h3 className="mt-2">{user?.name}</h3>
        </div>
        <hr />
        <DropdownMenuItem onClick={signOut} className="text-red-500 m-2">
          <LogOut />
          Sign Out
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
