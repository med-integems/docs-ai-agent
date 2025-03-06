"use client";

import Image from "next/image";
import Link from "next/link";
import { Button } from "./ui/button";
import { useCurrentUser } from "@/hooks/use-current-user";
import UserButton from "./UserButton";
import { Bot } from "lucide-react";

export const NavBar = () => {
  const user = useCurrentUser();

  return (
    <div className="fixed top-0 z-50 w-full max-w-7xl mx-auto px-4 md:px-20 bg-primary">
      <div className="flex flex-row justify-between h-20 items-center px-4">
        {/* Brand section */}
        <div className="flex flex-row">
          <h1 className="text-2xl text-white font-black md:text-3xl">Docs</h1>
          <h1 className="text-2xl text-green-400 font-black md:text-3xl">
            track
          </h1>
        </div>

        {/* <div className="flex items-center space-x-2">
          <Image
            src={"/logo.png"}
            width={100}
            height={100}
            alt="Vidstract Logo"
            className="m-2"
          />
        </div> */}
        <div className="flex items-center space-x-4">
          {user.isAuthenticated && (
            <div className="flex flex-row items-center space-x-2 gap-2">
              {/* <Button asChild className="border-white border bg-transparent"> */}
              <Link
                href={`/chat/${user.user?.userId}`}
                className="text-secondary"
              >
                <Bot />
              </Link>
              {/* </Button> */}

              {user.user?.role?.includes("admin") && (
                <Button asChild className="border-white border bg-transparent">
                  <Link href={`/dashboard`} className="font-bold">
                    Dashboard
                  </Link>
                </Button>
              )}
            </div>
          )}
          <UserButton />
        </div>
      </div>
    </div>
  );
};

export default NavBar;
